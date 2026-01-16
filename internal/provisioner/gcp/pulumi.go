package gcp

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/rs/zerolog/log"

	"github.com/alvesdmateus/app-deployer/internal/provisioner"
)

// Default timeout for provisioning operations (20 minutes)
const DefaultProvisionTimeout = 20 * time.Minute

// GCPProvisioner implements the Provisioner interface for GCP using Pulumi
type GCPProvisioner struct {
	projectName      string
	gcpProject       string
	gcpRegion        string
	backendURL       string
	tracker          *provisioner.Tracker
	defaultNodes     int
	defaultType      string
	provisionTimeout time.Duration
}

// Config holds GCP provisioner configuration
type Config struct {
	GCPProject       string
	GCPRegion        string
	PulumiBackend    string
	DefaultNodeType  string
	DefaultNodes     int
	ProvisionTimeout time.Duration // Optional: defaults to 20 minutes
}

// NewGCPProvisioner creates a new GCP provisioner
func NewGCPProvisioner(config Config, tracker *provisioner.Tracker) (*GCPProvisioner, error) {
	if config.GCPProject == "" {
		return nil, fmt.Errorf("GCP project is required")
	}

	if config.GCPRegion == "" {
		config.GCPRegion = "us-central1"
	}

	if config.PulumiBackend == "" {
		return nil, fmt.Errorf("Pulumi backend URL is required")
	}

	if config.DefaultNodeType == "" {
		config.DefaultNodeType = "e2-small"
	}

	if config.DefaultNodes == 0 {
		config.DefaultNodes = 2
	}

	if config.ProvisionTimeout == 0 {
		config.ProvisionTimeout = DefaultProvisionTimeout
	}

	log.Info().
		Str("gcpProject", config.GCPProject).
		Str("gcpRegion", config.GCPRegion).
		Str("backendURL", config.PulumiBackend).
		Dur("provisionTimeout", config.ProvisionTimeout).
		Msg("GCP provisioner initialized")

	return &GCPProvisioner{
		projectName:      "app-deployer",
		gcpProject:       config.GCPProject,
		gcpRegion:        config.GCPRegion,
		backendURL:       config.PulumiBackend,
		tracker:          tracker,
		defaultNodes:     config.DefaultNodes,
		defaultType:      config.DefaultNodeType,
		provisionTimeout: config.ProvisionTimeout,
	}, nil
}

// VerifyAccess verifies that gcloud is authenticated
func (p *GCPProvisioner) VerifyAccess(ctx context.Context) error {
	log.Info().Msg("Verifying GCP access via gcloud CLI")

	// Check if gcloud is installed
	cmd := exec.CommandContext(ctx, "gcloud", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gcloud CLI not found: %w (please install gcloud)", err)
	}

	// Check if authenticated
	cmd = exec.CommandContext(ctx, "gcloud", "auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check gcloud auth: %w", err)
	}

	if len(strings.TrimSpace(string(output))) == 0 {
		return fmt.Errorf("no active gcloud authentication found (run: gcloud auth login)")
	}

	log.Info().
		Str("account", strings.TrimSpace(string(output))).
		Msg("GCP access verified")

	return nil
}

// Provision provisions GCP infrastructure using Pulumi
func (p *GCPProvisioner) Provision(ctx context.Context, req *provisioner.ProvisionRequest) (*provisioner.ProvisionResult, error) {
	startTime := time.Now()

	// Apply timeout to context
	ctx, cancel := context.WithTimeout(ctx, p.provisionTimeout)
	defer cancel()

	log.Info().
		Str("deploymentID", req.DeploymentID).
		Str("appName", req.AppName).
		Str("region", req.Region).
		Dur("timeout", p.provisionTimeout).
		Msg("Starting GCP infrastructure provisioning")

	// Verify access
	if err := p.VerifyAccess(ctx); err != nil {
		return nil, fmt.Errorf("GCP access verification failed: %w", err)
	}

	// Generate stack name
	stackName := generateStackName(req.DeploymentID)

	// Check for existing infrastructure (idempotency)
	existingInfra, err := p.tracker.GetInfrastructureByDeployment(ctx, req.DeploymentID)
	if err == nil && existingInfra != nil && existingInfra.Status == "READY" {
		log.Info().
			Str("infraID", existingInfra.ID.String()).
			Str("stackName", stackName).
			Msg("Infrastructure already exists and is ready, reusing")

		return &provisioner.ProvisionResult{
			InfrastructureID: existingInfra.ID.String(),
			StackName:        existingInfra.PulumiStackName,
			ClusterName:      existingInfra.ClusterName,
			ClusterEndpoint:  existingInfra.ClusterEndpoint,
			ClusterCACert:    existingInfra.ClusterCACert,
			ClusterLocation:  existingInfra.ClusterLocation,
			VPCName:          existingInfra.VPCName,
			VPCNetwork:       existingInfra.VPCNetwork,
			SubnetName:       existingInfra.SubnetName,
			SubnetCIDR:       existingInfra.SubnetCIDR,
			ServiceAccount:   existingInfra.ServiceAccountEmail,
			Namespace:        existingInfra.Namespace,
			Duration:         0,
		}, nil
	}

	// Start provisioning tracking
	infraID, err := p.tracker.StartProvisioning(ctx, req.DeploymentID, stackName)
	if err != nil {
		return nil, fmt.Errorf("failed to start provisioning tracking: %w", err)
	}

	// Convert request to internal format
	internalReq := p.convertRequest(req)

	// Create Pulumi program
	program := p.createPulumiProgram(internalReq)

	// Create or select stack
	stack, err := p.createOrSelectStack(ctx, stackName, program)
	if err != nil {
		p.tracker.FailProvisioning(ctx, infraID, err)
		return nil, fmt.Errorf("failed to create stack: %w", err)
	}

	// Set stack configuration
	if err := p.setStackConfig(ctx, stack, internalReq); err != nil {
		p.tracker.FailProvisioning(ctx, infraID, err)
		return nil, fmt.Errorf("failed to set stack config: %w", err)
	}

	// Run pulumi up with progress streaming
	log.Info().Str("stackName", stackName).Msg("Running pulumi up")

	upResult, err := stack.Up(ctx, optup.ProgressStreams(p.createProgressWriter(ctx, infraID)))
	if err != nil {
		p.tracker.FailProvisioning(ctx, infraID, err)
		return nil, fmt.Errorf("pulumi up failed: %w", err)
	}

	// Extract outputs
	result, err := p.extractOutputs(upResult, stackName, infraID)
	if err != nil {
		p.tracker.FailProvisioning(ctx, infraID, err)
		return nil, fmt.Errorf("failed to extract outputs: %w", err)
	}

	result.Duration = time.Since(startTime)

	// Update tracker with success
	if err := p.tracker.CompleteProvisioning(ctx, infraID, result); err != nil {
		return nil, fmt.Errorf("failed to update provisioning status: %w", err)
	}

	log.Info().
		Str("infraID", infraID).
		Str("clusterName", result.ClusterName).
		Dur("duration", result.Duration).
		Msg("GCP infrastructure provisioning completed successfully")

	return result, nil
}

// Destroy destroys GCP infrastructure
func (p *GCPProvisioner) Destroy(ctx context.Context, req *provisioner.DestroyRequest) error {
	log.Info().
		Str("deploymentID", req.DeploymentID).
		Str("stackName", req.StackName).
		Msg("Starting infrastructure destruction")

	// Verify access
	if err := p.VerifyAccess(ctx); err != nil {
		return fmt.Errorf("GCP access verification failed: %w", err)
	}

	// Create empty program for destroy
	program := pulumi.RunFunc(func(ctx *pulumi.Context) error {
		return nil
	})

	// Select existing stack
	stack, err := auto.SelectStackInlineSource(ctx, req.StackName, p.projectName, program,
		auto.Project(workspace.Project{
			Name: tokens.PackageName(p.projectName),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			Backend: &workspace.ProjectBackend{
				URL: p.backendURL,
			},
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to select stack for destroy: %w", err)
	}

	// Run pulumi destroy
	log.Info().Str("stackName", req.StackName).Msg("Running pulumi destroy")

	_, err = stack.Destroy(ctx, optdestroy.ProgressStreams(p.createProgressWriter(ctx, req.InfrastructureID)))
	if err != nil {
		return fmt.Errorf("pulumi destroy failed: %w", err)
	}

	// Remove the stack
	if err := stack.Workspace().RemoveStack(ctx, req.StackName); err != nil {
		log.Warn().Err(err).Msg("Failed to remove stack (may already be removed)")
	}

	log.Info().
		Str("deploymentID", req.DeploymentID).
		Str("stackName", req.StackName).
		Msg("Infrastructure destruction completed successfully")

	return nil
}

// GetStatus gets the status of infrastructure
func (p *GCPProvisioner) GetStatus(ctx context.Context, stackName string) (*provisioner.InfrastructureStatus, error) {
	// Create empty program
	program := pulumi.RunFunc(func(ctx *pulumi.Context) error {
		return nil
	})

	// Select existing stack
	stack, err := auto.SelectStackInlineSource(ctx, stackName, p.projectName, program,
		auto.Project(workspace.Project{
			Name: tokens.PackageName(p.projectName),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			Backend: &workspace.ProjectBackend{
				URL: p.backendURL,
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to select stack: %w", err)
	}

	// Get stack outputs
	outputs, err := stack.Outputs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get stack outputs: %w", err)
	}

	// Convert outputs
	outputMap := make(map[string]interface{})
	for k, v := range outputs {
		outputMap[k] = v.Value
	}

	return &provisioner.InfrastructureStatus{
		Status:      "READY",
		Resources:   make(map[string]string),
		LastUpdated: time.Now(),
		Outputs:     outputMap,
	}, nil
}

// createPulumiProgram creates the inline Pulumi program
func (p *GCPProvisioner) createPulumiProgram(req *ProvisionRequestInternal) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		log.Info().Msg("Creating VPC resources")

		// Create VPC resources
		vpcResources, err := createVPCResources(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to create VPC resources: %w", err)
		}

		log.Info().Msg("Creating firewall rules")

		// Create firewall rules
		if err := createFirewallRules(ctx, vpcResources.VPC, req); err != nil {
			return fmt.Errorf("failed to create firewall rules: %w", err)
		}

		log.Info().Msg("Creating GKE cluster")

		// Create GKE resources
		gkeResources, err := createGKEResources(ctx, vpcResources, req)
		if err != nil {
			return fmt.Errorf("failed to create GKE resources: %w", err)
		}

		log.Info().Msg("Exporting outputs")

		// Export outputs
		ctx.Export("vpcName", vpcResources.VPC.Name)
		ctx.Export("vpcNetwork", vpcResources.VPC.SelfLink)
		ctx.Export("subnetName", vpcResources.Subnet.Name)
		ctx.Export("subnetCIDR", vpcResources.Subnet.IpCidrRange)
		ctx.Export("routerName", vpcResources.Router.Name)
		ctx.Export("natName", vpcResources.NAT.Name)
		ctx.Export("clusterName", gkeResources.Cluster.Name)
		ctx.Export("clusterEndpoint", gkeResources.Cluster.Endpoint)
		ctx.Export("clusterCACert", gkeResources.Cluster.MasterAuth.ClusterCaCertificate())
		ctx.Export("clusterLocation", gkeResources.Cluster.Location)
		ctx.Export("serviceAccount", gkeResources.ServiceAccount.Email)
		ctx.Export("nodePoolName", gkeResources.NodePool.Name)
		ctx.Export("namespace", pulumi.String(generateNamespace(req.AppName, req.DeploymentID)))
		ctx.Export("gcpProject", pulumi.String(ctx.Project()))

		return nil
	}
}

// createOrSelectStack creates or selects a Pulumi stack
func (p *GCPProvisioner) createOrSelectStack(ctx context.Context, stackName string, program pulumi.RunFunc) (auto.Stack, error) {
	log.Info().
		Str("stackName", stackName).
		Str("backend", p.backendURL).
		Msg("Creating or selecting Pulumi stack")

	stack, err := auto.UpsertStackInlineSource(ctx, stackName, p.projectName, program,
		auto.Project(workspace.Project{
			Name: tokens.PackageName(p.projectName),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			Backend: &workspace.ProjectBackend{
				URL: p.backendURL,
			},
		}),
	)

	if err != nil {
		return auto.Stack{}, fmt.Errorf("failed to upsert stack: %w", err)
	}

	return stack, nil
}

// setStackConfig sets Pulumi stack configuration
func (p *GCPProvisioner) setStackConfig(ctx context.Context, stack auto.Stack, req *ProvisionRequestInternal) error {
	log.Info().Msg("Setting stack configuration")

	// Set GCP project
	if err := stack.SetConfig(ctx, "gcp:project", auto.ConfigValue{Value: p.gcpProject}); err != nil {
		return fmt.Errorf("failed to set gcp:project: %w", err)
	}

	// Set GCP region
	if err := stack.SetConfig(ctx, "gcp:region", auto.ConfigValue{Value: req.Region}); err != nil {
		return fmt.Errorf("failed to set gcp:region: %w", err)
	}

	return nil
}

// extractOutputs extracts outputs from Pulumi up result
func (p *GCPProvisioner) extractOutputs(upResult auto.UpResult, stackName, infraID string) (*provisioner.ProvisionResult, error) {
	log.Info().Msg("Extracting Pulumi outputs")

	getString := func(key string) string {
		if val, ok := upResult.Outputs[key].Value.(string); ok {
			return val
		}
		return ""
	}

	return &provisioner.ProvisionResult{
		InfrastructureID: infraID,
		StackName:        stackName,
		CloudProvider:    "gcp",
		GCPProject:       getString("gcpProject"),
		ClusterName:      getString("clusterName"),
		ClusterEndpoint:  getString("clusterEndpoint"),
		ClusterCACert:    getString("clusterCACert"),
		ClusterLocation:  getString("clusterLocation"),
		VPCName:          getString("vpcName"),
		VPCNetwork:       getString("vpcNetwork"),
		SubnetName:       getString("subnetName"),
		SubnetCIDR:       getString("subnetCIDR"),
		RouterName:       getString("routerName"),
		NATName:          getString("natName"),
		ServiceAccount:   getString("serviceAccount"),
		Namespace:        getString("namespace"),
		ProvisionLog:     upResult.StdOut,
	}, nil
}

// createProgressWriter creates an io.Writer for Pulumi progress updates
func (p *GCPProvisioner) createProgressWriter(ctx context.Context, infraID string) io.Writer {
	return &progressWriter{
		ctx:     ctx,
		tracker: p.tracker,
		infraID: infraID,
	}
}

// progressWriter implements io.Writer to capture Pulumi progress
type progressWriter struct {
	ctx     context.Context
	tracker *provisioner.Tracker
	infraID string
}

func (w *progressWriter) Write(p []byte) (n int, err error) {
	logEntry := string(p)

	// Log to zerolog
	log.Debug().Str("infraID", w.infraID).Msg(strings.TrimSpace(logEntry))

	// Update tracker with progress
	if err := w.tracker.UpdateProgress(w.ctx, w.infraID, logEntry); err != nil {
		log.Warn().Err(err).Msg("Failed to update provision progress")
	}

	return len(p), nil
}

// convertRequest converts external request to internal format
func (p *GCPProvisioner) convertRequest(req *provisioner.ProvisionRequest) *ProvisionRequestInternal {
	internalReq := &ProvisionRequestInternal{
		DeploymentID: req.DeploymentID,
		AppName:      req.AppName,
		Version:      req.Version,
		Cloud:        req.Cloud,
		Region:       req.Region,
		GCPProject:   p.gcpProject, // Use the GCP project from provisioner config
	}

	// Convert config
	if req.Config != nil {
		internalReq.Config = &ProvisionConfigInternal{
			NodeCount:       req.Config.NodeCount,
			MachineType:     req.Config.MachineType,
			Preemptible:     req.Config.Preemptible,
			DiskSize:        req.Config.DiskSize,
			DiskType:        req.Config.DiskType,
			Labels:          req.Config.Labels,
			VPCCIDRBlock:    req.Config.VPCCIDRBlock,
			SubnetCIDRBlock: req.Config.SubnetCIDRBlock,
		}
	} else {
		// Use defaults
		internalReq.Config = &ProvisionConfigInternal{
			NodeCount:   p.defaultNodes,
			MachineType: p.defaultType,
		}
	}

	// Use region from request or default
	if internalReq.Region == "" {
		internalReq.Region = p.gcpRegion
	}

	return internalReq
}
