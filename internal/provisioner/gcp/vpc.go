package gcp

import (
	"fmt"

	"github.com/pulumi/pulumi-gcp/sdk/v7/go/gcp/compute"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// VPCResources holds references to created VPC resources
type VPCResources struct {
	VPC    *compute.Network
	Subnet *compute.Subnetwork
	Router *compute.Router
	NAT    *compute.RouterNat
}

// createVPC creates a VPC network with custom subnets
func createVPC(ctx *pulumi.Context, req *ProvisionRequestInternal) (*compute.Network, error) {
	vpcName := generateVPCName(req.AppName, req.DeploymentID)

	vpc, err := compute.NewNetwork(ctx, vpcName, &compute.NetworkArgs{
		Name:                  pulumi.String(vpcName),
		AutoCreateSubnetworks: pulumi.Bool(false), // We'll create custom subnets
		Description:           pulumi.Sprintf("VPC for %s deployment", req.AppName),
		RoutingMode:           pulumi.String("REGIONAL"), // Regional routing for cost optimization
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create VPC: %w", err)
	}

	return vpc, nil
}

// createSubnet creates a subnet within the VPC
func createSubnet(ctx *pulumi.Context, vpc *compute.Network, req *ProvisionRequestInternal) (*compute.Subnetwork, error) {
	subnetName := generateSubnetName(req.AppName, req.DeploymentID)

	// Default CIDR block, can be overridden by config
	cidrBlock := "10.0.0.0/24"
	if req.Config != nil && req.Config.SubnetCIDRBlock != "" {
		cidrBlock = req.Config.SubnetCIDRBlock
	}

	subnet, err := compute.NewSubnetwork(ctx, subnetName, &compute.SubnetworkArgs{
		Name:                  pulumi.String(subnetName),
		Network:               vpc.ID(),
		IpCidrRange:           pulumi.String(cidrBlock),
		Region:                pulumi.String(req.Region),
		PrivateIpGoogleAccess: pulumi.Bool(true), // Enable private Google access for GKE nodes
		Description:           pulumi.Sprintf("Subnet for %s deployment", req.AppName),

		// Secondary IP ranges for GKE pods and services
		SecondaryIpRanges: compute.SubnetworkSecondaryIpRangeArray{
			&compute.SubnetworkSecondaryIpRangeArgs{
				RangeName:   pulumi.String("pods"),
				IpCidrRange: pulumi.String("10.1.0.0/16"), // Large range for pods
			},
			&compute.SubnetworkSecondaryIpRangeArgs{
				RangeName:   pulumi.String("services"),
				IpCidrRange: pulumi.String("10.2.0.0/16"), // Range for services
			},
		},

		// Enable flow logs for monitoring (optional, can be disabled for cost)
		LogConfig: &compute.SubnetworkLogConfigArgs{
			AggregationInterval: pulumi.String("INTERVAL_5_SEC"),
			FlowSampling:        pulumi.Float64(0.5),
			Metadata:            pulumi.String("INCLUDE_ALL_METADATA"),
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create subnet: %w", err)
	}

	return subnet, nil
}

// createCloudRouter creates a Cloud Router for NAT
func createCloudRouter(ctx *pulumi.Context, vpc *compute.Network, req *ProvisionRequestInternal) (*compute.Router, error) {
	routerName := generateRouterName(req.AppName, req.DeploymentID)

	router, err := compute.NewRouter(ctx, routerName, &compute.RouterArgs{
		Name:        pulumi.String(routerName),
		Network:     vpc.ID(),
		Region:      pulumi.String(req.Region),
		Description: pulumi.Sprintf("Cloud Router for %s deployment", req.AppName),

		// BGP configuration (optional, for advanced routing)
		Bgp: &compute.RouterBgpArgs{
			Asn: pulumi.Int(64512), // Private ASN
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create cloud router: %w", err)
	}

	return router, nil
}

// createCloudNAT creates a Cloud NAT for private GKE nodes to access internet
func createCloudNAT(ctx *pulumi.Context, router *compute.Router, req *ProvisionRequestInternal) (*compute.RouterNat, error) {
	natName := generateNATName(req.AppName, req.DeploymentID)

	nat, err := compute.NewRouterNat(ctx, natName, &compute.RouterNatArgs{
		Name:   pulumi.String(natName),
		Router: router.Name,
		Region: router.Region,

		// Auto-allocate NAT IPs
		NatIpAllocateOption: pulumi.String("AUTO_ONLY"),

		// NAT all subnetworks automatically
		SourceSubnetworkIpRangesToNat: pulumi.String("ALL_SUBNETWORKS_ALL_IP_RANGES"),

		// Logging configuration
		LogConfig: &compute.RouterNatLogConfigArgs{
			Enable: pulumi.Bool(true),
			Filter: pulumi.String("ERRORS_ONLY"), // Only log errors to reduce costs
		},

		// Min ports per VM (default is 64, increase for high-traffic apps)
		MinPortsPerVm: pulumi.Int(64),

		// Enable endpoint independent mapping for better NAT performance
		EnableEndpointIndependentMapping: pulumi.Bool(true),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create cloud NAT: %w", err)
	}

	return nat, nil
}

// createVPCResources creates all VPC-related resources
func createVPCResources(ctx *pulumi.Context, req *ProvisionRequestInternal) (*VPCResources, error) {
	// Create VPC
	vpc, err := createVPC(ctx, req)
	if err != nil {
		return nil, err
	}

	// Create subnet
	subnet, err := createSubnet(ctx, vpc, req)
	if err != nil {
		return nil, err
	}

	// Create Cloud Router
	router, err := createCloudRouter(ctx, vpc, req)
	if err != nil {
		return nil, err
	}

	// Create Cloud NAT
	nat, err := createCloudNAT(ctx, router, req)
	if err != nil {
		return nil, err
	}

	return &VPCResources{
		VPC:    vpc,
		Subnet: subnet,
		Router: router,
		NAT:    nat,
	}, nil
}

// ProvisionRequestInternal is an internal version of ProvisionRequest
// Used to avoid import cycles
type ProvisionRequestInternal struct {
	DeploymentID string
	AppName      string
	Version      string
	Cloud        string
	Region       string
	Config       *ProvisionConfigInternal
}

// ProvisionConfigInternal is an internal version of ProvisionConfig
type ProvisionConfigInternal struct {
	NodeCount       int
	MachineType     string
	Preemptible     bool
	DiskSize        int
	DiskType        string
	Labels          map[string]string
	VPCCIDRBlock    string
	SubnetCIDRBlock string
}
