package api

import (
	"time"

	"github.com/google/uuid"
)

// CreateDeploymentRequest represents a request to create a new deployment
type CreateDeploymentRequest struct {
	Name     string `json:"name"`
	AppName  string `json:"app_name"`
	Version  string `json:"version"`
	Cloud    string `json:"cloud"`
	Region   string `json:"region"`
	ImageTag string `json:"image_tag,omitempty"` // Optional: if provided, triggers immediate provisioning
	Port     int    `json:"port,omitempty"`      // Optional: defaults to 8080
}

// UpdateDeploymentStatusRequest represents a request to update deployment status
type UpdateDeploymentStatusRequest struct {
	Status string `json:"status"`
}

// DeploymentResponse represents a deployment in API responses
type DeploymentResponse struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	AppName     string     `json:"app_name"`
	Version     string     `json:"version"`
	Status      string     `json:"status"`
	Cloud       string     `json:"cloud"`
	Region      string     `json:"region"`
	ExternalIP  string     `json:"external_ip,omitempty"`
	ExternalURL string     `json:"external_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeployedAt  *time.Time `json:"deployed_at,omitempty"`
}

// InfrastructureResponse represents infrastructure in API responses
type InfrastructureResponse struct {
	ID           uuid.UUID `json:"id"`
	DeploymentID uuid.UUID `json:"deployment_id"`
	ClusterName  string    `json:"cluster_name"`
	Namespace    string    `json:"namespace"`
	ServiceName  string    `json:"service_name,omitempty"`
	Status       string    `json:"status"`
	Config       string    `json:"config,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// BuildResponse represents a build in API responses
type BuildResponse struct {
	ID           uuid.UUID  `json:"id"`
	DeploymentID uuid.UUID  `json:"deployment_id"`
	ImageTag     string     `json:"image_tag"`
	Status       string     `json:"status"`
	BuildLog     string     `json:"build_log,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse represents a generic success response
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Version  string `json:"version"`
}

// ListDeploymentsResponse represents a paginated list of deployments
type ListDeploymentsResponse struct {
	Deployments []DeploymentResponse `json:"deployments"`
	Total       int                  `json:"total"`
	Limit       int                  `json:"limit"`
	Offset      int                  `json:"offset"`
}

// StartDeploymentRequest represents a request to start deployment (after build)
type StartDeploymentRequest struct {
	ImageTag string `json:"image_tag"` // Required: container image to deploy
	Port     int    `json:"port"`      // Optional: defaults to 8080
	Replicas int    `json:"replicas"`  // Optional: defaults to 2
}

// TriggerRollbackRequest represents a request to rollback a deployment
type TriggerRollbackRequest struct {
	TargetVersion string `json:"target_version"`        // Required: version to rollback to
	TargetTag     string `json:"target_tag,omitempty"`  // Optional: specific image tag
}

// OrchestrationResponse represents a response for async orchestration operations
type OrchestrationResponse struct {
	DeploymentID string `json:"deployment_id"`
	Status       string `json:"status"`
	Message      string `json:"message"`
}

// QueueStatsResponse represents queue statistics
type QueueStatsResponse struct {
	Provision int64 `json:"provision"`
	Deploy    int64 `json:"deploy"`
	Destroy   int64 `json:"destroy"`
	Rollback  int64 `json:"rollback"`
}

// DeploymentLogResponse represents a deployment log entry in API responses
type DeploymentLogResponse struct {
	ID        uuid.UUID `json:"id"`
	JobID     string    `json:"job_id,omitempty"`
	Phase     string    `json:"phase"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ListDeploymentLogsResponse represents a paginated list of deployment logs
type ListDeploymentLogsResponse struct {
	Logs   []DeploymentLogResponse `json:"logs"`
	Total  int64                   `json:"total"`
	Limit  int                     `json:"limit"`
	Offset int                     `json:"offset"`
}
