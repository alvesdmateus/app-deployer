package state

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Deployment represents a deployment in the system
type Deployment struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Name             string     `gorm:"not null;index"`
	AppName          string     `gorm:"not null"`
	Version          string     `gorm:"not null"`
	Status           string     `gorm:"not null;index"` // PENDING, BUILDING, PROVISIONING, DEPLOYING, EXPOSED, FAILED
	Cloud            string     `gorm:"not null"`       // gcp, aws, azure
	Region           string     `gorm:"not null"`
	Port             int        `gorm:"default:8080"`    // Application port
	InfrastructureID *uuid.UUID `gorm:"type:uuid;index"` // Reference to infrastructure
	ExternalIP       string
	ExternalURL      string
	Error            string `gorm:"type:text"` // Last error message
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeployedAt       *time.Time
	DeletedAt        gorm.DeletedAt `gorm:"index"`

	// Relationships
	Infrastructure *Infrastructure `gorm:"foreignKey:DeploymentID"`
	Builds         []Build         `gorm:"foreignKey:DeploymentID"`
}

// Infrastructure represents the provisioned infrastructure for a deployment
type Infrastructure struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	DeploymentID uuid.UUID `gorm:"type:uuid;not null;index"`

	// Basic cluster info
	ClusterName string `gorm:"not null"`
	Namespace   string `gorm:"not null"`
	ServiceName string
	Status      string `gorm:"not null"`                // PROVISIONING, READY, FAILED, DESTROYING
	Config      string `gorm:"type:jsonb;default:'{}'"` // JSON config, defaults to empty object

	// Pulumi state tracking
	PulumiStackName   string `gorm:"index"` // Unique stack name for idempotency
	PulumiProjectName string
	PulumiBackendURL  string

	// Cloud provider info
	CloudProvider string // gcp, aws, azure
	GCPProject    string // GCP project ID (for gcloud commands)

	// GCP resources
	VPCName    string
	VPCNetwork string // Full VPC network path
	SubnetName string
	SubnetCIDR string
	RouterName string
	NATName    string

	// GKE cluster details
	ClusterEndpoint     string
	ClusterCACert       string `gorm:"type:text"` // Base64 encoded CA cert
	ClusterLocation     string // GCP region/zone
	NodePoolName        string
	NodeCount           int `gorm:"default:2"`
	ServiceAccountEmail string

	// Kubernetes deployment details (from deployer phase)
	KubeNamespace   string // K8s namespace
	HelmReleaseName string // Helm release name
	ExternalIP      string // LoadBalancer external IP

	// Error tracking
	LastError    string `gorm:"type:text"` // Last error message
	ProvisionLog string `gorm:"type:text"` // Provision operation logs

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
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

// DeploymentLog represents a log entry for deployment operations
type DeploymentLog struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	DeploymentID uuid.UUID `gorm:"type:uuid;not null;index"`
	JobID        string    `gorm:"index"`                   // Job ID for correlation
	Phase        string    `gorm:"not null;index"`          // QUEUED, PROVISIONING, DEPLOYING, DESTROYING, ROLLING_BACK
	Level        string    `gorm:"not null;default:INFO"`   // DEBUG, INFO, WARN, ERROR
	Message      string    `gorm:"type:text;not null"`      // Log message
	Details      string    `gorm:"type:jsonb;default:'{}'"` // Additional structured details (JSON)
	Timestamp    time.Time `gorm:"not null;index"`          // When the log was created
	CreatedAt    time.Time
}

// User represents an authenticated user in the system
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email        string    `gorm:"not null;uniqueIndex"`
	PasswordHash string    `gorm:"not null"`
	Name         string
	Role         string         `gorm:"not null;default:user"` // admin, user
	Active       bool           `gorm:"not null;default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`

	// Relationships
	APIKeys []APIKey `gorm:"foreignKey:UserID"`
}

// APIKey represents an API key for programmatic access
type APIKey struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Name      string    `gorm:"not null"`           // Friendly name for the key
	KeyHash   string    `gorm:"not null;index"`     // SHA-256 hash of the key
	KeyPrefix string    `gorm:"not null"`           // First 8 chars for identification
	Scopes    string    `gorm:"type:text"`          // Comma-separated list of scopes
	ExpiresAt *time.Time
	LastUsed  *time.Time
	Active    bool           `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
