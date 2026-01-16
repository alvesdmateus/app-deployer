package gcp

import (
	"fmt"

	"github.com/pulumi/pulumi-gcp/sdk/v7/go/gcp/container"
	"github.com/pulumi/pulumi-gcp/sdk/v7/go/gcp/projects"
	"github.com/pulumi/pulumi-gcp/sdk/v7/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// GKEResources holds references to created GKE resources
type GKEResources struct {
	Cluster        *container.Cluster
	NodePool       *container.NodePool
	ServiceAccount *serviceaccount.Account
}

// createServiceAccount creates a service account for GKE nodes
func createServiceAccount(ctx *pulumi.Context, req *ProvisionRequestInternal) (*serviceaccount.Account, error) {
	saName := generateServiceAccountName(req.AppName, req.DeploymentID)

	sa, err := serviceaccount.NewAccount(ctx, saName, &serviceaccount.AccountArgs{
		AccountId:   pulumi.String(saName),
		DisplayName: pulumi.Sprintf("GKE service account for %s", req.AppName),
		Description: pulumi.String("Service account used by GKE nodes for cluster operations"),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create service account: %w", err)
	}

	return sa, nil
}

// bindServiceAccountRoles binds necessary IAM roles to the GKE service account
func bindServiceAccountRoles(ctx *pulumi.Context, sa *serviceaccount.Account, req *ProvisionRequestInternal) error {
	// Define the roles needed for GKE nodes to operate properly
	roles := []struct {
		role        string
		description string
	}{
		{"roles/logging.logWriter", "Write logs to Cloud Logging"},
		{"roles/monitoring.metricWriter", "Write metrics to Cloud Monitoring"},
		{"roles/monitoring.viewer", "View monitoring data"},
		{"roles/storage.objectViewer", "Pull images from GCS/GCR"},
		{"roles/artifactregistry.reader", "Pull images from Artifact Registry"},
	}

	for _, r := range roles {
		bindingName := fmt.Sprintf("sa-binding-%s-%s", sanitizeName(r.role), getShortID(req.DeploymentID))

		_, err := projects.NewIAMMember(ctx, bindingName, &projects.IAMMemberArgs{
			Project: pulumi.String(req.GCPProject), // Use GCP project ID, not Pulumi project name
			Role:    pulumi.String(r.role),
			Member:  pulumi.Sprintf("serviceAccount:%s", sa.Email),
		})
		if err != nil {
			return fmt.Errorf("failed to bind role %s: %w", r.role, err)
		}
	}

	return nil
}

// createGKECluster creates a GKE cluster
func createGKECluster(ctx *pulumi.Context, vpcResources *VPCResources, sa *serviceaccount.Account, req *ProvisionRequestInternal) (*container.Cluster, error) {
	clusterName := generateClusterName(req.AppName, req.DeploymentID)

	// Generate labels
	labels := generateLabels(req.AppName, req.DeploymentID, "production")
	if req.Config != nil && req.Config.Labels != nil {
		for k, v := range req.Config.Labels {
			labels[k] = v
		}
	}

	// Convert to Pulumi StringMap
	pulumiLabels := make(pulumi.StringMap)
	for k, v := range labels {
		pulumiLabels[k] = pulumi.String(v)
	}

	// Use a specific zone for zonal cluster (reduces SSD quota requirement from 300GB to 100GB)
	// Regional clusters need 3x the resources for HA control plane
	clusterLocation := req.Region + "-a" // e.g., us-central1-a

	// Create the GKE cluster
	cluster, err := container.NewCluster(ctx, clusterName, &container.ClusterArgs{
		Name:     pulumi.String(clusterName),
		Location: pulumi.String(clusterLocation),

		// Network configuration
		Network:    vpcResources.VPC.SelfLink,
		Subnetwork: vpcResources.Subnet.SelfLink,

		// Remove default node pool (we'll create a custom one)
		RemoveDefaultNodePool: pulumi.Bool(true),
		InitialNodeCount:      pulumi.Int(1),

		// Release channel for automatic updates
		ReleaseChannel: &container.ClusterReleaseChannelArgs{
			Channel: pulumi.String("REGULAR"), // Options: RAPID, REGULAR, STABLE
		},

		// IP allocation policy for VPC-native cluster
		IpAllocationPolicy: &container.ClusterIpAllocationPolicyArgs{
			ClusterSecondaryRangeName:  pulumi.String("pods"),
			ServicesSecondaryRangeName: pulumi.String("services"),
		},

		// Network policy configuration
		NetworkPolicy: &container.ClusterNetworkPolicyArgs{
			Enabled: pulumi.Bool(false), // Can enable for stricter security
		},

		// Addons configuration
		AddonsConfig: &container.ClusterAddonsConfigArgs{
			HttpLoadBalancing: &container.ClusterAddonsConfigHttpLoadBalancingArgs{
				Disabled: pulumi.Bool(false), // Enable HTTP(S) load balancing
			},
			HorizontalPodAutoscaling: &container.ClusterAddonsConfigHorizontalPodAutoscalingArgs{
				Disabled: pulumi.Bool(false), // Enable HPA
			},
			NetworkPolicyConfig: &container.ClusterAddonsConfigNetworkPolicyConfigArgs{
				Disabled: pulumi.Bool(true), // Disable for now
			},
		},

		// Master authorized networks (optional - for production, restrict access)
		MasterAuthorizedNetworksConfig: &container.ClusterMasterAuthorizedNetworksConfigArgs{
			// Leave empty to allow access from anywhere (can be restricted later)
		},

		// Private cluster configuration (optional - for better security)
		PrivateClusterConfig: &container.ClusterPrivateClusterConfigArgs{
			EnablePrivateNodes:    pulumi.Bool(true),  // Nodes have private IPs only
			EnablePrivateEndpoint: pulumi.Bool(false), // Master endpoint is public (change to true for full private)
			MasterIpv4CidrBlock:   pulumi.String("172.16.0.0/28"),
		},

		// Workload identity for pod-level IAM (recommended)
		WorkloadIdentityConfig: &container.ClusterWorkloadIdentityConfigArgs{
			WorkloadPool: pulumi.Sprintf("%s.svc.id.goog", req.GCPProject),
		},

		// Maintenance window
		MaintenancePolicy: &container.ClusterMaintenancePolicyArgs{
			DailyMaintenanceWindow: &container.ClusterMaintenancePolicyDailyMaintenanceWindowArgs{
				StartTime: pulumi.String("03:00"), // 3 AM UTC
			},
		},

		// Resource labels
		ResourceLabels: pulumiLabels,

		// Logging and monitoring configuration
		LoggingService:    pulumi.String("logging.googleapis.com/kubernetes"),
		MonitoringService: pulumi.String("monitoring.googleapis.com/kubernetes"),

		// Binary authorization (optional - for image security)
		BinaryAuthorization: &container.ClusterBinaryAuthorizationArgs{
			EvaluationMode: pulumi.String("DISABLED"), // Can enable for production
		},

		// Enable shielded nodes for enhanced security
		EnableShieldedNodes: pulumi.Bool(true),

		// Enable autopilot mode (optional - fully managed GKE)
		// EnableAutopilot: pulumi.Bool(false),

		// Datapath provider (use advanced networking features)
		DatapathProvider: pulumi.String("ADVANCED_DATAPATH"), // or "LEGACY_DATAPATH"
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create GKE cluster: %w", err)
	}

	return cluster, nil
}

// createNodePool creates a node pool for the GKE cluster
func createNodePool(ctx *pulumi.Context, cluster *container.Cluster, sa *serviceaccount.Account, req *ProvisionRequestInternal) (*container.NodePool, error) {
	nodePoolName := generateNodePoolName(req.AppName, req.DeploymentID)

	// Get configuration with defaults
	nodeCount := 2
	machineType := "e2-small"
	preemptible := false
	diskSize := 30 // GB - reduced to stay within quota
	diskType := "pd-standard"

	if req.Config != nil {
		if req.Config.NodeCount > 0 {
			nodeCount = req.Config.NodeCount
		}
		if req.Config.MachineType != "" {
			machineType = req.Config.MachineType
		}
		preemptible = req.Config.Preemptible
		if req.Config.DiskSize > 0 {
			diskSize = req.Config.DiskSize
		}
		if req.Config.DiskType != "" {
			diskType = req.Config.DiskType
		}
	}

	// Generate labels
	labels := generateLabels(req.AppName, req.DeploymentID, "production")
	if req.Config != nil && req.Config.Labels != nil {
		for k, v := range req.Config.Labels {
			labels[k] = v
		}
	}

	// Convert to Pulumi StringMap
	pulumiLabels := make(pulumi.StringMap)
	for k, v := range labels {
		pulumiLabels[k] = pulumi.String(v)
	}

	nodePool, err := container.NewNodePool(ctx, nodePoolName, &container.NodePoolArgs{
		Name:     pulumi.String(nodePoolName),
		Cluster:  cluster.Name,
		Location: cluster.Location,

		// Initial node count
		InitialNodeCount: pulumi.Int(nodeCount),

		// Autoscaling configuration (optional)
		// Autoscaling: &container.NodePoolAutoscalingArgs{
		// 	MinNodeCount: pulumi.Int(1),
		// 	MaxNodeCount: pulumi.Int(5),
		// },

		// Node configuration
		NodeConfig: &container.NodePoolNodeConfigArgs{
			MachineType: pulumi.String(machineType),
			DiskSizeGb:  pulumi.Int(diskSize),
			DiskType:    pulumi.String(diskType),

			// Service account
			ServiceAccount: sa.Email,

			// OAuth scopes for node permissions
			OauthScopes: pulumi.StringArray{
				pulumi.String("https://www.googleapis.com/auth/cloud-platform"),
			},

			// Labels
			Labels: pulumiLabels,

			// Metadata
			Metadata: pulumi.StringMap{
				"disable-legacy-endpoints": pulumi.String("true"),
			},

			// Preemptible nodes (for cost savings)
			Preemptible: pulumi.Bool(preemptible),

			// Spot VMs (alternative to preemptible, newer)
			// Spot: pulumi.Bool(preemptible),

			// Shielded instance configuration
			ShieldedInstanceConfig: &container.NodePoolNodeConfigShieldedInstanceConfigArgs{
				EnableSecureBoot:          pulumi.Bool(true),
				EnableIntegrityMonitoring: pulumi.Bool(true),
			},

			// Workload metadata configuration
			WorkloadMetadataConfig: &container.NodePoolNodeConfigWorkloadMetadataConfigArgs{
				Mode: pulumi.String("GKE_METADATA"), // Use workload identity
			},

			// Taints (optional - for node specialization)
			// Taints: container.NodePoolNodeConfigTaintArray{
			// 	&container.NodePoolNodeConfigTaintArgs{
			// 		Key:    pulumi.String("workload"),
			// 		Value:  pulumi.String("general"),
			// 		Effect: pulumi.String("NoSchedule"),
			// 	},
			// },
		},

		// Management configuration
		Management: &container.NodePoolManagementArgs{
			AutoRepair:  pulumi.Bool(true), // Automatically repair unhealthy nodes
			AutoUpgrade: pulumi.Bool(true), // Automatically upgrade nodes
		},

		// Upgrade settings
		UpgradeSettings: &container.NodePoolUpgradeSettingsArgs{
			MaxSurge:       pulumi.Int(1), // Max additional nodes during upgrade
			MaxUnavailable: pulumi.Int(0), // Max unavailable nodes during upgrade
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create node pool: %w", err)
	}

	return nodePool, nil
}

// createGKEResources creates all GKE-related resources
func createGKEResources(ctx *pulumi.Context, vpcResources *VPCResources, req *ProvisionRequestInternal) (*GKEResources, error) {
	// Create service account
	sa, err := createServiceAccount(ctx, req)
	if err != nil {
		return nil, err
	}

	// Bind IAM roles to service account for GKE node operations
	if err := bindServiceAccountRoles(ctx, sa, req); err != nil {
		return nil, fmt.Errorf("failed to bind service account roles: %w", err)
	}

	// Create GKE cluster
	cluster, err := createGKECluster(ctx, vpcResources, sa, req)
	if err != nil {
		return nil, err
	}

	// Create node pool
	nodePool, err := createNodePool(ctx, cluster, sa, req)
	if err != nil {
		return nil, err
	}

	return &GKEResources{
		Cluster:        cluster,
		NodePool:       nodePool,
		ServiceAccount: sa,
	}, nil
}
