//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/alvesdmateus/app-deployer/internal/deployer"
	"github.com/alvesdmateus/app-deployer/internal/orchestrator"
	"github.com/alvesdmateus/app-deployer/internal/provisioner"
	"github.com/alvesdmateus/app-deployer/internal/provisioner/gcp"
	"github.com/alvesdmateus/app-deployer/internal/queue"
	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/alvesdmateus/app-deployer/pkg/config"
	"github.com/alvesdmateus/app-deployer/pkg/database"
)

// E2EConfig holds configuration for E2E tests
type E2EConfig struct {
	GCPProject    string
	GCPRegion     string
	PulumiBackend string

	TestImage string
	TestPort  int

	TimeoutProvision time.Duration
	TimeoutDeploy    time.Duration
	TimeoutDestroy   time.Duration
	PollInterval     time.Duration

	SkipCleanup bool

	DatabaseURL string
	RedisURL    string
}

// TestEnvironment holds all initialized components for E2E tests
type TestEnvironment struct {
	Config      *E2EConfig
	AppConfig   *config.Config
	DB          *gorm.DB
	Redis       *queue.RedisQueue
	Repo        *state.Repository
	Provisioner provisioner.Provisioner
	Deployer    deployer.Deployer
	Engine      *orchestrator.Engine
	Logger      zerolog.Logger

	// Track resources for cleanup
	DeploymentIDs     []string
	InfrastructureIDs []string
}

// LoadE2EConfig loads E2E test configuration from environment variables
func LoadE2EConfig() (*E2EConfig, error) {
	cfg := &E2EConfig{
		GCPProject:       os.Getenv("E2E_GCP_PROJECT"),
		GCPRegion:        getEnvOrDefault("E2E_GCP_REGION", "us-central1"),
		PulumiBackend:    os.Getenv("E2E_PULUMI_BACKEND"),
		TestImage:        getEnvOrDefault("E2E_TEST_IMAGE", "nginx:alpine"),
		TestPort:         80,
		TimeoutProvision: 20 * time.Minute,
		TimeoutDeploy:    10 * time.Minute,
		TimeoutDestroy:   15 * time.Minute,
		PollInterval:     30 * time.Second,
		SkipCleanup:      os.Getenv("E2E_SKIP_CLEANUP") == "true",
		DatabaseURL:      getEnvOrDefault("E2E_DATABASE_URL", ""),
		RedisURL:         getEnvOrDefault("E2E_REDIS_URL", "localhost:6379"),
	}

	// Validate required configuration
	if cfg.GCPProject == "" {
		return nil, fmt.Errorf("E2E_GCP_PROJECT environment variable is required")
	}
	if cfg.PulumiBackend == "" {
		return nil, fmt.Errorf("E2E_PULUMI_BACKEND environment variable is required")
	}

	return cfg, nil
}

// SetupTestEnvironment initializes all components for E2E testing
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()

	// Load E2E config
	e2eCfg, err := LoadE2EConfig()
	require.NoError(t, err, "Failed to load E2E configuration")

	// Initialize logger
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().Timestamp().Str("test", t.Name()).Logger()

	logger.Info().
		Str("gcp_project", e2eCfg.GCPProject).
		Str("gcp_region", e2eCfg.GCPRegion).
		Str("pulumi_backend", e2eCfg.PulumiBackend).
		Msg("Setting up E2E test environment")

	// Load application config
	appCfg, err := config.Load()
	require.NoError(t, err, "Failed to load application configuration")

	// Override with E2E settings
	appCfg.Provisioner.GCPProject = e2eCfg.GCPProject
	appCfg.Provisioner.GCPRegion = e2eCfg.GCPRegion
	appCfg.Provisioner.PulumiBackend = e2eCfg.PulumiBackend

	// Connect to database
	logger.Info().Msg("Connecting to database...")
	dbConfig := database.Config{
		Host:            appCfg.Database.Host,
		Port:            appCfg.Database.Port,
		User:            appCfg.Database.User,
		Password:        appCfg.Database.Password,
		DBName:          appCfg.Database.DBName,
		SSLMode:         appCfg.Database.SSLMode,
		MaxOpenConns:    appCfg.Database.MaxOpenConns,
		MaxIdleConns:    appCfg.Database.MaxIdleConns,
		ConnMaxLifetime: appCfg.Database.ConnMaxLifetime,
	}
	db, err := database.New(dbConfig)
	require.NoError(t, err, "Failed to connect to database")

	// Run migrations
	err = database.Migrate(db, &state.Deployment{}, &state.Infrastructure{}, &state.Build{}, &state.DeploymentLog{})
	require.NoError(t, err, "Failed to run database migrations")

	// Create repository
	repo := state.NewRepository(db)

	// Connect to Redis
	logger.Info().Msg("Connecting to Redis...")
	redisQueue, err := queue.NewRedisQueue(e2eCfg.RedisURL, "", 1) // Use DB 1 for E2E tests
	require.NoError(t, err, "Failed to connect to Redis")

	// Initialize GCP provisioner
	logger.Info().Msg("Initializing GCP provisioner...")
	gcpConfig := gcp.Config{
		GCPProject:      e2eCfg.GCPProject,
		GCPRegion:       e2eCfg.GCPRegion,
		PulumiBackend:   e2eCfg.PulumiBackend,
		DefaultNodeType: "e2-small",
		DefaultNodes:    2,
	}

	provisionerTracker := provisioner.NewTracker(repo)
	gcpProv, err := gcp.NewGCPProvisioner(gcpConfig, provisionerTracker)
	require.NoError(t, err, "Failed to create GCP provisioner")

	// Verify GCP access
	ctx := context.Background()
	err = gcpProv.VerifyAccess(ctx)
	require.NoError(t, err, "Failed to verify GCP access - ensure gcloud is authenticated")

	// Initialize Helm deployer
	logger.Info().Msg("Initializing Helm deployer...")
	deployerConfig := deployer.Config{
		ChartPath:       "templates/helm/base-app",
		DefaultReplicas: 1,
		DefaultPort:     e2eCfg.TestPort,
	}

	deployerTracker := deployer.NewTracker(repo)
	helmDeployer, err := deployer.NewHelmDeployer(deployerConfig, deployerTracker)
	require.NoError(t, err, "Failed to create Helm deployer")

	// Create orchestrator engine (builder is nil for E2E tests that use pre-built images)
	engine := orchestrator.NewEngine(redisQueue, repo, nil, gcpProv, helmDeployer, logger)

	logger.Info().Msg("E2E test environment setup complete")

	return &TestEnvironment{
		Config:            e2eCfg,
		AppConfig:         appCfg,
		DB:                db,
		Redis:             redisQueue,
		Repo:              repo,
		Provisioner:       gcpProv,
		Deployer:          helmDeployer,
		Engine:            engine,
		Logger:            logger,
		DeploymentIDs:     make([]string, 0),
		InfrastructureIDs: make([]string, 0),
	}
}

// Cleanup tears down the test environment and destroys any created resources
func (env *TestEnvironment) Cleanup(t *testing.T) {
	t.Helper()

	if env.Config.SkipCleanup {
		env.Logger.Warn().Msg("Skipping cleanup - E2E_SKIP_CLEANUP is set")
		return
	}

	env.Logger.Info().Msg("Starting E2E cleanup...")
	ctx := context.Background()

	// Destroy any infrastructure that was created
	for _, infraID := range env.InfrastructureIDs {
		env.Logger.Info().Str("infrastructure_id", infraID).Msg("Destroying infrastructure")
		// Get infrastructure details for stack name
		infra, err := env.Repo.GetInfrastructureByStackName(ctx, fmt.Sprintf("e2e-%s", infraID[:8]))
		if err != nil {
			env.Logger.Warn().Err(err).Str("infrastructure_id", infraID).Msg("Failed to get infrastructure for cleanup")
			continue
		}

		destroyReq := &provisioner.DestroyRequest{
			InfrastructureID: infraID,
			StackName:        infra.PulumiStackName,
		}
		if err := env.Provisioner.Destroy(ctx, destroyReq); err != nil {
			env.Logger.Error().Err(err).Str("infrastructure_id", infraID).Msg("Failed to destroy infrastructure")
		}
	}

	// Close Redis connection
	if env.Redis != nil {
		env.Redis.Close()
	}

	// Close database connection
	if env.DB != nil {
		database.Close(env.DB)
	}

	env.Logger.Info().Msg("E2E cleanup complete")
}

// TrackDeployment adds a deployment ID to be cleaned up
func (env *TestEnvironment) TrackDeployment(deploymentID string) {
	env.DeploymentIDs = append(env.DeploymentIDs, deploymentID)
}

// TrackInfrastructure adds an infrastructure ID to be cleaned up
func (env *TestEnvironment) TrackInfrastructure(infraID string) {
	env.InfrastructureIDs = append(env.InfrastructureIDs, infraID)
}

// WaitForDeploymentStatus polls the deployment status until it matches the expected status or times out
func (env *TestEnvironment) WaitForDeploymentStatus(ctx context.Context, t *testing.T, deploymentID string, expectedStatus string, timeout time.Duration) *state.Deployment {
	t.Helper()

	env.Logger.Info().
		Str("deployment_id", deploymentID).
		Str("expected_status", expectedStatus).
		Dur("timeout", timeout).
		Msg("Waiting for deployment status")

	ticker := time.NewTicker(env.Config.PollInterval)
	defer ticker.Stop()

	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Context cancelled while waiting for deployment status: %v", ctx.Err())
		case <-ticker.C:
			if time.Now().After(deadline) {
				t.Fatalf("Timeout waiting for deployment %s to reach status %s", deploymentID, expectedStatus)
			}

			depID, parseErr := uuid.Parse(deploymentID)
			if parseErr != nil {
				t.Fatalf("Invalid deployment ID: %v", parseErr)
			}
			deployment, err := env.Repo.GetDeploymentByID(ctx, depID)
			if err != nil {
				env.Logger.Warn().Err(err).Msg("Failed to get deployment status, retrying...")
				continue
			}

			env.Logger.Debug().
				Str("deployment_id", deploymentID).
				Str("current_status", deployment.Status).
				Str("expected_status", expectedStatus).
				Msg("Checking deployment status")

			if deployment.Status == expectedStatus {
				env.Logger.Info().
					Str("deployment_id", deploymentID).
					Str("status", deployment.Status).
					Msg("Deployment reached expected status")
				return deployment
			}

			// Check for failure status
			if deployment.Status == "FAILED" && expectedStatus != "FAILED" {
				t.Fatalf("Deployment %s failed: %s", deploymentID, deployment.Error)
			}
		}
	}
}

// WaitForInfrastructureStatus polls infrastructure status
func (env *TestEnvironment) WaitForInfrastructureStatus(ctx context.Context, t *testing.T, infraID string, expectedStatus string, timeout time.Duration) *state.Infrastructure {
	t.Helper()

	env.Logger.Info().
		Str("infrastructure_id", infraID).
		Str("expected_status", expectedStatus).
		Msg("Waiting for infrastructure status")

	ticker := time.NewTicker(env.Config.PollInterval)
	defer ticker.Stop()

	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Context cancelled while waiting for infrastructure status: %v", ctx.Err())
		case <-ticker.C:
			if time.Now().After(deadline) {
				t.Fatalf("Timeout waiting for infrastructure %s to reach status %s", infraID, expectedStatus)
			}

			infra, err := env.Repo.GetInfrastructureByStackName(ctx, infraID)
			if err != nil {
				env.Logger.Warn().Err(err).Msg("Failed to get infrastructure status, retrying...")
				continue
			}

			env.Logger.Debug().
				Str("infrastructure_id", infraID).
				Str("current_status", infra.Status).
				Str("expected_status", expectedStatus).
				Msg("Checking infrastructure status")

			if infra.Status == expectedStatus {
				env.Logger.Info().
					Str("infrastructure_id", infraID).
					Str("status", infra.Status).
					Msg("Infrastructure reached expected status")
				return infra
			}

			if infra.Status == "FAILED" && expectedStatus != "FAILED" {
				t.Fatalf("Infrastructure %s failed: %s", infraID, infra.LastError)
			}
		}
	}
}

// VerifyExternalAccess makes an HTTP request to the external IP and verifies it's accessible
func (env *TestEnvironment) VerifyExternalAccess(t *testing.T, externalIP string, port int, expectedStatusCode int) {
	t.Helper()

	url := fmt.Sprintf("http://%s:%d", externalIP, port)
	env.Logger.Info().Str("url", url).Msg("Verifying external access")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Retry a few times as the load balancer may take time to become ready
	var lastErr error
	for i := 0; i < 5; i++ {
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			env.Logger.Warn().Err(err).Int("attempt", i+1).Msg("Failed to access external URL, retrying...")
			time.Sleep(10 * time.Second)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == expectedStatusCode {
			env.Logger.Info().
				Str("url", url).
				Int("status_code", resp.StatusCode).
				Msg("External access verified successfully")
			return
		}

		lastErr = fmt.Errorf("unexpected status code: got %d, expected %d", resp.StatusCode, expectedStatusCode)
		env.Logger.Warn().
			Int("got", resp.StatusCode).
			Int("expected", expectedStatusCode).
			Msg("Unexpected status code, retrying...")
		time.Sleep(10 * time.Second)
	}

	t.Fatalf("Failed to verify external access to %s: %v", url, lastErr)
}

// GenerateTestDeploymentName generates a unique deployment name for testing
func GenerateTestDeploymentName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()%100000)
}

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
