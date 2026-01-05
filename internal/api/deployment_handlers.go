package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/rs/zerolog/log"
)

// DeploymentHandler handles deployment-related HTTP requests
type DeploymentHandler struct {
	repo *state.Repository
}

// NewDeploymentHandler creates a new deployment handler
func NewDeploymentHandler(repo *state.Repository) *DeploymentHandler {
	return &DeploymentHandler{repo: repo}
}

// CreateDeployment handles POST /api/v1/deployments
func (h *DeploymentHandler) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	var req CreateDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.Name == "" || req.AppName == "" || req.Version == "" {
		RespondWithError(w, http.StatusBadRequest, "Name, app_name, and version are required")
		return
	}

	if req.Cloud == "" {
		req.Cloud = "gcp" // default
	}

	if req.Region == "" {
		req.Region = "us-central1" // default
	}

	// Create deployment
	deployment := &state.Deployment{
		Name:    req.Name,
		AppName: req.AppName,
		Version: req.Version,
		Status:  "PENDING",
		Cloud:   req.Cloud,
		Region:  req.Region,
	}

	if err := h.repo.CreateDeployment(r.Context(), deployment); err != nil {
		log.Error().Err(err).Msg("Failed to create deployment")
		RespondWithError(w, http.StatusInternalServerError, "Failed to create deployment")
		return
	}

	response := DeploymentToResponse(deployment)
	RespondWithJSON(w, http.StatusCreated, response)
}

// GetDeployment handles GET /api/v1/deployments/{id}
func (h *DeploymentHandler) GetDeployment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid deployment ID")
		return
	}

	deployment, err := h.repo.GetDeployment(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("id", idStr).Msg("Failed to get deployment")
		RespondWithError(w, http.StatusNotFound, "Deployment not found")
		return
	}

	response := DeploymentToResponse(deployment)
	RespondWithJSON(w, http.StatusOK, response)
}

// ListDeployments handles GET /api/v1/deployments
func (h *DeploymentHandler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20 // default
	offset := 0

	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	deployments, err := h.repo.ListDeployments(r.Context(), limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list deployments")
		RespondWithError(w, http.StatusInternalServerError, "Failed to list deployments")
		return
	}

	response := ListDeploymentsResponse{
		Deployments: DeploymentsToResponse(deployments),
		Total:       len(deployments),
		Limit:       limit,
		Offset:      offset,
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// UpdateDeploymentStatus handles PATCH /api/v1/deployments/{id}/status
func (h *DeploymentHandler) UpdateDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid deployment ID")
		return
	}

	var req UpdateDeploymentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Status == "" {
		RespondWithError(w, http.StatusBadRequest, "Status is required")
		return
	}

	if err := h.repo.UpdateDeploymentStatus(r.Context(), id, req.Status); err != nil {
		log.Error().Err(err).Str("id", idStr).Msg("Failed to update deployment status")
		RespondWithError(w, http.StatusInternalServerError, "Failed to update deployment status")
		return
	}

	RespondWithSuccess(w, http.StatusOK, "Deployment status updated", nil)
}

// DeleteDeployment handles DELETE /api/v1/deployments/{id}
func (h *DeploymentHandler) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid deployment ID")
		return
	}

	if err := h.repo.DeleteDeployment(r.Context(), id); err != nil {
		log.Error().Err(err).Str("id", idStr).Msg("Failed to delete deployment")
		RespondWithError(w, http.StatusInternalServerError, "Failed to delete deployment")
		return
	}

	RespondWithSuccess(w, http.StatusOK, "Deployment deleted", nil)
}

// GetDeploymentsByStatus handles GET /api/v1/deployments/status/{status}
func (h *DeploymentHandler) GetDeploymentsByStatus(w http.ResponseWriter, r *http.Request) {
	status := chi.URLParam(r, "status")
	if status == "" {
		RespondWithError(w, http.StatusBadRequest, "Status is required")
		return
	}

	deployments, err := h.repo.GetDeploymentsByStatus(r.Context(), status)
	if err != nil {
		log.Error().Err(err).Str("status", status).Msg("Failed to get deployments by status")
		RespondWithError(w, http.StatusInternalServerError, "Failed to get deployments")
		return
	}

	response := DeploymentsToResponse(deployments)
	RespondWithJSON(w, http.StatusOK, response)
}
