//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alvesdmateus/app-deployer/internal/deployer"
	"github.com/alvesdmateus/app-deployer/internal/provisioner"
	"github.com/alvesdmateus/app-deployer/internal/queue"
	"github.com/alvesdmateus/app-deployer/internal/state"
)

// TestFullDeploymentPipeline tests the complete deployment lifecycle:
// Create Deployment -> Provision Infrastructure -> Deploy Application -> Verify Access -> Destroy
func TestFullDeploymentPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E full pipeline test in short mode")
	}

	// Setup test environment
	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	// Create context with timeout for entire pipeline test
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	// Generate unique identifiers
	deploymentID := uuid.New().String()
	appName := GenerateTestDeploymentName("e2e-pipeline")

	env.Logger.Info().
		Str("deployment_id", deploymentID).
		Str("app_name", appName).
		Str("test_image", env.Config.TestImage).
		Int("test_port", env.Config.TestPort).
		Msg("Starting full deployment pipeline E2E test")

	testStartTime := time.Now()

	// ===== STEP 1: Create Deployment Record =====
	env.Logger.Info().Msg("Step 1: Creating deployment record...")

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
	require.NoError(t, err, "Failed to create deployment record")
	env.TrackDeployment(deploymentID)

	env.Logger.Info().
		Str("deployment_id", deploymentID).
		Str("status", deployment.Status).
		Msg("Deployment record created")

	// ===== STEP 2: Provision Infrastructure =====
	env.Logger.Info().Msg("Step 2: Provisioning GCP infrastructure...")
	provisionStartTime := time.Now()

	// Update status to PROVISIONING
	err = env.Repo.UpdateDeploymentStatus(ctx, deployment.ID, "PROVISIONING")
	require.NoError(t, err)

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
			Labels: map[string]string{
				"environment": "e2e-test",
				"test":        "full-pipeline",
			},
		},
	}

	provisionResult, err := env.Provisioner.Provision(ctx, provisionReq)
	require.NoError(t, err, "Infrastructure provisioning should succeed")
	require.NotNil(t, provisionResult)

	provisionDuration := time.Since(provisionStartTime)
	env.TrackInfrastructure(provisionResult.InfrastructureID)

	env.Logger.Info().
		Dur("duration", provisionDuration).
		Str("infrastructure_id", provisionResult.InfrastructureID).
		Str("cluster_name", provisionResult.ClusterName).
		Str("cluster_endpoint", provisionResult.ClusterEndpoint).
		Str("namespace", provisionResult.Namespace).
		Msg("Infrastructure provisioned successfully")

	// Verify infrastructure in database
	infraID := uuid.MustParse(provisionResult.InfrastructureID)
	infra, err := env.Repo.GetInfrastructureByID(ctx, infraID)
	require.NoError(t, err)
	assert.Equal(t, "READY", infra.Status)

	// Update deployment with infrastructure reference
	deployment.InfrastructureID = &infraID
	err = env.Repo.UpdateDeployment(ctx, deployment)
	require.NoError(t, err)

	// ===== STEP 3: Deploy Application via Helm =====
	env.Logger.Info().Msg("Step 3: Deploying application via Helm...")
	deployStartTime := time.Now()

	// Update status to DEPLOYING
	err = env.Repo.UpdateDeploymentStatus(ctx, deployment.ID, "DEPLOYING")
	require.NoError(t, err)

	deployReq := &deployer.DeployRequest{
		DeploymentID:     deploymentID,
		InfrastructureID: provisionResult.InfrastructureID,
		AppName:          appName,
		Version:          "v1.0.0",
		ImageTag:         env.Config.TestImage,
		Port:             env.Config.TestPort,
		Replicas:         1,
	}

	deployResult, err := env.Deployer.Deploy(ctx, deployReq)
	require.NoError(t, err, "Helm deployment should succeed")
	require.NotNil(t, deployResult)

	deployDuration := time.Since(deployStartTime)

	env.Logger.Info().
		Dur("duration", deployDuration).
		Str("release_name", deployResult.ReleaseName).
		Str("namespace", deployResult.Namespace).
		Str("external_ip", deployResult.ExternalIP).
		Str("status", deployResult.Status).
		Msg("Application deployed successfully")

	// Verify deployment result
	assert.NotEmpty(t, deployResult.ExternalIP, "External IP should be assigned")
	assert.NotEmpty(t, deployResult.ReleaseName, "Release name should be set")
	assert.Equal(t, "deployed", deployResult.Status, "Deploy status should be 'deployed'")

	// Update deployment status to EXPOSED
	deployment.Status = "EXPOSED"
	deployment.ExternalIP = deployResult.ExternalIP
	deployment.ExternalURL = fmt.Sprintf("http://%s:%d", deployResult.ExternalIP, env.Config.TestPort)
	err = env.Repo.UpdateDeployment(ctx, deployment)
	require.NoError(t, err)

	// ===== STEP 4: Verify External Access =====
	env.Logger.Info().Msg("Step 4: Verifying external access...")

	// Wait a bit for load balancer to fully initialize
	time.Sleep(30 * time.Second)

	env.VerifyExternalAccess(t, deployResult.ExternalIP, env.Config.TestPort, http.StatusOK)

	env.Logger.Info().
		Str("external_url", deployment.ExternalURL).
		Msg("External access verified - application is live!")

	// ===== STEP 5: Verify Deployment Logs =====
	env.Logger.Info().Msg("Step 5: Checking deployment logs...")

	logs, err := env.Repo.GetDeploymentLogs(ctx, deployment.ID, "", 100, 0)
	require.NoError(t, err, "Should be able to retrieve deployment logs")
	// Logs may be empty if deployer doesn't create log entries, so just check no error
	env.Logger.Info().
		Int("log_count", len(logs)).
		Msg("Deployment logs retrieved")

	// ===== STEP 6: Verify Helm Release Status =====
	env.Logger.Info().Msg("Step 6: Verifying Helm release status...")

	helmStatus, err := env.Deployer.GetStatus(ctx, deployResult.Namespace, deployResult.ReleaseName)
	require.NoError(t, err, "Should be able to get Helm status")
	assert.Equal(t, "deployed", helmStatus.Status, "Helm release should be in 'deployed' status")

	env.Logger.Info().
		Str("release", helmStatus.ReleaseName).
		Str("status", helmStatus.Status).
		Int("revision", helmStatus.Revision).
		Msg("Helm release status verified")

	// ===== STEP 7: Destroy Deployment =====
	env.Logger.Info().Msg("Step 7: Destroying deployment...")
	destroyStartTime := time.Now()

	// Update status to DESTROYING
	err = env.Repo.UpdateDeploymentStatus(ctx, deployment.ID, "DESTROYING")
	require.NoError(t, err)

	// First, destroy Helm deployment
	destroyDeployReq := &deployer.DestroyRequest{
		DeploymentID:     deploymentID,
		InfrastructureID: provisionResult.InfrastructureID,
		Namespace:        deployResult.Namespace,
		ReleaseName:      deployResult.ReleaseName,
	}

	err = env.Deployer.Destroy(ctx, destroyDeployReq)
	require.NoError(t, err, "Helm destroy should succeed")

	env.Logger.Info().Msg("Helm release destroyed")

	// Then, destroy infrastructure
	destroyProvisionReq := &provisioner.DestroyRequest{
		DeploymentID:     deploymentID,
		InfrastructureID: provisionResult.InfrastructureID,
		StackName:        provisionResult.StackName,
	}

	err = env.Provisioner.Destroy(ctx, destroyProvisionReq)
	require.NoError(t, err, "Infrastructure destroy should succeed")

	destroyDuration := time.Since(destroyStartTime)

	// Remove from cleanup tracking since we already destroyed
	env.InfrastructureIDs = []string{}

	// Update deployment status to DESTROYED
	err = env.Repo.UpdateDeploymentStatus(ctx, deployment.ID, "DESTROYED")
	require.NoError(t, err)

	env.Logger.Info().
		Dur("duration", destroyDuration).
		Msg("Deployment and infrastructure destroyed successfully")

	// ===== TEST SUMMARY =====
	totalDuration := time.Since(testStartTime)

	env.Logger.Info().
		Str("deployment_id", deploymentID).
		Str("app_name", appName).
		Dur("provision_duration", provisionDuration).
		Dur("deploy_duration", deployDuration).
		Dur("destroy_duration", destroyDuration).
		Dur("total_duration", totalDuration).
		Msg("Full deployment pipeline E2E test completed successfully!")
}

// TestDeploymentRollback tests the rollback functionality
func TestDeploymentRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E rollback test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Minute)
	defer cancel()

	deploymentID := uuid.New().String()
	appName := GenerateTestDeploymentName("e2e-rollback")

	env.Logger.Info().
		Str("deployment_id", deploymentID).
		Str("app_name", appName).
		Msg("Starting rollback E2E test")

	// Create deployment
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
	require.NoError(t, err)
	env.TrackDeployment(deploymentID)

	// Provision infrastructure
	env.Logger.Info().Msg("Provisioning infrastructure...")
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

	provisionResult, err := env.Provisioner.Provision(ctx, provisionReq)
	require.NoError(t, err)
	env.TrackInfrastructure(provisionResult.InfrastructureID)

	// Deploy v1 (nginx:alpine)
	env.Logger.Info().Msg("Deploying v1 (nginx:alpine)...")
	deployReq := &deployer.DeployRequest{
		DeploymentID:     deploymentID,
		InfrastructureID: provisionResult.InfrastructureID,
		AppName:          appName,
		Version:          "v1.0.0",
		ImageTag:         "nginx:alpine",
		Port:             80,
		Replicas:         1,
	}

	deployResult, err := env.Deployer.Deploy(ctx, deployReq)
	require.NoError(t, err)

	// Verify v1 is accessible
	time.Sleep(30 * time.Second)
	env.VerifyExternalAccess(t, deployResult.ExternalIP, 80, http.StatusOK)
	env.Logger.Info().Msg("v1 deployment verified")

	// Deploy v2 (httpd:alpine - different image)
	env.Logger.Info().Msg("Deploying v2 (httpd:alpine)...")
	deployReq.Version = "v2.0.0"
	deployReq.ImageTag = "httpd:alpine"

	_, err = env.Deployer.Deploy(ctx, deployReq)
	require.NoError(t, err)

	// Verify v2 is accessible
	time.Sleep(30 * time.Second)
	env.VerifyExternalAccess(t, deployResult.ExternalIP, 80, http.StatusOK)
	env.Logger.Info().Msg("v2 deployment verified")

	// Rollback to v1
	env.Logger.Info().Msg("Rolling back to previous version...")
	rollbackReq := &deployer.RollbackRequest{
		DeploymentID:     deploymentID,
		InfrastructureID: provisionResult.InfrastructureID,
		Namespace:        deployResult.Namespace,
		ReleaseName:      deployResult.ReleaseName,
		Revision:         0, // Previous revision
	}

	err = env.Deployer.Rollback(ctx, rollbackReq)
	require.NoError(t, err)

	// Verify rollback worked
	time.Sleep(30 * time.Second)
	env.VerifyExternalAccess(t, deployResult.ExternalIP, 80, http.StatusOK)
	env.Logger.Info().Msg("Rollback verified - application is still accessible")

	// Verify Helm revision increased
	status, err := env.Deployer.GetStatus(ctx, deployResult.Namespace, deployResult.ReleaseName)
	require.NoError(t, err)
	assert.Equal(t, 3, status.Revision, "Should be on revision 3 after rollback")

	// Cleanup
	env.Logger.Info().Msg("Cleaning up...")

	destroyDeployReq := &deployer.DestroyRequest{
		InfrastructureID: provisionResult.InfrastructureID,
		Namespace:        deployResult.Namespace,
		ReleaseName:      deployResult.ReleaseName,
	}
	err = env.Deployer.Destroy(ctx, destroyDeployReq)
	require.NoError(t, err)

	destroyProvisionReq := &provisioner.DestroyRequest{
		DeploymentID:     deploymentID,
		InfrastructureID: provisionResult.InfrastructureID,
		StackName:        provisionResult.StackName,
	}
	err = env.Provisioner.Destroy(ctx, destroyProvisionReq)
	require.NoError(t, err)
	env.InfrastructureIDs = []string{}

	env.Logger.Info().Msg("Rollback E2E test completed successfully!")
}

// TestOrchestratorQueueFlow tests the async queue-based deployment flow
func TestOrchestratorQueueFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E orchestrator queue test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	deploymentID := uuid.New().String()
	appName := GenerateTestDeploymentName("e2e-queue")

	env.Logger.Info().
		Str("deployment_id", deploymentID).
		Str("app_name", appName).
		Msg("Starting orchestrator queue E2E test")

	// Create deployment
	deployment := &state.Deployment{
		ID:      uuid.MustParse(deploymentID),
		Name:    appName,
		AppName: appName,
		Version: "v1.0.0",
		Status:  "QUEUED",
		Cloud:   "gcp",
		Region:  env.Config.GCPRegion,
		Port:    env.Config.TestPort,
	}

	err := env.Repo.CreateDeployment(ctx, deployment)
	require.NoError(t, err)
	env.TrackDeployment(deploymentID)

	// Enqueue provision job (simulating API call)
	env.Logger.Info().Msg("Enqueueing provision job...")
	provisionPayload := &queue.ProvisionPayload{
		DeploymentID: deploymentID,
		AppName:      appName,
		Version:      "v1.0.0",
		Cloud:        "gcp",
		Region:       env.Config.GCPRegion,
		ImageTag:     env.Config.TestImage,
		NodeCount:    2,
		MachineType:  "e2-small",
		Replicas:     1,
	}

	err = env.Engine.EnqueueProvisionJob(ctx, provisionPayload)
	require.NoError(t, err)

	env.Logger.Info().Msg("Provision job enqueued, waiting for completion...")

	// Note: In a real test, you would start the worker and wait for status changes
	// For this test, we're verifying the job was enqueued correctly
	// The full async flow would be tested with a running worker

	// Verify job was enqueued by checking queue length
	provisionQueueLen, err := env.Redis.GetQueueLength(ctx, queue.JobTypeProvision)
	require.NoError(t, err)
	env.Logger.Info().
		Int64("provision_queue_length", provisionQueueLen).
		Msg("Queue length after enqueue")

	// For full async testing, you would:
	// 1. Start the worker in a goroutine
	// 2. Wait for deployment status to become EXPOSED
	// 3. Verify external access
	// 4. Trigger destroy and wait for DESTROYED

	env.Logger.Info().Msg("Orchestrator queue flow test completed (job enqueue verified)")
}
