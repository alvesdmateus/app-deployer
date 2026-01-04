package state

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Deployment database model
type Deployment struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	RepoURL       string    `gorm:"not null"`
	GitSHA        string
	Status        string `gorm:"not null"`
	CloudProvider string
	Region        string
	ExternalURL   string
	ExternalIP    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeployedAt    *time.Time
	Metadata      string `gorm:"type:jsonb"`

	// Relationships
	Infrastructure *Infrastructure `gorm:"foreignKey:DeploymentID"`
	Builds         []Build         `gorm:"foreignKey:DeploymentID"`
}

// Infrastructure tracks cloud resources created for a deployment
type Infrastructure struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	DeploymentID     uuid.UUID `gorm:"type:uuid;not null"`
	PulumiStackName  string
	Resources        string    `gorm:"type:jsonb"` // JSON array of created resources
	StateBackendURL  string
	CreatedAt        time.Time
}

// Build tracks container image builds
type Build struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	DeploymentID uuid.UUID `gorm:"type:uuid;not null"`
	ImageTag     string
	BuildLogs    string
	StartedAt    time.Time
	CompletedAt  *time.Time
	Success      bool
}

// AutoMigrate runs database migrations
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Deployment{},
		&Infrastructure{},
		&Build{},
	)
}
