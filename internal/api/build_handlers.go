package api

import (
	"net/http"

	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// BuildHandler handles build-related HTTP requests
type BuildHandler struct {
	repo *state.Repository
}

// NewBuildHandler creates a new build handler
func NewBuildHandler(repo *state.Repository) *BuildHandler {
	return &BuildHandler{repo: repo}
}

// GetLatestBuild handles GET /api/v1/deployments/{deployment_id}/builds/latest
func (h *BuildHandler) GetLatestBuild(w http.ResponseWriter, r *http.Request) {
	deploymentIDStr := chi.URLParam(r, "deployment_id")
	deploymentID, err := uuid.Parse(deploymentIDStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid deployment ID")
		return
	}

	build, err := h.repo.GetLatestBuild(r.Context(), deploymentID)
	if err != nil {
		log.Error().Err(err).Str("deployment_id", deploymentIDStr).Msg("Failed to get latest build")
		RespondWithError(w, http.StatusNotFound, "Build not found")
		return
	}

	response := BuildToResponse(build)
	RespondWithJSON(w, http.StatusOK, response)
}
