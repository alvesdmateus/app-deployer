package api

import (
	"net/http"

	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// InfrastructureHandler handles infrastructure-related HTTP requests
type InfrastructureHandler struct {
	repo *state.Repository
}

// NewInfrastructureHandler creates a new infrastructure handler
func NewInfrastructureHandler(repo *state.Repository) *InfrastructureHandler {
	return &InfrastructureHandler{repo: repo}
}

// GetInfrastructure handles GET /api/v1/deployments/{id}/infrastructure
func (h *InfrastructureHandler) GetInfrastructure(w http.ResponseWriter, r *http.Request) {
	deploymentIDStr := chi.URLParam(r, "id")
	deploymentID, err := uuid.Parse(deploymentIDStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid deployment ID")
		return
	}

	infra, err := h.repo.GetInfrastructure(r.Context(), deploymentID)
	if err != nil {
		log.Error().Err(err).Str("deployment_id", deploymentIDStr).Msg("Failed to get infrastructure")
		RespondWithError(w, http.StatusNotFound, "Infrastructure not found")
		return
	}

	response := InfrastructureToResponse(infra)
	RespondWithJSON(w, http.StatusOK, response)
}
