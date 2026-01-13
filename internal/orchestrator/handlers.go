package orchestrator

import (
	"context"
	"fmt"

	"github.com/alvesdmateus/app-deployer/internal/deployer"
	"github.com/alvesdmateus/app-deployer/internal/provisioner"
	"github.com/alvesdmateus/app-deployer/internal/queue"
	"github.com/google/uuid"
)

// handleProvisionJob handles infrastructure provisioning jobs
func (w *Worker) handleProvisionJob(ctx context.Context, job *queue.Job) error {
	logger := w.logger.With().
		Str("job_id", job.ID).
		Str("deployment_id", job.DeploymentID).
		Logger()

	logger.Info().Msg("Handling provision job")

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

	deployment, err := w.engine.repo.GetDeploymentByID(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	logger.Info().
		Str("app_name", payload.AppName).
		Str("cloud", payload.Cloud).
		Str("region", payload.Region).
		Msg("Starting infrastructure provisioning")

	// Apply defaults for infrastructure configuration
	nodeCount := payload.NodeCount
	if nodeCount == 0 {
		nodeCount = 2
	}
	machineType := payload.MachineType
	if machineType == "" {
		machineType = "e2-small"
	}

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

	// Provision infrastructure
	result, err := w.engine.provisioner.Provision(ctx, provisionReq)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("Infrastructure provisioning failed")

		// Update deployment status to FAILED
		deployment.Status = "FAILED"
		deployment.Error = err.Error()
		if updateErr := w.engine.repo.UpdateDeployment(ctx, deployment); updateErr != nil {
			logger.Error().
				Err(updateErr).
				Msg("Failed to update deployment status")
		}

		return fmt.Errorf("provision infrastructure: %w", err)
	}

	logger.Info().
		Str("infrastructure_id", result.InfrastructureID).
		Str("cluster_name", result.ClusterName).
		Str("namespace", result.Namespace).
		Msg("Infrastructure provisioning completed successfully")

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

	if err := w.engine.EnqueueDeployJob(ctx, deployPayload); err != nil {
		logger.Error().
			Err(err).
			Msg("Failed to enqueue deploy job")
		return fmt.Errorf("enqueue deploy job: %w", err)
	}

	logger.Info().Msg("Deploy job enqueued, provision job complete")
	return nil
}

// handleDeployJob handles Kubernetes deployment jobs
func (w *Worker) handleDeployJob(ctx context.Context, job *queue.Job) error {
	logger := w.logger.With().
		Str("job_id", job.ID).
		Str("deployment_id", job.DeploymentID).
		Logger()

	logger.Info().Msg("Handling deploy job")

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

	deployment, err := w.engine.repo.GetDeploymentByID(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	logger.Info().
		Str("infrastructure_id", payload.InfrastructureID).
		Str("image_tag", payload.ImageTag).
		Int("port", payload.Port).
		Int("replicas", payload.Replicas).
		Msg("Starting Kubernetes deployment")

	// Create deploy request
	deployReq := &deployer.DeployRequest{
		DeploymentID:     payload.DeploymentID,
		InfrastructureID: payload.InfrastructureID,
		ImageTag:         payload.ImageTag,
		Port:             payload.Port,
		Replicas:         payload.Replicas,
	}

	// Deploy to Kubernetes
	result, err := w.engine.deployer.Deploy(ctx, deployReq)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("Kubernetes deployment failed")

		// Update deployment status to FAILED (keep infrastructure for retry)
		deployment.Status = "FAILED"
		deployment.Error = err.Error()
		if updateErr := w.engine.repo.UpdateDeployment(ctx, deployment); updateErr != nil {
			logger.Error().
				Err(updateErr).
				Msg("Failed to update deployment status")
		}

		return fmt.Errorf("deploy to kubernetes: %w", err)
	}

	logger.Info().
		Str("namespace", result.Namespace).
		Str("release_name", result.ReleaseName).
		Str("external_ip", result.ExternalIP).
		Msg("Kubernetes deployment completed successfully")

	// Update deployment status to EXPOSED with external URL
	deployment.Status = "EXPOSED"
	deployment.ExternalURL = fmt.Sprintf("http://%s:%d", result.ExternalIP, payload.Port)
	deployment.Error = ""

	if err := w.engine.repo.UpdateDeployment(ctx, deployment); err != nil {
		logger.Error().
			Err(err).
			Msg("Failed to update deployment status")
		return fmt.Errorf("update deployment: %w", err)
	}

	logger.Info().
		Str("external_url", deployment.ExternalURL).
		Msg("Deploy job complete, application is live")

	return nil
}

// handleDestroyJob handles infrastructure and deployment destruction jobs
func (w *Worker) handleDestroyJob(ctx context.Context, job *queue.Job) error {
	logger := w.logger.With().
		Str("job_id", job.ID).
		Str("deployment_id", job.DeploymentID).
		Logger()

	logger.Info().Msg("Handling destroy job")

	// Parse destroy payload
	payload, err := parseDestroyPayload(job)
	if err != nil {
		return fmt.Errorf("parse destroy payload: %w", err)
	}

	// Get infrastructure from database
	infraID, err := uuid.Parse(payload.InfrastructureID)
	if err != nil {
		return fmt.Errorf("parse infrastructure ID: %w", err)
	}

	infra, err := w.engine.repo.GetInfrastructureByID(ctx, infraID)
	if err != nil {
		return fmt.Errorf("get infrastructure: %w", err)
	}

	logger.Info().
		Str("infrastructure_id", payload.InfrastructureID).
		Str("cluster_name", infra.ClusterName).
		Msg("Starting destruction process")

	// Step 1: Destroy Helm deployment if it exists
	if infra.HelmReleaseName != "" && infra.KubeNamespace != "" {
		logger.Info().
			Str("namespace", infra.KubeNamespace).
			Str("release", infra.HelmReleaseName).
			Msg("Destroying Helm release")

		destroyDeployReq := &deployer.DestroyRequest{
			InfrastructureID: payload.InfrastructureID,
			Namespace:        infra.KubeNamespace,
			ReleaseName:      infra.HelmReleaseName,
		}

		if err := w.engine.deployer.Destroy(ctx, destroyDeployReq); err != nil {
			logger.Warn().
				Err(err).
				Msg("Failed to destroy Helm release, continuing with infrastructure destruction")
			// Continue even if Helm destroy fails - we want to clean up infrastructure
		} else {
			logger.Info().Msg("Helm release destroyed successfully")
		}
	}

	// Step 2: Destroy infrastructure (Pulumi stack)
	logger.Info().
		Str("stack_name", infra.PulumiStackName).
		Msg("Destroying Pulumi stack")

	destroyProvisionReq := &provisioner.DestroyRequest{
		InfrastructureID: payload.InfrastructureID,
		StackName:        infra.PulumiStackName,
		DeploymentID:     payload.DeploymentID,
	}

	if err := w.engine.provisioner.Destroy(ctx, destroyProvisionReq); err != nil {
		logger.Error().
			Err(err).
			Msg("Failed to destroy infrastructure")
		return fmt.Errorf("destroy infrastructure: %w", err)
	}

	logger.Info().Msg("Infrastructure destroyed successfully")

	// Step 3: Update database - mark infrastructure as DESTROYED
	infra.Status = "DESTROYED"
	if err := w.engine.repo.UpdateInfrastructure(ctx, infra); err != nil {
		logger.Error().
			Err(err).
			Msg("Failed to update infrastructure status")
		return fmt.Errorf("update infrastructure: %w", err)
	}

	// Step 4: Update deployment status
	deploymentID, err := uuid.Parse(payload.DeploymentID)
	if err != nil {
		return fmt.Errorf("parse deployment ID: %w", err)
	}

	deployment, err := w.engine.repo.GetDeploymentByID(ctx, deploymentID)
	if err != nil {
		logger.Warn().
			Err(err).
			Msg("Failed to get deployment for status update")
	} else {
		deployment.Status = "DESTROYED"
		deployment.ExternalURL = ""
		if updateErr := w.engine.repo.UpdateDeployment(ctx, deployment); updateErr != nil {
			logger.Error().
				Err(updateErr).
				Msg("Failed to update deployment status")
		}
	}

	logger.Info().Msg("Destroy job complete")
	return nil
}

// handleRollbackJob handles deployment rollback jobs
func (w *Worker) handleRollbackJob(ctx context.Context, job *queue.Job) error {
	logger := w.logger.With().
		Str("job_id", job.ID).
		Str("deployment_id", job.DeploymentID).
		Logger()

	logger.Info().Msg("Handling rollback job")

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

	deployment, err := w.engine.repo.GetDeploymentByID(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	// Get infrastructure
	if deployment.InfrastructureID == nil {
		return fmt.Errorf("deployment has no infrastructure")
	}

	infra, err := w.engine.repo.GetInfrastructureByID(ctx, *deployment.InfrastructureID)
	if err != nil {
		return fmt.Errorf("get infrastructure: %w", err)
	}

	logger.Info().
		Str("current_version", deployment.Version).
		Str("target_version", payload.TargetVersion).
		Str("target_tag", payload.TargetTag).
		Msg("Starting rollback")

	// Create rollback request
	rollbackReq := &deployer.RollbackRequest{
		DeploymentID:     payload.DeploymentID,
		InfrastructureID: infra.ID.String(),
		Namespace:        infra.KubeNamespace,
		ReleaseName:      infra.HelmReleaseName,
		Revision:         0, // 0 means previous revision
	}

	if err := w.engine.deployer.Rollback(ctx, rollbackReq); err != nil {
		logger.Error().
			Err(err).
			Msg("Rollback failed")

		// Update deployment with error
		deployment.Error = fmt.Sprintf("rollback failed: %v", err)
		if updateErr := w.engine.repo.UpdateDeployment(ctx, deployment); updateErr != nil {
			logger.Error().
				Err(updateErr).
				Msg("Failed to update deployment with rollback error")
		}

		return fmt.Errorf("rollback deployment: %w", err)
	}

	logger.Info().Msg("Rollback completed successfully")

	// Update deployment version
	deployment.Version = payload.TargetVersion
	deployment.Status = "EXPOSED"
	deployment.Error = ""

	if err := w.engine.repo.UpdateDeployment(ctx, deployment); err != nil {
		logger.Error().
			Err(err).
			Msg("Failed to update deployment after rollback")
		return fmt.Errorf("update deployment: %w", err)
	}

	logger.Info().Msg("Rollback job complete")
	return nil
}
