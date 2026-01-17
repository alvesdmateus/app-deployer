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

// GetInfrastructure handles GET /api/v1/deployments/{deployment_id}/infrastructure
// @Summary      Get infrastructure details
// @Description  Returns the infrastructure details for a specific deployment
// @Tags         infrastructure
// @Produce      json
// @Param        deployment_id  path      string  true  "Deployment ID"
// @Success      200            {object}  InfrastructureResponse
// @Failure      400            {object}  ErrorResponse
// @Failure      404            {object}  ErrorResponse
// @Router       /deployments/{deployment_id}/infrastructure [get]
func (h *InfrastructureHandler) GetInfrastructure(w http.ResponseWriter, r *http.Request) {
	deploymentIDStr := chi.URLParam(r, "deployment_id")
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
