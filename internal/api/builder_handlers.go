package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/alvesdmateus/app-deployer/internal/analyzer"
	"github.com/alvesdmateus/app-deployer/internal/builder"
	"github.com/alvesdmateus/app-deployer/internal/builder/dockerfile"
)

// BuilderHandler handles build-related HTTP requests
type BuilderHandler struct {
	buildService builder.BuildService
	analyzer     *analyzer.Analyzer
}

// NewBuilderHandler creates a new builder handler
func NewBuilderHandler(buildService builder.BuildService, analyzer *analyzer.Analyzer) *BuilderHandler {
	return &BuilderHandler{
		buildService: buildService,
		analyzer:     analyzer,
	}
}

// BuildImageRequest represents a request to build a container image
type BuildImageRequest struct {
	DeploymentID string `json:"deployment_id"`
	AppName      string `json:"app_name"`
	Version      string `json:"version"`
	SourcePath   string `json:"source_path"`
}

// BuildImageResponse represents the response from a build request
type BuildImageResponse struct {
	BuildID      string `json:"build_id"`
	ImageTag     string `json:"image_tag"`
	ImageDigest  string `json:"image_digest"`
	Status       string `json:"status"`
	BuildLog     string `json:"build_log,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// BuildImage handles POST /api/v1/builds
// @Summary      Build container image
// @Description  Build a container image from source code
// @Tags         builder
// @Accept       json
// @Produce      json
// @Param        request  body      BuildImageRequest  true  "Build request"
// @Success      201      {object}  BuildImageResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /builds [post]
func (h *BuilderHandler) BuildImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req BuildImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode build image request")
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.DeploymentID == "" {
		RespondWithError(w, http.StatusBadRequest, "deployment_id is required")
		return
	}
	if req.AppName == "" {
		RespondWithError(w, http.StatusBadRequest, "app_name is required")
		return
	}
	if req.Version == "" {
		RespondWithError(w, http.StatusBadRequest, "version is required")
		return
	}
	if req.SourcePath == "" {
		RespondWithError(w, http.StatusBadRequest, "source_path is required")
		return
	}

	log.Info().
		Str("deploymentID", req.DeploymentID).
		Str("appName", req.AppName).
		Str("sourcePath", req.SourcePath).
		Msg("Received build image request")

	// Analyze source code
	log.Info().Str("sourcePath", req.SourcePath).Msg("Analyzing source code")
	analysis, err := h.analyzer.Analyze(req.SourcePath)
	if err != nil {
		log.Error().Err(err).Str("sourcePath", req.SourcePath).Msg("Failed to analyze source code")
		RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to analyze source code: %v", err))
		return
	}

	// Create build context
	buildCtx := &builder.BuildContext{
		DeploymentID: req.DeploymentID,
		AppName:      req.AppName,
		Version:      req.Version,
		SourcePath:   req.SourcePath,
		Analysis:     analysis,
	}

	// Build image (this is a long-running operation)
	// In production, this should be async via a job queue
	result, err := h.buildService.BuildImage(ctx, buildCtx)
	if err != nil {
		log.Error().
			Err(err).
			Str("deploymentID", req.DeploymentID).
			Msg("Failed to build image")

		response := BuildImageResponse{
			BuildID:      buildCtx.BuildID,
			Status:       "FAILED",
			ErrorMessage: err.Error(),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Success response
	response := BuildImageResponse{
		BuildID:     buildCtx.BuildID,
		ImageTag:    result.ImageTag,
		ImageDigest: result.ImageDigest,
		Status:      "SUCCESS",
	}

	log.Info().
		Str("buildID", buildCtx.BuildID).
		Str("imageTag", result.ImageTag).
		Msg("Image built successfully")

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetBuildLogs handles GET /api/v1/builds/{buildID}/logs
// @Summary      Get build logs
// @Description  Stream the logs for a specific build
// @Tags         builder
// @Produce      text/plain
// @Param        buildID  path      string  true  "Build ID"
// @Success      200      {string}  string  "Build logs"
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /builds/{buildID}/logs [get]
func (h *BuilderHandler) GetBuildLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	buildID := chi.URLParam(r, "buildID")

	if buildID == "" {
		RespondWithError(w, http.StatusBadRequest, "buildID is required")
		return
	}

	log.Debug().Str("buildID", buildID).Msg("Retrieving build logs")

	// Get build logs
	logsReader, err := h.buildService.GetBuildLogs(ctx, buildID)
	if err != nil {
		log.Error().Err(err).Str("buildID", buildID).Msg("Failed to get build logs")
		RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get build logs: %v", err))
		return
	}

	// Stream logs
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, logsReader); err != nil {
		log.Error().Err(err).Str("buildID", buildID).Msg("Failed to stream build logs")
	}
}

// CancelBuild handles POST /api/v1/builds/{buildID}/cancel
// @Summary      Cancel build
// @Description  Cancel an in-progress build
// @Tags         builder
// @Produce      json
// @Param        buildID  path      string  true  "Build ID"
// @Success      200      {object}  SuccessResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /builds/{buildID}/cancel [post]
func (h *BuilderHandler) CancelBuild(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	buildID := chi.URLParam(r, "buildID")

	if buildID == "" {
		RespondWithError(w, http.StatusBadRequest, "buildID is required")
		return
	}

	log.Info().Str("buildID", buildID).Msg("Cancelling build")

	// Cancel build
	if err := h.buildService.CancelBuild(ctx, buildID); err != nil {
		log.Error().Err(err).Str("buildID", buildID).Msg("Failed to cancel build")
		RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to cancel build: %v", err))
		return
	}

	RespondWithSuccess(w, http.StatusOK, "Build cancelled successfully", nil)
}

// GenerateDockerfileRequest represents a request to generate a Dockerfile
type GenerateDockerfileRequest struct {
	SourcePath string `json:"source_path"`
}

// GenerateDockerfileResponse represents the generated Dockerfile
type GenerateDockerfileResponse struct {
	Dockerfile string                   `json:"dockerfile"`
	Language   string                   `json:"language"`
	Framework  string                   `json:"framework"`
	Analysis   *analyzer.AnalysisResult `json:"analysis"`
}

// GenerateDockerfile handles POST /api/v1/builds/generate-dockerfile
// @Summary      Generate Dockerfile
// @Description  Analyze source code and generate an optimized Dockerfile
// @Tags         builder
// @Accept       json
// @Produce      json
// @Param        request  body      GenerateDockerfileRequest  true  "Source path"
// @Success      200      {object}  GenerateDockerfileResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /builds/generate-dockerfile [post]
func (h *BuilderHandler) GenerateDockerfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req GenerateDockerfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode generate dockerfile request")
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.SourcePath == "" {
		RespondWithError(w, http.StatusBadRequest, "source_path is required")
		return
	}

	log.Info().Str("sourcePath", req.SourcePath).Msg("Generating Dockerfile")

	// Analyze source code
	analysis, err := h.analyzer.Analyze(req.SourcePath)
	if err != nil {
		log.Error().Err(err).Str("sourcePath", req.SourcePath).Msg("Failed to analyze source code")
		RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to analyze source code: %v", err))
		return
	}

	// Generate Dockerfile
	generator := dockerfile.NewGenerator()
	dockerfileContent, err := generator.Generate(ctx, analysis)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate Dockerfile")
		RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate Dockerfile: %v", err))
		return
	}

	response := GenerateDockerfileResponse{
		Dockerfile: dockerfileContent,
		Language:   string(analysis.Language),
		Framework:  string(analysis.Framework),
		Analysis:   analysis,
	}

	RespondWithJSON(w, http.StatusOK, response)
}
