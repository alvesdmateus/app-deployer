package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/alvesdmateus/app-deployer/internal/deployer"
	"github.com/alvesdmateus/app-deployer/internal/orchestrator"
	"github.com/alvesdmateus/app-deployer/internal/provisioner"
	"github.com/alvesdmateus/app-deployer/internal/provisioner/gcp"
	"github.com/alvesdmateus/app-deployer/internal/queue"
	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/alvesdmateus/app-deployer/pkg/config"
	"github.com/alvesdmateus/app-deployer/pkg/database"
)

func main() {
	// Initialize logger
	zlog := zerolog.New(os.Stdout).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	zlog.Info().Msg("Starting app-deployer orchestrator worker")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to load configuration")
	}

	zlog.Info().
		Str("gcp_project", cfg.Provisioner.GCPProject).
		Str("gcp_region", cfg.Provisioner.GCPRegion).
		Int("worker_concurrency", cfg.Worker.Concurrency).
		Msg("Configuration loaded")

	// Connect to database
	zlog.Info().Msg("Connecting to database...")
	dbConfig := database.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		DBName:          cfg.Database.DBName,
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}
	db, err := database.New(dbConfig)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer database.Close(db)

	zlog.Info().Msg("Database connected successfully")

	// Run migrations
	zlog.Info().Msg("Running database migrations...")
	if err := database.Migrate(db, &state.Deployment{}, &state.Infrastructure{}, &state.Build{}); err != nil {
		zlog.Fatal().Err(err).Msg("Failed to run database migrations")
	}
	zlog.Info().Msg("Database migrations completed")

	// Create repository
	repo := state.NewRepository(db)

	// Connect to Redis queue
	zlog.Info().
		Str("redis_url", cfg.Redis.URL).
		Int("redis_db", cfg.Redis.DB).
		Msg("Connecting to Redis...")

	redisQueue, err := queue.NewRedisQueue(cfg.Redis.URL, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer redisQueue.Close()

	zlog.Info().Msg("Redis connected successfully")

	// Verify Redis connection
	ctx := context.Background()
	if err := redisQueue.Ping(ctx); err != nil {
		zlog.Fatal().Err(err).Msg("Redis ping failed")
	}

	// Initialize GCP provisioner
	zlog.Info().Msg("Initializing GCP provisioner...")

	if cfg.Provisioner.GCPProject == "" {
		zlog.Fatal().Msg("provisioner.gcp_project must be configured")
	}
	if cfg.Provisioner.PulumiBackend == "" {
		zlog.Fatal().Msg("provisioner.pulumi_backend must be configured (e.g., gs://bucket/path)")
	}

	gcpConfig := gcp.Config{
		GCPProject:      cfg.Provisioner.GCPProject,
		GCPRegion:       cfg.Provisioner.GCPRegion,
		PulumiBackend:   cfg.Provisioner.PulumiBackend,
		DefaultNodeType: cfg.Provisioner.DefaultNodeType,
		DefaultNodes:    cfg.Provisioner.DefaultNodes,
	}

	provisionerTracker := provisioner.NewTracker(repo)
	gcpProv, err := gcp.NewGCPProvisioner(gcpConfig, provisionerTracker)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to create GCP provisioner")
	}

	// Verify GCP access
	if err := gcpProv.VerifyAccess(ctx); err != nil {
		zlog.Fatal().Err(err).Msg("Failed to verify GCP access - ensure gcloud is authenticated")
	}

	zlog.Info().Msg("GCP provisioner initialized successfully")

	// Initialize Helm deployer
	zlog.Info().Msg("Initializing Helm deployer...")

	deployerConfig := deployer.Config{
		ChartPath:       "templates/helm/base-app",
		DefaultReplicas: cfg.Deployer.DefaultReplicas,
		DefaultPort:     cfg.Deployer.DefaultPort,
	}

	deployerTracker := deployer.NewTracker(repo)
	helmDeployer, err := deployer.NewHelmDeployer(deployerConfig, deployerTracker)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to create Helm deployer")
	}

	zlog.Info().Msg("Helm deployer initialized successfully")

	// Create orchestrator engine
	zlog.Info().Msg("Creating orchestrator engine...")
	engine := orchestrator.NewEngine(redisQueue, repo, gcpProv, helmDeployer, zlog)

	// Create and start worker
	worker := orchestrator.NewWorker(engine, cfg.Worker.Concurrency, zlog)

	// Create context that listens for interrupt signals
	workerCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	zlog.Info().
		Int("concurrency", cfg.Worker.Concurrency).
		Dur("poll_interval", cfg.Worker.PollInterval).
		Msg("Starting orchestrator worker...")

	// Start worker in goroutine
	workerErrChan := make(chan error, 1)
	go func() {
		if err := worker.Start(workerCtx); err != nil {
			workerErrChan <- err
		}
	}()

	zlog.Info().Msg("Orchestrator worker started successfully, processing jobs...")

	// Wait for interrupt signal or worker error
	select {
	case <-workerCtx.Done():
		zlog.Info().Msg("Received shutdown signal, stopping worker gracefully...")
	case err := <-workerErrChan:
		zlog.Error().Err(err).Msg("Worker encountered an error")
	}

	// Graceful shutdown
	zlog.Info().Msg("Worker stopped")
	zlog.Info().Msg("Orchestrator worker shutdown complete")
}

// Helper function to format error messages
func fatalError(msg string, err error) string {
	if err != nil {
		return fmt.Sprintf("%s: %v", msg, err)
	}
	return msg
}
