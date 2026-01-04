package api

import (
	"time"

	"github.com/google/uuid"
)

// CreateDeploymentRequest represents a request to create a new deployment
type CreateDeploymentRequest struct {
	Name    string `json:"name"`
	AppName string `json:"app_name"`
	Version string `json:"version"`
	Cloud   string `json:"cloud"`
	Region  string `json:"region"`
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
