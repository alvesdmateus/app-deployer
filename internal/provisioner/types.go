package provisioner

import (
	"context"
	"time"

	"github.com/alvesdmateus/app-deployer/internal/analyzer"
)

// Provisioner defines the interface for infrastructure provisioning
type Provisioner interface {
	// Provision creates all required infrastructure
	Provision(ctx context.Context, req *ProvisionRequest) (*ProvisionResult, error)

	// Destroy tears down all infrastructure
	Destroy(ctx context.Context, req *DestroyRequest) error

	// GetStatus checks infrastructure status
	GetStatus(ctx context.Context, stackName string) (*InfrastructureStatus, error)

	// VerifyAccess verifies cloud provider access
	VerifyAccess(ctx context.Context) error
}

// ProvisionRequest contains all info needed to provision infrastructure
type ProvisionRequest struct {
	DeploymentID string
	AppName      string
	Version      string
	Cloud        string // "gcp", "aws", "azure"
	Region       string

	// Analysis results to determine resource requirements
	Analysis *analyzer.AnalysisResult

	// Optional configuration overrides
	Config *ProvisionConfig

	// Infrastructure ID (if already created)
	InfrastructureID string
}

// ProvisionConfig holds optional configuration for provisioning
type ProvisionConfig struct {
	// GKE configuration
	NodeCount     int
	MachineType   string
	Preemptible   bool
	DiskSize      int
	DiskType      string

	// Resource labels
	Labels map[string]string

	// Networking
	VPCCIDRBlock    string
	SubnetCIDRBlock string

	// Advanced options
	EnableAutoScaling bool
	MinNodes          int
	MaxNodes          int
}

// ProvisionResult contains the result of provisioning
type ProvisionResult struct {
	InfrastructureID string
	StackName        string

	// Cloud provider info
	CloudProvider string // gcp, aws, azure
	GCPProject    string // GCP project ID

	// Cluster info
	ClusterName     string
	ClusterEndpoint string
	ClusterCACert   string // Base64 encoded
	ClusterLocation string

	// Network info
	VPCName    string
	VPCNetwork string
	SubnetName string
	SubnetCIDR string
	RouterName string
	NATName    string

	// Other
	Namespace      string
	ServiceAccount string
	ProvisionLog   string
	Duration       time.Duration
}

// DestroyRequest contains info for destroying infrastructure
type DestroyRequest struct {
	DeploymentID     string
	InfrastructureID string
	StackName        string
}

// InfrastructureStatus represents current infrastructure state
type InfrastructureStatus struct {
	Status      string // "PROVISIONING", "READY", "FAILED", "DESTROYING"
	Resources   map[string]string
	LastUpdated time.Time
	Outputs     map[string]interface{}
}
