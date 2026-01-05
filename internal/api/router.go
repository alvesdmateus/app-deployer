package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/alvesdmateus/app-deployer/internal/analyzer"
	"github.com/alvesdmateus/app-deployer/internal/builder"
	"github.com/alvesdmateus/app-deployer/internal/builder/registry"
	"github.com/alvesdmateus/app-deployer/internal/builder/strategies"
	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/alvesdmateus/app-deployer/pkg/config"
	"github.com/alvesdmateus/app-deployer/pkg/database"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Server represents the HTTP API server
type Server struct {
	router                *chi.Mux
	db                    *gorm.DB
	deploymentHandler     *DeploymentHandler
	infrastructureHandler *InfrastructureHandler
	buildHandler          *BuildHandler
	analyzerHandler       *AnalyzerHandler
	builderHandler        *BuilderHandler
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
		}
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

	s := &Server{
		router:                chi.NewRouter(),
		db:                    db,
		deploymentHandler:     NewDeploymentHandler(repo),
		infrastructureHandler: NewInfrastructureHandler(repo),
		buildHandler:          NewBuildHandler(repo),
		analyzerHandler:       NewAnalyzerHandler(),
		builderHandler:        NewBuilderHandler(buildService, analyzer),
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
	s.router.Use(RequestLogger)
	s.router.Use(CORSMiddleware())
	s.router.Use(middleware.RealIP)

	// Health check
	s.router.Get("/health", s.healthCheck)

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

// Handler returns the http.Handler for the server
func (s *Server) Handler() http.Handler {
	return s.router
}
