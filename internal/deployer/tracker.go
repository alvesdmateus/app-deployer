package deployer

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/alvesdmateus/app-deployer/internal/state"
)

// Tracker tracks deployment state in the database
type Tracker struct {
	repo *state.Repository
}

// NewTracker creates a new deployment tracker
func NewTracker(repo *state.Repository) *Tracker {
	return &Tracker{repo: repo}
}

// StartDeployment marks deployment as starting
func (t *Tracker) StartDeployment(ctx context.Context, infraID string) error {
	log.Info().
		Str("infraID", infraID).
		Msg("Starting Kubernetes deployment")

	id, err := uuid.Parse(infraID)
	if err != nil {
		return fmt.Errorf("invalid infrastructure ID: %w", err)
	}

	// Get infrastructure
	infra, err := t.repo.GetInfrastructureByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get infrastructure: %w", err)
	}

	// Update deployment status to DEPLOYING
	if err := t.repo.UpdateDeploymentStatus(ctx, infra.DeploymentID, "DEPLOYING"); err != nil {
		log.Warn().Err(err).Msg("Failed to update deployment status to DEPLOYING")
	}

	log.Info().
		Str("infraID", infraID).
		Str("deploymentID", infra.DeploymentID.String()).
		Msg("Deployment tracking started")

	return nil
}

// CompleteDeployment marks deployment as complete with external IP
func (t *Tracker) CompleteDeployment(ctx context.Context, infraID string, result *DeployResult) error {
	log.Info().
		Str("infraID", infraID).
		Str("releaseName", result.ReleaseName).
		Str("externalIP", result.ExternalIP).
		Msg("Completing Kubernetes deployment")

	id, err := uuid.Parse(infraID)
	if err != nil {
		return fmt.Errorf("invalid infrastructure ID: %w", err)
	}

	// Get infrastructure
	infra, err := t.repo.GetInfrastructureByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get infrastructure: %w", err)
	}

	// Update infrastructure with Kubernetes details
	infra.KubeNamespace = result.Namespace
	infra.HelmReleaseName = result.ReleaseName
	infra.ExternalIP = result.ExternalIP

	if err := t.repo.UpdateInfrastructure(ctx, infra); err != nil {
		return fmt.Errorf("failed to update infrastructure: %w", err)
	}

	// Mark deployment as deployed with external IP/URL
	externalURL := result.ExternalURL
	if externalURL == "" && result.ExternalIP != "" {
		externalURL = fmt.Sprintf("http://%s", result.ExternalIP)
	}

	if err := t.repo.MarkDeploymentAsDeployed(ctx, infra.DeploymentID, result.ExternalIP, externalURL); err != nil {
		return fmt.Errorf("failed to mark deployment as deployed: %w", err)
	}

	log.Info().
		Str("infraID", infraID).
		Str("deploymentID", infra.DeploymentID.String()).
		Str("externalURL", externalURL).
		Msg("Deployment completed successfully")

	return nil
}

// FailDeployment marks deployment as failed
func (t *Tracker) FailDeployment(ctx context.Context, infraID string, deployErr error) error {
	log.Error().
		Err(deployErr).
		Str("infraID", infraID).
		Msg("Kubernetes deployment failed")

	id, err := uuid.Parse(infraID)
	if err != nil {
		return fmt.Errorf("invalid infrastructure ID: %w", err)
	}

	// Get infrastructure
	infra, err := t.repo.GetInfrastructureByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get infrastructure: %w", err)
	}

	// Update infrastructure with error
	infra.LastError = deployErr.Error()

	if err := t.repo.UpdateInfrastructure(ctx, infra); err != nil {
		return fmt.Errorf("failed to update infrastructure: %w", err)
	}

	// Update deployment status to FAILED
	if err := t.repo.UpdateDeploymentStatus(ctx, infra.DeploymentID, "FAILED"); err != nil {
		log.Warn().Err(err).Msg("Failed to update deployment status to FAILED")
	}

	log.Info().
		Str("infraID", infraID).
		Str("deploymentID", infra.DeploymentID.String()).
		Msg("Deployment failure recorded")

	return nil
}

// GetInfrastructure retrieves infrastructure by ID
func (t *Tracker) GetInfrastructure(ctx context.Context, infraID string) (*state.Infrastructure, error) {
	id, err := uuid.Parse(infraID)
	if err != nil {
		return nil, fmt.Errorf("invalid infrastructure ID: %w", err)
	}

	return t.repo.GetInfrastructureByID(ctx, id)
}

// GetInfrastructureByDeployment retrieves infrastructure by deployment ID
func (t *Tracker) GetInfrastructureByDeployment(ctx context.Context, deploymentID string) (*state.Infrastructure, error) {
	depID, err := uuid.Parse(deploymentID)
	if err != nil {
		return nil, fmt.Errorf("invalid deployment ID: %w", err)
	}

	return t.repo.GetInfrastructure(ctx, depID)
}
