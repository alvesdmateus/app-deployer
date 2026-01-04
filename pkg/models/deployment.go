package models

import (
	"time"

	"github.com/google/uuid"
)

// DeploymentStatus represents the current state of a deployment
type DeploymentStatus string

const (
	StatusQueued       DeploymentStatus = "QUEUED"
	StatusAnalyzing    DeploymentStatus = "ANALYZING"
	StatusBuilding     DeploymentStatus = "BUILDING"
	StatusProvisioning DeploymentStatus = "PROVISIONING"
	StatusDeploying    DeploymentStatus = "DEPLOYING"
	StatusHealthy      DeploymentStatus = "HEALTHY"
	StatusExposed      DeploymentStatus = "EXPOSED"
	StatusFailed       DeploymentStatus = "FAILED"
	StatusRollingBack  DeploymentStatus = "ROLLING_BACK"
)

// Deployment represents a deployed application
type Deployment struct {
	ID            uuid.UUID        `json:"id" gorm:"type:uuid;primary_key"`
	RepoURL       string           `json:"repo_url" gorm:"not null"`
	GitSHA        string           `json:"git_sha"`
	Status        DeploymentStatus `json:"status" gorm:"not null"`
	CloudProvider string           `json:"cloud_provider"` // gcp, aws, azure
	Region        string           `json:"region"`
	ExternalURL   string           `json:"external_url"`
	ExternalIP    string           `json:"external_ip"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
	DeployedAt    *time.Time       `json:"deployed_at"`
	Metadata      string           `json:"metadata" gorm:"type:jsonb"` // Additional flexible storage
}

// DeploymentRequest represents the request to create a deployment
type DeploymentRequest struct {
	RepoURL       string `json:"repo_url" validate:"required,url"`
	CloudProvider string `json:"cloud_provider,omitempty"` // Optional, uses default if not specified
	Region        string `json:"region,omitempty"`
}

// DeploymentResponse represents the response after creating a deployment
type DeploymentResponse struct {
	ID        uuid.UUID        `json:"id"`
	Status    DeploymentStatus `json:"status"`
	RepoURL   string           `json:"repo_url"`
	CreatedAt time.Time        `json:"created_at"`
}
