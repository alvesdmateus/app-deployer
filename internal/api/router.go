package api

import (
	"net/http"

	"github.com/alvesdmateus/app-deployer/internal/analyzer"
	"github.com/alvesdmateus/app-deployer/internal/builder"
	"github.com/alvesdmateus/app-deployer/internal/builder/registry"
	"github.com/alvesdmateus/app-deployer/internal/builder/strategies"
	"github.com/alvesdmateus/app-deployer/internal/observability"
	"github.com/alvesdmateus/app-deployer/internal/orchestrator"
	"github.com/alvesdmateus/app-deployer/internal/queue"
	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/alvesdmateus/app-deployer/pkg/config"
	"github.com/alvesdmateus/app-deployer/pkg/database"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Server represents the HTTP API server
type Server struct {
	router                *chi.Mux
	db                    *gorm.DB
	redisQueue            *queue.RedisQueue
	orchestratorClient    *orchestrator.Client
	deploymentHandler     *DeploymentHandler
	infrastructureHandler *InfrastructureHandler
	buildHandler          *BuildHandler
	analyzerHandler       *AnalyzerHandler
	builderHandler        *BuilderHandler
	metrics               *observability.Metrics
	tracer                *observability.Tracer
}

// NewServer creates a new API server
func NewServer(db *gorm.DB) *Server {
	repo := state.NewRepository(db)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load config, using defaults")
		// Use a minimal default config
		cfg = &config.Config{
			Platform: config.PlatformConfig{
				DefaultCloud:  "gcp",
				DefaultRegion: "us-central1",
			},
			Registry: config.RegistryConfig{
				Type:     "artifact-registry",
				Project:  "",
				Location: "us-central1",
			},
			Redis: config.RedisConfig{
				URL:      "localhost:6379",
				Password: "",
				DB:       0,
			},
		}
	}

	// Initialize Redis queue
	redisQueue, err := queue.NewRedisQueue(cfg.Redis.URL, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to connect to Redis, orchestration features disabled")
		// Continue without Redis - orchestration endpoints will return errors
	}

	// Initialize orchestrator client
	var orchClient *orchestrator.Client
	if redisQueue != nil {
		orchClient = orchestrator.NewClient(redisQueue, log.Logger)
	}

	// Initialize analyzer
	analyzer := analyzer.New()

	// Initialize build tracker
	buildTracker := builder.NewTracker(repo)

	// Initialize build service
	buildService, err := initializeBuildService(cfg, buildTracker)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize build service")
		// Continue with nil build service - endpoints will return errors
	}

	// Initialize metrics
	metrics := observability.NewMetrics("app_deployer")

	// Initialize tracer (uses global tracer, must be initialized at application startup)
	tracer := observability.GetGlobalTracer()

	s := &Server{
		router:                chi.NewRouter(),
		db:                    db,
		redisQueue:            redisQueue,
		orchestratorClient:    orchClient,
		deploymentHandler:     NewDeploymentHandler(repo, orchClient),
		infrastructureHandler: NewInfrastructureHandler(repo),
		buildHandler:          NewBuildHandler(repo),
		analyzerHandler:       NewAnalyzerHandler(),
		builderHandler:        NewBuilderHandler(buildService, analyzer),
		metrics:               metrics,
		tracer:                tracer,
	}

	s.setupRoutes()
	return s
}

// initializeBuildService creates and configures the build service
func initializeBuildService(cfg *config.Config, tracker builder.BuildTracker) (builder.BuildService, error) {
	// Create registry config
	registryConfig := registry.Config{
		Type:     cfg.Registry.Type,
		Host:     "", // Will be determined by registry type
		Project:  cfg.Registry.Project,
		Location: cfg.Registry.Location,
	}

	// Create build service config
	serviceConfig := builder.ServiceConfig{
		RegistryConfig: registryConfig,
		StrategyType:   strategies.StrategyTypeDocker,
	}

	// Create build service
	buildService, err := builder.NewService(serviceConfig, tracker)
	if err != nil {
		return nil, err
	}

	return buildService, nil
}

// setupRoutes configures all routes
func (s *Server) setupRoutes() {
	// Middleware
	s.router.Use(middleware.RequestID)
	s.router.Use(RecoveryMiddleware)
	s.router.Use(TracingMiddleware(s.tracer)) // Distributed tracing
	s.router.Use(RequestLogger)
	s.router.Use(CORSMiddleware())
	s.router.Use(middleware.RealIP)
	s.router.Use(MetricsMiddleware(s.metrics))

	// Health check endpoints
	s.router.Get("/health", s.healthCheck)
	s.router.Get("/health/live", s.livenessCheck)
	s.router.Get("/health/ready", s.readinessCheck)

	// Prometheus metrics endpoint
	s.router.Handle("/metrics", promhttp.Handler())

	// API v1 routes
	s.router.Route("/api/v1", func(r chi.Router) {
		// Deployment routes
		r.Route("/deployments", func(r chi.Router) {
			r.Get("/", s.deploymentHandler.ListDeployments)
			r.Post("/", s.deploymentHandler.CreateDeployment)
			r.Get("/status/{status}", s.deploymentHandler.GetDeploymentsByStatus)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.deploymentHandler.GetDeployment)
				r.Delete("/", s.deploymentHandler.DeleteDeployment)
				r.Patch("/status", s.deploymentHandler.UpdateDeploymentStatus)

				// Orchestration endpoints
				r.Post("/deploy", s.deploymentHandler.StartDeployment)
				r.Post("/rollback", s.deploymentHandler.TriggerRollback)

				// Logs endpoint
				r.Get("/logs", s.deploymentHandler.GetDeploymentLogs)

				// Infrastructure sub-routes
				r.Get("/infrastructure", s.infrastructureHandler.GetInfrastructure)

				// Build sub-routes
				r.Get("/builds/latest", s.buildHandler.GetLatestBuild)
			})
		})

		// Analyzer routes
		r.Route("/analyze", func(r chi.Router) {
			r.Post("/", s.analyzerHandler.AnalyzeSourceCode)
			r.Post("/upload", s.analyzerHandler.UploadAndAnalyze)
			r.Get("/languages", s.analyzerHandler.GetSupportedLanguages)
		})

		// Build routes
		r.Route("/builds", func(r chi.Router) {
			r.Post("/", s.builderHandler.BuildImage)
			r.Post("/generate-dockerfile", s.builderHandler.GenerateDockerfile)

			r.Route("/{buildID}", func(r chi.Router) {
				r.Get("/logs", s.builderHandler.GetBuildLogs)
				r.Post("/cancel", s.builderHandler.CancelBuild)
			})
		})

		// Orchestrator routes
		r.Route("/orchestrator", func(r chi.Router) {
			r.Get("/stats", s.deploymentHandler.GetQueueStats)
		})
	})
}

// healthCheck handles GET /health
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := database.HealthCheck(s.db); err != nil {
		dbStatus = "error"
	}

	response := HealthResponse{
		Status:   "ok",
		Database: dbStatus,
		Version:  "1.0.0",
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// livenessCheck handles GET /health/live - checks if process is running
func (s *Server) livenessCheck(w http.ResponseWriter, r *http.Request) {
	RespondWithJSON(w, http.StatusOK, map[string]string{
		"status": "alive",
	})
}

// readinessCheck handles GET /health/ready - checks if service can accept traffic
func (s *Server) readinessCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":   "ready",
		"database": "healthy",
		"redis":    "healthy",
	}

	// Check database connection
	if err := database.HealthCheck(s.db); err != nil {
		log.Error().Err(err).Msg("Readiness check failed: database unhealthy")
		response["status"] = "not_ready"
		response["database"] = "unhealthy"
		RespondWithJSON(w, http.StatusServiceUnavailable, response)
		return
	}

	// Check Redis connection (if configured)
	if s.redisQueue != nil {
		if err := s.redisQueue.Ping(r.Context()); err != nil {
			log.Error().Err(err).Msg("Readiness check failed: redis unhealthy")
			response["status"] = "not_ready"
			response["redis"] = "unhealthy"
			RespondWithJSON(w, http.StatusServiceUnavailable, response)
			return
		}
	} else {
		response["redis"] = "not_configured"
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// Handler returns the http.Handler for the server
func (s *Server) Handler() http.Handler {
	return s.router
}
