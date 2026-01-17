package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/alvesdmateus/app-deployer/internal/orchestrator"
	"github.com/alvesdmateus/app-deployer/internal/queue"
	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// DeploymentHandler handles deployment-related HTTP requests
type DeploymentHandler struct {
	repo       *state.Repository
	orchClient *orchestrator.Client
}

// NewDeploymentHandler creates a new deployment handler
func NewDeploymentHandler(repo *state.Repository, orchClient *orchestrator.Client) *DeploymentHandler {
	return &DeploymentHandler{
		repo:       repo,
		orchClient: orchClient,
	}
}

// CreateDeployment handles POST /api/v1/deployments
// @Summary      Create a new deployment
// @Description  Create a new deployment configuration. Optionally triggers immediate provisioning if image_tag is provided.
// @Tags         deployments
// @Accept       json
// @Produce      json
// @Param        deployment  body      CreateDeploymentRequest  true  "Deployment configuration"
// @Success      201         {object}  DeploymentResponse
// @Failure      400         {object}  ErrorResponse
// @Failure      500         {object}  ErrorResponse
// @Router       /deployments [post]
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

	// Set default port
	port := req.Port
	if port == 0 {
		port = 8080
	}

	// Create deployment
	deployment := &state.Deployment{
		Name:    req.Name,
		AppName: req.AppName,
		Version: req.Version,
		Status:  "PENDING",
		Cloud:   req.Cloud,
		Region:  req.Region,
		Port:    port,
	}

	if err := h.repo.CreateDeployment(r.Context(), deployment); err != nil {
		log.Error().Err(err).Msg("Failed to create deployment")
		RespondWithError(w, http.StatusInternalServerError, "Failed to create deployment")
		return
	}

	// Trigger provision job if orchestrator is available and image_tag is provided
	if h.orchClient != nil && req.ImageTag != "" {
		provisionPayload := &queue.ProvisionPayload{
			DeploymentID: deployment.ID.String(),
			AppName:      deployment.AppName,
			Version:      deployment.Version,
			Cloud:        deployment.Cloud,
			Region:       deployment.Region,
			ImageTag:     req.ImageTag,
		}

		if err := h.orchClient.TriggerProvision(r.Context(), provisionPayload); err != nil {
			log.Error().Err(err).
				Str("deployment_id", deployment.ID.String()).
				Msg("Failed to trigger provision job")
			// Update status to indicate failure
			_ = h.repo.UpdateDeploymentStatus(r.Context(), deployment.ID, "FAILED")
			deployment.Status = "FAILED"
			deployment.Error = "Failed to start provisioning: " + err.Error()
			RespondWithError(w, http.StatusInternalServerError,
				"Deployment created but provisioning failed to start")
			return
		}

		// Update status to QUEUED
		_ = h.repo.UpdateDeploymentStatus(r.Context(), deployment.ID, "QUEUED")
		deployment.Status = "QUEUED"
	}

	response := DeploymentToResponse(deployment)
	RespondWithJSON(w, http.StatusCreated, response)
}

// GetDeployment handles GET /api/v1/deployments/{id}
// @Summary      Get a deployment by ID
// @Description  Retrieve detailed information about a specific deployment
// @Tags         deployments
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Deployment ID (UUID)"
// @Success      200  {object}  DeploymentResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Router       /deployments/{id} [get]
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
// @Summary      List all deployments
// @Description  Retrieve a paginated list of all deployments
// @Tags         deployments
// @Accept       json
// @Produce      json
// @Param        limit   query     int     false  "Maximum number of results (default: 20)"
// @Param        offset  query     int     false  "Number of results to skip (default: 0)"
// @Success      200     {object}  ListDeploymentsResponse
// @Failure      500     {object}  ErrorResponse
// @Router       /deployments [get]
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
// @Summary      Update deployment status
// @Description  Manually update the status of a deployment
// @Tags         deployments
// @Accept       json
// @Produce      json
// @Param        id      path      string                       true  "Deployment ID (UUID)"
// @Param        status  body      UpdateDeploymentStatusRequest  true  "New status"
// @Success      200     {object}  SuccessResponse
// @Failure      400     {object}  ErrorResponse
// @Failure      500     {object}  ErrorResponse
// @Router       /deployments/{id}/status [patch]
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
// @Summary      Delete a deployment
// @Description  Delete a deployment and its associated infrastructure. If infrastructure exists, triggers async destruction.
// @Tags         deployments
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Deployment ID (UUID)"
// @Success      200  {object}  SuccessResponse  "Deployment deleted (no infrastructure)"
// @Success      202  {object}  SuccessResponse  "Destruction initiated (has infrastructure)"
// @Failure      400  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /deployments/{id} [delete]
func (h *DeploymentHandler) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid deployment ID")
		return
	}

	// Get deployment to check for infrastructure
	deployment, err := h.repo.GetDeployment(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("id", idStr).Msg("Deployment not found")
		RespondWithError(w, http.StatusNotFound, "Deployment not found")
		return
	}

	// If deployment has infrastructure, trigger destroy job
	if h.orchClient != nil && deployment.InfrastructureID != nil {
		destroyPayload := &queue.DestroyPayload{
			DeploymentID:     id.String(),
			InfrastructureID: deployment.InfrastructureID.String(),
		}

		if err := h.orchClient.TriggerDestroy(r.Context(), destroyPayload); err != nil {
			log.Error().Err(err).
				Str("deployment_id", idStr).
				Msg("Failed to trigger destroy job")
			RespondWithError(w, http.StatusInternalServerError,
				"Failed to initiate destruction process")
			return
		}

		// Update status to DESTROYING
		_ = h.repo.UpdateDeploymentStatus(r.Context(), id, "DESTROYING")

		RespondWithSuccess(w, http.StatusAccepted,
			"Destruction initiated. Infrastructure will be cleaned up asynchronously.", nil)
		return
	}

	// No infrastructure, just delete from database
	if err := h.repo.DeleteDeployment(r.Context(), id); err != nil {
		log.Error().Err(err).Str("id", idStr).Msg("Failed to delete deployment")
		RespondWithError(w, http.StatusInternalServerError, "Failed to delete deployment")
		return
	}

	RespondWithSuccess(w, http.StatusOK, "Deployment deleted", nil)
}

// GetDeploymentsByStatus handles GET /api/v1/deployments/status/{status}
// @Summary      Get deployments by status
// @Description  Retrieve all deployments with a specific status
// @Tags         deployments
// @Accept       json
// @Produce      json
// @Param        status  path      string  true  "Deployment status (PENDING, QUEUED, BUILDING, PROVISIONING, DEPLOYING, HEALTHY, FAILED, etc.)"
// @Success      200     {array}   DeploymentResponse
// @Failure      400     {object}  ErrorResponse
// @Failure      500     {object}  ErrorResponse
// @Router       /deployments/status/{status} [get]
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

// StartDeployment handles POST /api/v1/deployments/{id}/deploy
// @Summary      Start deployment
// @Description  Trigger infrastructure provisioning and application deployment with a built container image
// @Tags         deployments
// @Accept       json
// @Produce      json
// @Param        id       path      string                  true  "Deployment ID (UUID)"
// @Param        request  body      StartDeploymentRequest  true  "Deployment configuration"
// @Success      202      {object}  OrchestrationResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      404      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Failure      503      {object}  ErrorResponse  "Orchestration service unavailable"
// @Router       /deployments/{id}/deploy [post]
func (h *DeploymentHandler) StartDeployment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid deployment ID")
		return
	}

	var req StartDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ImageTag == "" {
		RespondWithError(w, http.StatusBadRequest, "image_tag is required")
		return
	}

	// Get deployment
	deployment, err := h.repo.GetDeployment(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("id", idStr).Msg("Deployment not found")
		RespondWithError(w, http.StatusNotFound, "Deployment not found")
		return
	}

	// Check if orchestrator is available
	if h.orchClient == nil {
		RespondWithError(w, http.StatusServiceUnavailable,
			"Orchestration service unavailable")
		return
	}

	// Set defaults for port and replicas
	port := req.Port
	if port == 0 {
		port = deployment.Port
		if port == 0 {
			port = 8080
		}
	}

	// Trigger provision job with image tag
	provisionPayload := &queue.ProvisionPayload{
		DeploymentID: deployment.ID.String(),
		AppName:      deployment.AppName,
		Version:      deployment.Version,
		Cloud:        deployment.Cloud,
		Region:       deployment.Region,
		ImageTag:     req.ImageTag,
	}

	if err := h.orchClient.TriggerProvision(r.Context(), provisionPayload); err != nil {
		log.Error().Err(err).
			Str("deployment_id", idStr).
			Msg("Failed to trigger provision job")
		RespondWithError(w, http.StatusInternalServerError,
			"Failed to start deployment")
		return
	}

	// Update deployment status and port
	deployment.Port = port
	_ = h.repo.UpdateDeploymentStatus(r.Context(), id, "QUEUED")

	response := OrchestrationResponse{
		DeploymentID: idStr,
		Status:       "QUEUED",
		Message:      "Deployment started. Infrastructure will be provisioned and application deployed.",
	}
	RespondWithJSON(w, http.StatusAccepted, response)
}

// TriggerRollback handles POST /api/v1/deployments/{id}/rollback
// @Summary      Trigger rollback
// @Description  Rollback a deployment to a previous version
// @Tags         deployments
// @Accept       json
// @Produce      json
// @Param        id       path      string                  true  "Deployment ID (UUID)"
// @Param        request  body      TriggerRollbackRequest  true  "Rollback configuration"
// @Success      202      {object}  OrchestrationResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      404      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Failure      503      {object}  ErrorResponse  "Orchestration service unavailable"
// @Router       /deployments/{id}/rollback [post]
func (h *DeploymentHandler) TriggerRollback(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid deployment ID")
		return
	}

	var req TriggerRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.TargetVersion == "" {
		RespondWithError(w, http.StatusBadRequest, "target_version is required")
		return
	}

	// Get deployment
	deployment, err := h.repo.GetDeployment(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("id", idStr).Msg("Deployment not found")
		RespondWithError(w, http.StatusNotFound, "Deployment not found")
		return
	}

	// Check if deployment has infrastructure
	if deployment.InfrastructureID == nil {
		RespondWithError(w, http.StatusBadRequest,
			"Deployment has no infrastructure to rollback")
		return
	}

	// Check if orchestrator is available
	if h.orchClient == nil {
		RespondWithError(w, http.StatusServiceUnavailable,
			"Orchestration service unavailable")
		return
	}

	// Trigger rollback job
	rollbackPayload := &queue.RollbackPayload{
		DeploymentID:  idStr,
		TargetVersion: req.TargetVersion,
		TargetTag:     req.TargetTag,
	}

	if err := h.orchClient.TriggerRollback(r.Context(), rollbackPayload); err != nil {
		log.Error().Err(err).
			Str("deployment_id", idStr).
			Msg("Failed to trigger rollback job")
		RespondWithError(w, http.StatusInternalServerError, "Failed to start rollback")
		return
	}

	// Update deployment status
	_ = h.repo.UpdateDeploymentStatus(r.Context(), id, "ROLLING_BACK")

	response := OrchestrationResponse{
		DeploymentID: idStr,
		Status:       "ROLLING_BACK",
		Message:      fmt.Sprintf("Rollback to version %s initiated", req.TargetVersion),
	}
	RespondWithJSON(w, http.StatusAccepted, response)
}

// GetQueueStats handles GET /api/v1/orchestrator/stats
// @Summary      Get queue statistics
// @Description  Retrieve statistics about the job queues (provision, deploy, destroy, rollback)
// @Tags         orchestrator
// @Accept       json
// @Produce      json
// @Success      200  {object}  QueueStatsResponse
// @Failure      500  {object}  ErrorResponse
// @Failure      503  {object}  ErrorResponse  "Orchestration service unavailable"
// @Router       /orchestrator/stats [get]
func (h *DeploymentHandler) GetQueueStats(w http.ResponseWriter, r *http.Request) {
	if h.orchClient == nil {
		RespondWithError(w, http.StatusServiceUnavailable,
			"Orchestration service unavailable")
		return
	}

	stats, err := h.orchClient.GetQueueStats(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get queue stats")
		RespondWithError(w, http.StatusInternalServerError, "Failed to get queue statistics")
		return
	}

	response := QueueStatsResponse{
		Provision: stats["provision"],
		Deploy:    stats["deploy"],
		Destroy:   stats["destroy"],
		Rollback:  stats["rollback"],
	}
	RespondWithJSON(w, http.StatusOK, response)
}

// GetDeploymentLogs handles GET /api/v1/deployments/{id}/logs
// @Summary      Get deployment logs
// @Description  Retrieve logs for a specific deployment, optionally filtered by phase
// @Tags         deployments
// @Accept       json
// @Produce      json
// @Param        id      path      string  true   "Deployment ID (UUID)"
// @Param        phase   query     string  false  "Filter by phase (BUILDING, PROVISIONING, DEPLOYING, etc.)"
// @Param        limit   query     int     false  "Maximum number of results (default: 100, max: 1000)"
// @Param        offset  query     int     false  "Number of results to skip (default: 0)"
// @Success      200     {object}  ListDeploymentLogsResponse
// @Failure      400     {object}  ErrorResponse
// @Failure      404     {object}  ErrorResponse
// @Failure      500     {object}  ErrorResponse
// @Router       /deployments/{id}/logs [get]
func (h *DeploymentHandler) GetDeploymentLogs(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid deployment ID")
		return
	}

	// Verify deployment exists
	_, err = h.repo.GetDeployment(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("id", idStr).Msg("Deployment not found")
		RespondWithError(w, http.StatusNotFound, "Deployment not found")
		return
	}

	// Parse query parameters
	phase := r.URL.Query().Get("phase")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 100 // default
	offset := 0

	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > 1000 {
				limit = 1000 // max limit
			}
		}
	}

	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get logs from database
	logs, err := h.repo.GetDeploymentLogs(r.Context(), id, phase, limit, offset)
	if err != nil {
		log.Error().Err(err).Str("id", idStr).Msg("Failed to get deployment logs")
		RespondWithError(w, http.StatusInternalServerError, "Failed to get deployment logs")
		return
	}

	// Get total count
	total, err := h.repo.CountDeploymentLogs(r.Context(), id, phase)
	if err != nil {
		log.Error().Err(err).Str("id", idStr).Msg("Failed to count deployment logs")
		RespondWithError(w, http.StatusInternalServerError, "Failed to get deployment logs")
		return
	}

	// Convert to response
	logResponses := make([]DeploymentLogResponse, len(logs))
	for i, l := range logs {
		logResponses[i] = DeploymentLogResponse{
			ID:        l.ID,
			JobID:     l.JobID,
			Phase:     l.Phase,
			Level:     l.Level,
			Message:   l.Message,
			Details:   l.Details,
			Timestamp: l.Timestamp,
		}
	}

	response := ListDeploymentLogsResponse{
		Logs:   logResponses,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	RespondWithJSON(w, http.StatusOK, response)
}
