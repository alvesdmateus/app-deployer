package gcp

import (
	"fmt"
	"regexp"
	"strings"
)

// Resource naming follows pattern: deployer-{type}-{app}-{deployment-id-short}
// Example: deployer-cluster-myapp-a3f9b2c1
//
// GCP naming requirements:
// - Lowercase letters, numbers, and hyphens only
// - Must start with a letter
// - Must end with a letter or number
// - Max length varies by resource type (we use 63 chars as safe default)

var (
	// Regex to match valid characters (lowercase letters, numbers, hyphens)
	validCharsRegex = regexp.MustCompile(`[^a-z0-9-]`)

	// Regex to match multiple consecutive hyphens
	multiHyphenRegex = regexp.MustCompile(`-+`)
)

// generateStackName generates a Pulumi stack name from deployment ID
// Format: deployer-{deployment-id}
func generateStackName(deploymentID string) string {
	return fmt.Sprintf("deployer-%s", deploymentID)
}

// generateClusterName generates a GKE cluster name
// Format: deployer-cluster-{app}-{id-short}
func generateClusterName(appName, deploymentID string) string {
	shortID := getShortID(deploymentID)
	sanitized := sanitizeName(appName)
	return fmt.Sprintf("deployer-cluster-%s-%s", sanitized, shortID)
}

// generateVPCName generates a VPC network name
// Format: deployer-vpc-{app}-{id-short}
func generateVPCName(appName, deploymentID string) string {
	shortID := getShortID(deploymentID)
	sanitized := sanitizeName(appName)
	return fmt.Sprintf("deployer-vpc-%s-%s", sanitized, shortID)
}

// generateSubnetName generates a subnet name
// Format: deployer-subnet-{app}-{id-short}
func generateSubnetName(appName, deploymentID string) string {
	shortID := getShortID(deploymentID)
	sanitized := sanitizeName(appName)
	return fmt.Sprintf("deployer-subnet-%s-%s", sanitized, shortID)
}

// generateRouterName generates a Cloud Router name
// Format: deployer-router-{app}-{id-short}
func generateRouterName(appName, deploymentID string) string {
	shortID := getShortID(deploymentID)
	sanitized := sanitizeName(appName)
	return fmt.Sprintf("deployer-router-%s-%s", sanitized, shortID)
}

// generateNATName generates a Cloud NAT name
// Format: deployer-nat-{app}-{id-short}
func generateNATName(appName, deploymentID string) string {
	shortID := getShortID(deploymentID)
	sanitized := sanitizeName(appName)
	return fmt.Sprintf("deployer-nat-%s-%s", sanitized, shortID)
}

// generateNodePoolName generates a GKE node pool name
// Format: deployer-pool-{app}-{id-short}
func generateNodePoolName(appName, deploymentID string) string {
	shortID := getShortID(deploymentID)
	sanitized := sanitizeName(appName)
	return fmt.Sprintf("deployer-pool-%s-%s", sanitized, shortID)
}

// generateServiceAccountName generates a service account name
// Format: deployer-sa-{app}-{id-short}
// Note: SA names have stricter limits (6-30 chars)
func generateServiceAccountName(appName, deploymentID string) string {
	shortID := getShortID(deploymentID)
	sanitized := sanitizeName(appName)

	// Service account names must be 6-30 characters
	name := fmt.Sprintf("deployer-sa-%s-%s", sanitized, shortID)

	// Truncate if too long (max 30 chars)
	if len(name) > 30 {
		// Keep the short ID for uniqueness
		maxAppLen := 30 - len("deployer-sa-") - len(shortID) - 1 // -1 for hyphen
		if maxAppLen > 0 {
			name = fmt.Sprintf("deployer-sa-%s-%s", sanitized[:maxAppLen], shortID)
		} else {
			// Fallback: just use deployer-sa-{id}
			name = fmt.Sprintf("deployer-sa-%s", shortID)
		}
	}

	return name
}

// generateFirewallRuleName generates a firewall rule name
// Format: deployer-fw-{purpose}-{app}-{id-short}
func generateFirewallRuleName(purpose, appName, deploymentID string) string {
	shortID := getShortID(deploymentID)
	sanitized := sanitizeName(appName)
	sanitizedPurpose := sanitizeName(purpose)
	return fmt.Sprintf("deployer-fw-%s-%s-%s", sanitizedPurpose, sanitized, shortID)
}

// generateNamespace generates a Kubernetes namespace name
// Format: deployer-{app}-{id-short}
func generateNamespace(appName, deploymentID string) string {
	shortID := getShortID(deploymentID)
	sanitized := sanitizeName(appName)
	return fmt.Sprintf("deployer-%s-%s", sanitized, shortID)
}

// sanitizeName converts a string to GCP-compliant naming
// Rules:
// - Lowercase only
// - Replace underscores and spaces with hyphens
// - Remove invalid characters
// - Ensure starts with letter
// - Ensure ends with letter or number
// - Truncate to max 20 chars (safe for most resources when combined with prefixes)
func sanitizeName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace underscores and spaces with hyphens
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")

	// Remove invalid characters (keep only a-z, 0-9, -)
	name = validCharsRegex.ReplaceAllString(name, "")

	// Replace multiple consecutive hyphens with a single hyphen
	name = multiHyphenRegex.ReplaceAllString(name, "-")

	// Trim hyphens from start and end
	name = strings.Trim(name, "-")

	// Ensure starts with a letter
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "a-" + name
	}

	// If empty after sanitization, use default
	if name == "" {
		name = "app"
	}

	// Truncate to 20 characters (safe for most GCP resources)
	if len(name) > 20 {
		name = name[:20]
	}

	// Ensure ends with letter or number (trim trailing hyphens)
	name = strings.TrimRight(name, "-")

	return name
}

// getShortID returns the first 8 characters of a UUID/deployment ID
func getShortID(deploymentID string) string {
	// Remove hyphens from UUID and take first 8 chars
	cleaned := strings.ReplaceAll(deploymentID, "-", "")
	if len(cleaned) >= 8 {
		return cleaned[:8]
	}
	return cleaned
}

// generateLabels creates standard labels for all resources
func generateLabels(appName, deploymentID, environment string) map[string]string {
	return map[string]string{
		"app":           sanitizeName(appName),
		"deployment-id": getShortID(deploymentID),
		"managed-by":    "app-deployer",
		"environment":   environment,
		"cost-tracking": "enabled",
	}
}
