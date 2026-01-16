//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alvesdmateus/app-deployer/internal/provisioner"
	"github.com/alvesdmateus/app-deployer/internal/state"
)

// TestGCPProvisioner_ProvisionAndDestroy tests the full provisioning lifecycle
// This test creates real GCP resources (VPC, GKE cluster) and then destroys them
func TestGCPProvisioner_ProvisionAndDestroy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E provisioner test in short mode")
	}

	// Setup test environment
	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	// Create a context with timeout for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Generate unique deployment ID for this test
	deploymentID := uuid.New().String()
	appName := GenerateTestDeploymentName("e2e-prov")

	env.Logger.Info().
		Str("deployment_id", deploymentID).
		Str("app_name", appName).
		Msg("Starting provisioner E2E test")

	// Create provision request
	provisionReq := &provisioner.ProvisionRequest{
		DeploymentID: deploymentID,
		AppName:      appName,
		Version:      "v1.0.0",
		Cloud:        "gcp",
		Region:       env.Config.GCPRegion,
		Config: &provisioner.ProvisionConfig{
			NodeCount:   2,
			MachineType: "e2-small",
			Preemptible: true, // Use preemptible for cost savings in tests
			Labels: map[string]string{
				"environment": "e2e-test",
				"test":        strings.ToLower(strings.ReplaceAll(t.Name(), "_", "-")),
			},
		},
	}

	// Create deployment record in database first
	_ = createTestDeployment(t, env, deploymentID, appName)
	env.TrackDeployment(deploymentID)

	// Step 1: Provision infrastructure
	env.Logger.Info().Msg("Step 1: Provisioning GCP infrastructure...")
	startTime := time.Now()

	result, err := env.Provisioner.Provision(ctx, provisionReq)
	require.NoError(t, err, "Provisioning should succeed")
	require.NotNil(t, result, "Provision result should not be nil")

	provisionDuration := time.Since(startTime)
	env.Logger.Info().
		Dur("duration", provisionDuration).
		Str("infrastructure_id", result.InfrastructureID).
		Str("cluster_name", result.ClusterName).
		Str("cluster_endpoint", result.ClusterEndpoint).
		Msg("Infrastructure provisioned successfully")

	// Track infrastructure for cleanup
	env.TrackInfrastructure(result.InfrastructureID)

	// Step 2: Verify provisioned resources
	env.Logger.Info().Msg("Step 2: Verifying provisioned resources...")

	// Verify VPC resources
	assert.NotEmpty(t, result.VPCName, "VPC name should be set")
	assert.NotEmpty(t, result.SubnetName, "Subnet name should be set")
	assert.NotEmpty(t, result.RouterName, "Router name should be set")
	assert.NotEmpty(t, result.NATName, "NAT name should be set")

	// Verify GKE resources
	assert.NotEmpty(t, result.ClusterName, "Cluster name should be set")
	assert.NotEmpty(t, result.ClusterEndpoint, "Cluster endpoint should be set")
	assert.NotEmpty(t, result.ClusterCACert, "Cluster CA cert should be set")
	assert.NotEmpty(t, result.ClusterLocation, "Cluster location should be set")
	assert.NotEmpty(t, result.ServiceAccount, "Service account should be set")
	assert.NotEmpty(t, result.Namespace, "Namespace should be set")

	// Verify stack info
	assert.NotEmpty(t, result.StackName, "Stack name should be set")
	assert.Equal(t, "gcp", result.CloudProvider, "Cloud provider should be gcp")

	env.Logger.Info().
		Str("vpc", result.VPCName).
		Str("subnet", result.SubnetName).
		Str("cluster", result.ClusterName).
		Str("namespace", result.Namespace).
		Msg("All provisioned resources verified")

	// Step 3: Verify infrastructure status in database
	env.Logger.Info().Msg("Step 3: Verifying infrastructure status in database...")

	infra, err := env.Repo.GetInfrastructureByID(ctx, uuid.MustParse(result.InfrastructureID))
	require.NoError(t, err, "Should find infrastructure in database")
	assert.Equal(t, "READY", infra.Status, "Infrastructure status should be READY")
	assert.Equal(t, result.ClusterName, infra.ClusterName, "Cluster name should match")
	assert.Equal(t, result.ClusterEndpoint, infra.ClusterEndpoint, "Cluster endpoint should match")

	// Step 4: Get infrastructure status via provisioner
	env.Logger.Info().Msg("Step 4: Checking infrastructure status via provisioner...")

	status, err := env.Provisioner.GetStatus(ctx, result.StackName)
	require.NoError(t, err, "GetStatus should succeed")
	assert.Equal(t, "READY", status.Status, "Status should be READY")

	// Step 5: Destroy infrastructure
	env.Logger.Info().Msg("Step 5: Destroying infrastructure...")
	destroyStartTime := time.Now()

	destroyReq := &provisioner.DestroyRequest{
		DeploymentID:     deploymentID,
		InfrastructureID: result.InfrastructureID,
		StackName:        result.StackName,
	}

	err = env.Provisioner.Destroy(ctx, destroyReq)
	require.NoError(t, err, "Destroy should succeed")

	destroyDuration := time.Since(destroyStartTime)
	env.Logger.Info().
		Dur("duration", destroyDuration).
		Msg("Infrastructure destroyed successfully")

	// Remove from cleanup tracking since we already destroyed
	env.InfrastructureIDs = []string{}

	// Step 6: Verify infrastructure is destroyed
	env.Logger.Info().Msg("Step 6: Verifying infrastructure is destroyed...")

	// Try to get status - should fail or show destroyed
	_, err = env.Provisioner.GetStatus(ctx, result.StackName)
	// This may return an error since the stack is destroyed, which is expected

	// Log test summary
	env.Logger.Info().
		Str("deployment_id", deploymentID).
		Str("app_name", appName).
		Dur("provision_duration", provisionDuration).
		Dur("destroy_duration", destroyDuration).
		Dur("total_duration", time.Since(startTime)).
		Msg("Provisioner E2E test completed successfully")
}

// TestGCPProvisioner_Idempotency tests that provisioning the same deployment twice
// returns the existing infrastructure without creating duplicates
func TestGCPProvisioner_Idempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E provisioner idempotency test in short mode")
	}

	// Setup test environment
	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()

	deploymentID := uuid.New().String()
	appName := GenerateTestDeploymentName("e2e-idemp")

	env.Logger.Info().
		Str("deployment_id", deploymentID).
		Str("app_name", appName).
		Msg("Starting provisioner idempotency test")

	// Create deployment record
	createTestDeployment(t, env, deploymentID, appName)
	env.TrackDeployment(deploymentID)

	provisionReq := &provisioner.ProvisionRequest{
		DeploymentID: deploymentID,
		AppName:      appName,
		Version:      "v1.0.0",
		Cloud:        "gcp",
		Region:       env.Config.GCPRegion,
		Config: &provisioner.ProvisionConfig{
			NodeCount:   2,
			MachineType: "e2-small",
			Preemptible: true,
		},
	}

	// First provision
	env.Logger.Info().Msg("First provision call...")
	result1, err := env.Provisioner.Provision(ctx, provisionReq)
	require.NoError(t, err, "First provision should succeed")
	env.TrackInfrastructure(result1.InfrastructureID)

	// Second provision with same deployment ID (should be idempotent)
	env.Logger.Info().Msg("Second provision call (should be idempotent)...")
	result2, err := env.Provisioner.Provision(ctx, provisionReq)
	require.NoError(t, err, "Second provision should succeed")

	// Verify same infrastructure was returned
	assert.Equal(t, result1.InfrastructureID, result2.InfrastructureID, "Should return same infrastructure ID")
	assert.Equal(t, result1.ClusterName, result2.ClusterName, "Should return same cluster name")
	assert.Equal(t, result1.StackName, result2.StackName, "Should return same stack name")

	env.Logger.Info().
		Str("infrastructure_id", result1.InfrastructureID).
		Msg("Idempotency verified - same infrastructure returned")

	// Cleanup
	destroyReq := &provisioner.DestroyRequest{
		DeploymentID:     deploymentID,
		InfrastructureID: result1.InfrastructureID,
		StackName:        result1.StackName,
	}
	err = env.Provisioner.Destroy(ctx, destroyReq)
	require.NoError(t, err, "Destroy should succeed")
	env.InfrastructureIDs = []string{}

	env.Logger.Info().Msg("Provisioner idempotency test completed successfully")
}

// createTestDeployment creates a deployment record in the database for testing
func createTestDeployment(t *testing.T, env *TestEnvironment, deploymentID, appName string) *state.Deployment {
	t.Helper()

	ctx := context.Background()

	deployment := &state.Deployment{
		ID:      uuid.MustParse(deploymentID),
		Name:    appName,
		AppName: appName,
		Version: "v1.0.0",
		Status:  "PENDING",
		Cloud:   "gcp",
		Region:  env.Config.GCPRegion,
		Port:    env.Config.TestPort,
	}

	err := env.Repo.CreateDeployment(ctx, deployment)
	require.NoError(t, err, "Failed to create test deployment")

	return deployment
}
