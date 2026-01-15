package gcp

import (
	"fmt"

	"github.com/pulumi/pulumi-gcp/sdk/v7/go/gcp/compute"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// createFirewallRules creates minimal firewall rules for GKE cluster
func createFirewallRules(ctx *pulumi.Context, vpc *compute.Network, req *ProvisionRequestInternal) error {
	// Get labels for tagging
	labels := generateLabels(req.AppName, req.DeploymentID, "production")
	if req.Config != nil && req.Config.Labels != nil {
		for k, v := range req.Config.Labels {
			labels[k] = v
		}
	}

	// Convert labels to Pulumi StringMap
	pulumiLabels := make(pulumi.StringMap)
	for k, v := range labels {
		pulumiLabels[k] = pulumi.String(v)
	}

	// 1. Allow internal communication within VPC
	if err := createInternalFirewallRule(ctx, vpc, req, pulumiLabels); err != nil {
		return err
	}

	// 2. Allow GKE master to communicate with nodes
	if err := createGKEMasterFirewallRule(ctx, vpc, req, pulumiLabels); err != nil {
		return err
	}

	// 3. Allow health checks from Google Cloud
	if err := createHealthCheckFirewallRule(ctx, vpc, req, pulumiLabels); err != nil {
		return err
	}

	// 4. Allow SSH from IAP (Identity-Aware Proxy) for debugging
	if err := createSSHFirewallRule(ctx, vpc, req, pulumiLabels); err != nil {
		return err
	}

	return nil
}

// createInternalFirewallRule allows internal communication within the VPC
func createInternalFirewallRule(ctx *pulumi.Context, vpc *compute.Network, req *ProvisionRequestInternal, labels pulumi.StringMap) error {
	ruleName := generateFirewallRuleName("internal", req.AppName, req.DeploymentID)

	_, err := compute.NewFirewall(ctx, ruleName, &compute.FirewallArgs{
		Name:        pulumi.String(ruleName),
		Network:     vpc.SelfLink,
		Description: pulumi.String("Allow internal communication within VPC"),
		Direction:   pulumi.String("INGRESS"),
		Priority:    pulumi.Int(1000),

		// Allow from any source within the VPC
		SourceRanges: pulumi.StringArray{
			pulumi.String("10.0.0.0/8"), // Cover all internal subnets
		},

		// Allow all protocols for internal communication
		Allows: compute.FirewallAllowArray{
			&compute.FirewallAllowArgs{
				Protocol: pulumi.String("tcp"),
				Ports:    pulumi.StringArray{pulumi.String("0-65535")},
			},
			&compute.FirewallAllowArgs{
				Protocol: pulumi.String("udp"),
				Ports:    pulumi.StringArray{pulumi.String("0-65535")},
			},
			&compute.FirewallAllowArgs{
				Protocol: pulumi.String("icmp"),
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create internal firewall rule: %w", err)
	}

	return nil
}

// createGKEMasterFirewallRule allows GKE master to communicate with nodes
func createGKEMasterFirewallRule(ctx *pulumi.Context, vpc *compute.Network, req *ProvisionRequestInternal, labels pulumi.StringMap) error {
	ruleName := generateFirewallRuleName("gke-master", req.AppName, req.DeploymentID)

	_, err := compute.NewFirewall(ctx, ruleName, &compute.FirewallArgs{
		Name:        pulumi.String(ruleName),
		Network:     vpc.SelfLink,
		Description: pulumi.String("Allow GKE master to communicate with nodes"),
		Direction:   pulumi.String("INGRESS"),
		Priority:    pulumi.Int(1000),

		// GKE master IP ranges (these are Google-managed ranges)
		SourceRanges: pulumi.StringArray{
			pulumi.String("172.16.0.0/28"), // Default GKE master range
		},

		// Allow necessary ports for GKE
		Allows: compute.FirewallAllowArray{
			&compute.FirewallAllowArgs{
				Protocol: pulumi.String("tcp"),
				Ports: pulumi.StringArray{
					pulumi.String("443"),   // HTTPS
					pulumi.String("10250"), // Kubelet
					pulumi.String("8443"),  // Webhook
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create GKE master firewall rule: %w", err)
	}

	return nil
}

// createHealthCheckFirewallRule allows health checks from Google Cloud Load Balancers
func createHealthCheckFirewallRule(ctx *pulumi.Context, vpc *compute.Network, req *ProvisionRequestInternal, labels pulumi.StringMap) error {
	ruleName := generateFirewallRuleName("health-check", req.AppName, req.DeploymentID)

	_, err := compute.NewFirewall(ctx, ruleName, &compute.FirewallArgs{
		Name:        pulumi.String(ruleName),
		Network:     vpc.SelfLink,
		Description: pulumi.String("Allow health checks from Google Cloud Load Balancers"),
		Direction:   pulumi.String("INGRESS"),
		Priority:    pulumi.Int(1000),

		// Google Cloud health check IP ranges
		SourceRanges: pulumi.StringArray{
			pulumi.String("35.191.0.0/16"),  // Google Cloud health checks
			pulumi.String("130.211.0.0/22"), // Google Cloud health checks
		},

		// Allow health check traffic on all ports (will be restricted by service)
		Allows: compute.FirewallAllowArray{
			&compute.FirewallAllowArgs{
				Protocol: pulumi.String("tcp"),
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create health check firewall rule: %w", err)
	}

	return nil
}

// createSSHFirewallRule allows SSH access via Identity-Aware Proxy for debugging
func createSSHFirewallRule(ctx *pulumi.Context, vpc *compute.Network, req *ProvisionRequestInternal, labels pulumi.StringMap) error {
	ruleName := generateFirewallRuleName("ssh-iap", req.AppName, req.DeploymentID)

	_, err := compute.NewFirewall(ctx, ruleName, &compute.FirewallArgs{
		Name:        pulumi.String(ruleName),
		Network:     vpc.SelfLink,
		Description: pulumi.String("Allow SSH via Identity-Aware Proxy"),
		Direction:   pulumi.String("INGRESS"),
		Priority:    pulumi.Int(1000),

		// IAP IP range for SSH tunneling
		SourceRanges: pulumi.StringArray{
			pulumi.String("35.235.240.0/20"), // IAP for TCP forwarding
		},

		// Allow SSH
		Allows: compute.FirewallAllowArray{
			&compute.FirewallAllowArgs{
				Protocol: pulumi.String("tcp"),
				Ports: pulumi.StringArray{
					pulumi.String("22"), // SSH
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create SSH IAP firewall rule: %w", err)
	}

	return nil
}

// createEgressFirewallRule creates an egress rule (optional, for strict security)
func createEgressFirewallRule(ctx *pulumi.Context, vpc *compute.Network, req *ProvisionRequestInternal, labels pulumi.StringMap) error {
	ruleName := generateFirewallRuleName("egress", req.AppName, req.DeploymentID)

	_, err := compute.NewFirewall(ctx, ruleName, &compute.FirewallArgs{
		Name:        pulumi.String(ruleName),
		Network:     vpc.SelfLink,
		Description: pulumi.String("Allow egress traffic to internet"),
		Direction:   pulumi.String("EGRESS"),
		Priority:    pulumi.Int(1000),

		// Allow to any destination
		DestinationRanges: pulumi.StringArray{
			pulumi.String("0.0.0.0/0"),
		},

		// Allow all protocols for egress
		Allows: compute.FirewallAllowArray{
			&compute.FirewallAllowArgs{
				Protocol: pulumi.String("tcp"),
			},
			&compute.FirewallAllowArgs{
				Protocol: pulumi.String("udp"),
			},
			&compute.FirewallAllowArgs{
				Protocol: pulumi.String("icmp"),
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create egress firewall rule: %w", err)
	}

	return nil
}
