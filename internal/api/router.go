package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mateus/app-deployer/internal/state"
	"github.com/mateus/app-deployer/pkg/database"
	"gorm.io/gorm"
)

// Server represents the HTTP API server
type Server struct {
	router               *chi.Mux
	db                   *gorm.DB
	deploymentHandler    *DeploymentHandler
	infrastructureHandler *InfrastructureHandler
	buildHandler         *BuildHandler
	analyzerHandler      *AnalyzerHandler
}

// NewServer creates a new API server
func NewServer(db *gorm.DB) *Server {
	repo := state.NewRepository(db)

	s := &Server{
		router:               chi.NewRouter(),
		db:                   db,
		deploymentHandler:    NewDeploymentHandler(repo),
		infrastructureHandler: NewInfrastructureHandler(repo),
		buildHandler:         NewBuildHandler(repo),
		analyzerHandler:      NewAnalyzerHandler(),
	}

	s.setupRoutes()
	return s
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
