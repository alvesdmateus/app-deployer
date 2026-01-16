package deployer

import (
	"context"
	"time"
)

// Deployer defines the interface for Kubernetes deployment
type Deployer interface {
	// Deploy deploys an application to Kubernetes
	Deploy(ctx context.Context, req *DeployRequest) (*DeployResult, error)

	// Destroy removes an application from Kubernetes
	Destroy(ctx context.Context, req *DestroyRequest) error

	// GetStatus checks deployment status
	GetStatus(ctx context.Context, namespace, releaseName string) (*DeploymentStatus, error)

	// Rollback rolls back to a previous version
	Rollback(ctx context.Context, req *RollbackRequest) error
}

// DeployRequest contains information needed to deploy an application
type DeployRequest struct {
	DeploymentID     string
	InfrastructureID string
	AppName          string
	Version          string

	// Container image
	ImageTag string

	// Application configuration
	Port     int
	Replicas int

	// Resource limits
	CPURequest    string
	CPULimit      string
	MemoryRequest string
	MemoryLimit   string

	// Environment variables
	Env map[string]string

	// Optional configuration
	Config *DeployConfig
}

// DeployConfig holds optional deployment configuration
type DeployConfig struct {
	// Service configuration
	ServiceType string // LoadBalancer, NodePort, ClusterIP

	// Ingress configuration
	EnableIngress bool
	IngressHost   string
	IngressTLS    bool

	// Health checks
	HealthCheckPath string
	ReadinessPath   string
	LivenessPath    string

	// Advanced options
	EnableHPA    bool // Horizontal Pod Autoscaler
	MinReplicas  int
	MaxReplicas  int
	TargetCPU    int // Target CPU utilization percentage
	TargetMemory int // Target memory utilization percentage

	// Labels and annotations
	Labels      map[string]string
	Annotations map[string]string
}

// DeployResult contains the result of a deployment
type DeployResult struct {
	ReleaseName string
	Namespace   string
	ExternalIP  string
	ExternalURL string
	IngressURL  string
	Status      string
	Message     string
	Duration    time.Duration
}

// DestroyRequest contains information for destroying a deployment
type DestroyRequest struct {
	DeploymentID     string
	InfrastructureID string
	Namespace        string
	ReleaseName      string
}

// RollbackRequest contains information for rolling back a deployment
type RollbackRequest struct {
	DeploymentID     string
	InfrastructureID string
	Namespace        string
	ReleaseName      string
	Revision         int // 0 for previous revision
}

// DeploymentStatus represents the current status of a deployment
type DeploymentStatus struct {
	ReleaseName   string
	Namespace     string
	Status        string // deployed, failed, pending
	Revision      int
	UpdatedAt     time.Time
	ReadyReplicas int
	TotalReplicas int
	ExternalIP    string
}
