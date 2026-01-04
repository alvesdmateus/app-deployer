package state

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Deployment represents a deployment in the system
type Deployment struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name        string    `gorm:"not null;index"`
	AppName     string    `gorm:"not null"`
	Version     string    `gorm:"not null"`
	Status      string    `gorm:"not null;index"` // PENDING, BUILDING, PROVISIONING, DEPLOYING, EXPOSED, FAILED
	Cloud       string    `gorm:"not null"`       // gcp, aws, azure
	Region      string    `gorm:"not null"`
	ExternalIP  string
	ExternalURL string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeployedAt  *time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`

	// Relationships
	Infrastructure *Infrastructure `gorm:"foreignKey:DeploymentID"`
	Builds         []Build         `gorm:"foreignKey:DeploymentID"`
}

// Infrastructure represents the provisioned infrastructure for a deployment
type Infrastructure struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	DeploymentID uuid.UUID `gorm:"type:uuid;not null;index"`
	ClusterName  string    `gorm:"not null"`
	Namespace    string    `gorm:"not null"`
	ServiceName  string
	Status       string    `gorm:"not null"` // PROVISIONING, READY, FAILED
	Config       string    `gorm:"type:jsonb"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

// Build represents a container build for a deployment
type Build struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	DeploymentID uuid.UUID `gorm:"type:uuid;not null;index"`
	ImageTag     string    `gorm:"not null"`
	Status       string    `gorm:"not null"` // PENDING, BUILDING, SUCCESS, FAILED
	BuildLog     string    `gorm:"type:text"`
	StartedAt    time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}
