package orchestrator

import (
	"context"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/alvesdmateus/app-deployer/internal/analyzer"
	"github.com/alvesdmateus/app-deployer/internal/builder"
	"github.com/alvesdmateus/app-deployer/internal/deployer"
	"github.com/alvesdmateus/app-deployer/internal/provisioner"
	"github.com/alvesdmateus/app-deployer/internal/queue"
	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/google/uuid"
)

// PhaseBuilding is the building phase constant
const PhaseBuilding = "BUILDING"

// handleBuildJob handles container image build jobs
func (w *Worker) handleBuildJob(ctx context.Context, job *queue.Job) error {
	// Parse build payload
	payload, err := parseBuildPayload(job)
	if err != nil {
		return fmt.Errorf("parse build payload: %w", err)
	}

	// Get deployment from database
	deploymentID, err := uuid.Parse(payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("parse deployment ID: %w", err)
	}

	// Create deployment logger for database logging
	deployLogger := NewDeploymentLogger(w.engine.repo, deploymentID, job.ID, PhaseBuilding, w.logger)

	deployLogger.Info(ctx, "Starting build job", Details(
		"app_name", payload.AppName,
		"repo_url", payload.RepoURL,
		"branch", payload.Branch,
	))

	deployment, err := w.engine.repo.GetDeploymentByID(ctx, deploymentID)
	if err != nil {
		deployLogger.Error(ctx, "Failed to get deployment from database", err, nil)
		return fmt.Errorf("get deployment: %w", err)
	}

	// Update deployment status to BUILDING
	deployment.Status = "BUILDING"
	if updateErr := w.engine.repo.UpdateDeployment(ctx, deployment); updateErr != nil {
		deployLogger.Warn(ctx, "Failed to update deployment status to BUILDING", Details(
			"error", updateErr.Error(),
		))
	}

	// Step 1: Clone repository
	deployLogger.Info(ctx, "Cloning repository", Details(
		"repo_url", payload.RepoURL,
		"branch", payload.Branch,
	))

	sourcePath, err := cloneRepository(ctx, payload.RepoURL, payload.Branch)
	if err != nil {
		deployLogger.Error(ctx, "Failed to clone repository", err, nil)
		return fmt.Errorf("clone repository: %w", err)
	}
	defer os.RemoveAll(sourcePath) // Clean up after build

	deployLogger.Info(ctx, "Repository cloned successfully", Details(
		"source_path", sourcePath,
	))

	// Step 2: Analyze repository
	deployLogger.Info(ctx, "Analyzing repository", nil)

	repoAnalyzer := analyzer.New()
	analysis, err := repoAnalyzer.Analyze(sourcePath)
	if err != nil {
		deployLogger.Error(ctx, "Failed to analyze repository", err, nil)
		return fmt.Errorf("analyze repository: %w", err)
	}

	deployLogger.Info(ctx, "Repository analyzed successfully", Details(
		"language", string(analysis.Language),
		"framework", string(analysis.Framework),
		"port", analysis.Port,
	))

	// Step 3: Build container image
	deployLogger.Info(ctx, "Building container image", Details(
		"strategy", payload.BuildStrategy,
	))

	// Check if builder is available
	if w.engine.builder == nil {
		deployLogger.Warn(ctx, "Builder not configured, skipping to provision with existing image", nil)

		// If no builder, try to proceed with provision using provided image tag
		// This allows testing without a full builder setup
		if deployment.Version != "" {
			return w.skipToProvision(ctx, deployment, payload, deployLogger)
		}
		return fmt.Errorf("builder not configured and no image tag provided")
	}

	buildCtx := &builder.BuildContext{
		DeploymentID: payload.DeploymentID,
		AppName:      payload.AppName,
		Version:      payload.Version,
		SourcePath:   sourcePath,
		Analysis:     analysis,
	}

	result, err := w.engine.builder.BuildImage(ctx, buildCtx)
	if err != nil {
		deployLogger.Error(ctx, "Container image build failed", err, nil)
		return fmt.Errorf("build image: %w", err)
	}

	deployLogger.Info(ctx, "Container image built successfully", Details(
		"image_tag", result.ImageTag,
		"duration", result.BuildDuration.String(),
	))

	// Step 4: Enqueue provision job
	// Note: The builder tracker handles this automatically on CompleteBuild
	// But we log it here for clarity
	deployLogger.Info(ctx, "Build completed, provision job will be enqueued by tracker", Details(
		"image_tag", result.ImageTag,
	))

	return nil
}

// cloneRepository clones a git repository to a temporary directory
func cloneRepository(ctx context.Context, repoURL, branch string) (string, error) {
	// Create temporary directory for the clone
	tempDir, err := os.MkdirTemp("", "deployer-build-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Set default branch
	if branch == "" {
		branch = "main"
	}

	// Clone options
	cloneOpts := &git.CloneOptions{
		URL:           repoURL,
		Progress:      nil, // Could wire up to logging
		Depth:         1,   // Shallow clone for speed
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
	}

	// Clone the repository
	_, err = git.PlainCloneContext(ctx, tempDir, false, cloneOpts)
	if err != nil {
		os.RemoveAll(tempDir) // Clean up on error
		return "", fmt.Errorf("git clone: %w", err)
	}

	return tempDir, nil
}

// skipToProvision enqueues a provision job directly (for testing without builder)
func (w *Worker) skipToProvision(ctx context.Context, deployment *state.Deployment, payload *queue.BuildPayload, deployLogger *DeploymentLogger) error {
	deployLogger.Info(ctx, "Skipping build, enqueueing provision job directly", nil)

	provisionPayload := &queue.ProvisionPayload{
		DeploymentID: payload.DeploymentID,
		AppName:      payload.AppName,
		Version:      payload.Version,
		Cloud:        payload.Cloud,
		Region:       payload.Region,
		ImageTag:     "", // Will need to be provided externally
	}

	if err := w.engine.EnqueueProvisionJob(ctx, provisionPayload); err != nil {
		deployLogger.Error(ctx, "Failed to enqueue provision job", err, nil)
		return fmt.Errorf("enqueue provision job: %w", err)
	}

	deployLogger.Info(ctx, "Provision job enqueued (skipped build)", nil)
	return nil
}

// handleProvisionJob handles infrastructure provisioning jobs
func (w *Worker) handleProvisionJob(ctx context.Context, job *queue.Job) error {
	// Parse provision payload
	payload, err := parseProvisionPayload(job)
	if err != nil {
		return fmt.Errorf("parse provision payload: %w", err)
	}

	// Get deployment from database
	deploymentID, err := uuid.Parse(payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("parse deployment ID: %w", err)
	}

	// Create deployment logger for database logging
	deployLogger := NewDeploymentLogger(w.engine.repo, deploymentID, job.ID, PhaseProvisioning, w.logger)

	deployLogger.Info(ctx, "Starting provision job", Details(
		"app_name", payload.AppName,
		"cloud", payload.Cloud,
		"region", payload.Region,
	))

	deployment, err := w.engine.repo.GetDeploymentByID(ctx, deploymentID)
	if err != nil {
		deployLogger.Error(ctx, "Failed to get deployment from database", err, nil)
		return fmt.Errorf("get deployment: %w", err)
	}

	// Apply defaults for infrastructure configuration
	nodeCount := payload.NodeCount
	if nodeCount == 0 {
		nodeCount = 2
	}
	machineType := payload.MachineType
	if machineType == "" {
		machineType = "e2-small"
	}

	deployLogger.Info(ctx, "Configuring infrastructure", Details(
		"node_count", nodeCount,
		"machine_type", machineType,
	))

	// Create provision request
	provisionReq := &provisioner.ProvisionRequest{
		DeploymentID: payload.DeploymentID,
		AppName:      payload.AppName,
		Version:      payload.Version,
		Cloud:        payload.Cloud,
		Region:       payload.Region,
		Config: &provisioner.ProvisionConfig{
			NodeCount:   nodeCount,
			MachineType: machineType,
		},
	}

	deployLogger.Info(ctx, "Creating VPC and GKE cluster", Details(
		"cloud", payload.Cloud,
		"region", payload.Region,
	))

	// Provision infrastructure
	result, err := w.engine.provisioner.Provision(ctx, provisionReq)
	if err != nil {
		deployLogger.Error(ctx, "Infrastructure provisioning failed", err, nil)

		// Update deployment status to FAILED
		deployment.Status = "FAILED"
		deployment.Error = err.Error()
		if updateErr := w.engine.repo.UpdateDeployment(ctx, deployment); updateErr != nil {
			deployLogger.Error(ctx, "Failed to update deployment status", updateErr, nil)
		}

		return fmt.Errorf("provision infrastructure: %w", err)
	}

	deployLogger.Info(ctx, "Infrastructure provisioning completed", Details(
		"infrastructure_id", result.InfrastructureID,
		"cluster_name", result.ClusterName,
		"namespace", result.Namespace,
	))

	// Apply defaults for replicas
	replicas := payload.Replicas
	if replicas == 0 {
		replicas = 2
	}

	// Enqueue deploy job
	deployPayload := &queue.DeployPayload{
		DeploymentID:     payload.DeploymentID,
		InfrastructureID: result.InfrastructureID,
		ImageTag:         payload.ImageTag,
		Port:             deployment.Port,
		Replicas:         replicas,
	}

	deployLogger.Info(ctx, "Enqueueing deployment job", Details(
		"image_tag", payload.ImageTag,
		"port", deployment.Port,
		"replicas", replicas,
	))

	if err := w.engine.EnqueueDeployJob(ctx, deployPayload); err != nil {
		deployLogger.Error(ctx, "Failed to enqueue deploy job", err, nil)
		return fmt.Errorf("enqueue deploy job: %w", err)
	}

	deployLogger.Info(ctx, "Provision job completed successfully", nil)
	return nil
}

// handleDeployJob handles Kubernetes deployment jobs
func (w *Worker) handleDeployJob(ctx context.Context, job *queue.Job) error {
	// Parse deploy payload
	payload, err := parseDeployPayload(job)
	if err != nil {
		return fmt.Errorf("parse deploy payload: %w", err)
	}

	// Get deployment from database
	deploymentID, err := uuid.Parse(payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("parse deployment ID: %w", err)
	}

	// Create deployment logger for database logging
	deployLogger := NewDeploymentLogger(w.engine.repo, deploymentID, job.ID, PhaseDeploying, w.logger)

	deployLogger.Info(ctx, "Starting deploy job", Details(
		"infrastructure_id", payload.InfrastructureID,
		"image_tag", payload.ImageTag,
		"port", payload.Port,
		"replicas", payload.Replicas,
	))

	deployment, err := w.engine.repo.GetDeploymentByID(ctx, deploymentID)
	if err != nil {
		deployLogger.Error(ctx, "Failed to get deployment from database", err, nil)
		return fmt.Errorf("get deployment: %w", err)
	}

	// Create deploy request
	deployReq := &deployer.DeployRequest{
		DeploymentID:     payload.DeploymentID,
		InfrastructureID: payload.InfrastructureID,
		AppName:          deployment.AppName,
		Version:          deployment.Version,
		ImageTag:         payload.ImageTag,
		Port:             payload.Port,
		Replicas:         payload.Replicas,
	}

	deployLogger.Info(ctx, "Deploying Helm chart to Kubernetes", Details(
		"image", payload.ImageTag,
		"replicas", payload.Replicas,
	))

	// Deploy to Kubernetes
	result, err := w.engine.deployer.Deploy(ctx, deployReq)
	if err != nil {
		deployLogger.Error(ctx, "Kubernetes deployment failed", err, nil)

		// Update deployment status to FAILED (keep infrastructure for retry)
		deployment.Status = "FAILED"
		deployment.Error = err.Error()
		if updateErr := w.engine.repo.UpdateDeployment(ctx, deployment); updateErr != nil {
			deployLogger.Error(ctx, "Failed to update deployment status", updateErr, nil)
		}

		return fmt.Errorf("deploy to kubernetes: %w", err)
	}

	deployLogger.Info(ctx, "Helm release deployed successfully", Details(
		"namespace", result.Namespace,
		"release_name", result.ReleaseName,
	))

	deployLogger.Info(ctx, "Waiting for pods to become ready", nil)

	// Update deployment status to EXPOSED with external URL
	externalURL := fmt.Sprintf("http://%s:%d", result.ExternalIP, payload.Port)
	deployment.Status = "EXPOSED"
	deployment.ExternalURL = externalURL
	deployment.ExternalIP = result.ExternalIP
	deployment.Error = ""

	if err := w.engine.repo.UpdateDeployment(ctx, deployment); err != nil {
		deployLogger.Error(ctx, "Failed to update deployment status", err, nil)
		return fmt.Errorf("update deployment: %w", err)
	}

	deployLogger.Info(ctx, "Deployment completed - application is live", Details(
		"external_ip", result.ExternalIP,
		"external_url", externalURL,
	))

	return nil
}

// handleDestroyJob handles infrastructure and deployment destruction jobs
func (w *Worker) handleDestroyJob(ctx context.Context, job *queue.Job) error {
	// Parse destroy payload
	payload, err := parseDestroyPayload(job)
	if err != nil {
		return fmt.Errorf("parse destroy payload: %w", err)
	}

	// Get deployment ID for logging
	deploymentID, err := uuid.Parse(payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("parse deployment ID: %w", err)
	}

	// Create deployment logger for database logging
	deployLogger := NewDeploymentLogger(w.engine.repo, deploymentID, job.ID, PhaseDestroying, w.logger)

	deployLogger.Info(ctx, "Starting destroy job", Details(
		"infrastructure_id", payload.InfrastructureID,
	))

	// Get infrastructure from database
	infraID, err := uuid.Parse(payload.InfrastructureID)
	if err != nil {
		deployLogger.Error(ctx, "Failed to parse infrastructure ID", err, nil)
		return fmt.Errorf("parse infrastructure ID: %w", err)
	}

	infra, err := w.engine.repo.GetInfrastructureByID(ctx, infraID)
	if err != nil {
		deployLogger.Error(ctx, "Failed to get infrastructure from database", err, nil)
		return fmt.Errorf("get infrastructure: %w", err)
	}

	deployLogger.Info(ctx, "Starting destruction process", Details(
		"cluster_name", infra.ClusterName,
	))

	// Step 1: Destroy Helm deployment if it exists
	if infra.HelmReleaseName != "" && infra.KubeNamespace != "" {
		deployLogger.Info(ctx, "Destroying Helm release", Details(
			"namespace", infra.KubeNamespace,
			"release", infra.HelmReleaseName,
		))

		destroyDeployReq := &deployer.DestroyRequest{
			InfrastructureID: payload.InfrastructureID,
			Namespace:        infra.KubeNamespace,
			ReleaseName:      infra.HelmReleaseName,
		}

		if err := w.engine.deployer.Destroy(ctx, destroyDeployReq); err != nil {
			deployLogger.Warn(ctx, "Failed to destroy Helm release, continuing with infrastructure destruction", Details(
				"error", err.Error(),
			))
			// Continue even if Helm destroy fails - we want to clean up infrastructure
		} else {
			deployLogger.Info(ctx, "Helm release destroyed successfully", nil)
		}
	}

	// Step 2: Destroy infrastructure (Pulumi stack)
	deployLogger.Info(ctx, "Destroying Pulumi stack and cloud resources", Details(
		"stack_name", infra.PulumiStackName,
	))

	destroyProvisionReq := &provisioner.DestroyRequest{
		InfrastructureID: payload.InfrastructureID,
		StackName:        infra.PulumiStackName,
		DeploymentID:     payload.DeploymentID,
	}

	if err := w.engine.provisioner.Destroy(ctx, destroyProvisionReq); err != nil {
		deployLogger.Error(ctx, "Failed to destroy infrastructure", err, nil)
		return fmt.Errorf("destroy infrastructure: %w", err)
	}

	deployLogger.Info(ctx, "Infrastructure destroyed successfully", nil)

	// Step 3: Update database - mark infrastructure as DESTROYED
	infra.Status = "DESTROYED"
	if err := w.engine.repo.UpdateInfrastructure(ctx, infra); err != nil {
		deployLogger.Error(ctx, "Failed to update infrastructure status", err, nil)
		return fmt.Errorf("update infrastructure: %w", err)
	}

	// Step 4: Update deployment status
	deployment, err := w.engine.repo.GetDeploymentByID(ctx, deploymentID)
	if err != nil {
		deployLogger.Warn(ctx, "Failed to get deployment for status update", Details(
			"error", err.Error(),
		))
	} else {
		deployment.Status = "DESTROYED"
		deployment.ExternalURL = ""
		deployment.ExternalIP = ""
		if updateErr := w.engine.repo.UpdateDeployment(ctx, deployment); updateErr != nil {
			deployLogger.Error(ctx, "Failed to update deployment status", updateErr, nil)
		}
	}

	deployLogger.Info(ctx, "Destroy job completed successfully", nil)
	return nil
}

// handleRollbackJob handles deployment rollback jobs
func (w *Worker) handleRollbackJob(ctx context.Context, job *queue.Job) error {
	// Parse rollback payload
	payload, err := parseRollbackPayload(job)
	if err != nil {
		return fmt.Errorf("parse rollback payload: %w", err)
	}

	// Get deployment from database
	deploymentID, err := uuid.Parse(payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("parse deployment ID: %w", err)
	}

	// Create deployment logger for database logging
	deployLogger := NewDeploymentLogger(w.engine.repo, deploymentID, job.ID, PhaseRollingBack, w.logger)

	deployLogger.Info(ctx, "Starting rollback job", Details(
		"target_version", payload.TargetVersion,
		"target_tag", payload.TargetTag,
	))

	deployment, err := w.engine.repo.GetDeploymentByID(ctx, deploymentID)
	if err != nil {
		deployLogger.Error(ctx, "Failed to get deployment from database", err, nil)
		return fmt.Errorf("get deployment: %w", err)
	}

	// Get infrastructure
	if deployment.InfrastructureID == nil {
		deployLogger.Error(ctx, "Deployment has no infrastructure", nil, nil)
		return fmt.Errorf("deployment has no infrastructure")
	}

	infra, err := w.engine.repo.GetInfrastructureByID(ctx, *deployment.InfrastructureID)
	if err != nil {
		deployLogger.Error(ctx, "Failed to get infrastructure from database", err, nil)
		return fmt.Errorf("get infrastructure: %w", err)
	}

	deployLogger.Info(ctx, "Rolling back to previous version", Details(
		"current_version", deployment.Version,
		"target_version", payload.TargetVersion,
		"namespace", infra.KubeNamespace,
		"release", infra.HelmReleaseName,
	))

	// Create rollback request
	rollbackReq := &deployer.RollbackRequest{
		DeploymentID:     payload.DeploymentID,
		InfrastructureID: infra.ID.String(),
		Namespace:        infra.KubeNamespace,
		ReleaseName:      infra.HelmReleaseName,
		Revision:         0, // 0 means previous revision
	}

	if err := w.engine.deployer.Rollback(ctx, rollbackReq); err != nil {
		deployLogger.Error(ctx, "Rollback failed", err, nil)

		// Update deployment with error
		deployment.Error = fmt.Sprintf("rollback failed: %v", err)
		if updateErr := w.engine.repo.UpdateDeployment(ctx, deployment); updateErr != nil {
			deployLogger.Error(ctx, "Failed to update deployment with rollback error", updateErr, nil)
		}

		return fmt.Errorf("rollback deployment: %w", err)
	}

	deployLogger.Info(ctx, "Helm rollback completed successfully", nil)

	// Update deployment version
	deployment.Version = payload.TargetVersion
	deployment.Status = "EXPOSED"
	deployment.Error = ""

	if err := w.engine.repo.UpdateDeployment(ctx, deployment); err != nil {
		deployLogger.Error(ctx, "Failed to update deployment after rollback", err, nil)
		return fmt.Errorf("update deployment: %w", err)
	}

	deployLogger.Info(ctx, "Rollback job completed successfully", Details(
		"new_version", payload.TargetVersion,
	))
	return nil
}
