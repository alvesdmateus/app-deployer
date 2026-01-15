package provisioner

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/alvesdmateus/app-deployer/internal/state"
)

// Tracker tracks infrastructure provisioning state in the database
type Tracker struct {
	repo *state.Repository
}

// NewTracker creates a new infrastructure tracker
func NewTracker(repo *state.Repository) *Tracker {
	return &Tracker{repo: repo}
}

// StartProvisioning creates an infrastructure record and marks deployment as PROVISIONING
func (t *Tracker) StartProvisioning(ctx context.Context, deploymentID, stackName string) (string, error) {
	log.Info().
		Str("deploymentID", deploymentID).
		Str("stackName", stackName).
		Msg("Starting infrastructure provisioning")

	depID, err := uuid.Parse(deploymentID)
	if err != nil {
		return "", fmt.Errorf("invalid deployment ID: %w", err)
	}

	// Check for existing infrastructure (idempotency)
	existing, err := t.repo.GetInfrastructure(ctx, depID)
	if err == nil && existing != nil {
		log.Info().
			Str("infraID", existing.ID.String()).
			Str("status", existing.Status).
			Msg("Infrastructure already exists, reusing")
		return existing.ID.String(), nil
	}

	// Check by stack name as well
	existingByStack, err := t.repo.GetInfrastructureByStackName(ctx, stackName)
	if err == nil && existingByStack != nil {
		log.Info().
			Str("infraID", existingByStack.ID.String()).
			Str("stackName", stackName).
			Msg("Infrastructure found by stack name, reusing")
		return existingByStack.ID.String(), nil
	}

	// Create new infrastructure record
	infra := &state.Infrastructure{
		ID:                uuid.New(),
		DeploymentID:      depID,
		PulumiStackName:   stackName,
		PulumiProjectName: "app-deployer",
		Status:            "PROVISIONING",
		ProvisionLog:      "",
	}

	if err := t.repo.CreateInfrastructure(ctx, infra); err != nil {
		return "", fmt.Errorf("failed to create infrastructure record: %w", err)
	}

	// Update deployment status to PROVISIONING
	if err := t.repo.UpdateDeploymentStatus(ctx, depID, "PROVISIONING"); err != nil {
		log.Warn().Err(err).Msg("Failed to update deployment status to PROVISIONING")
		// Don't fail the operation, just log warning
	}

	log.Info().
		Str("infraID", infra.ID.String()).
		Str("deploymentID", deploymentID).
		Msg("Infrastructure tracking started")

	return infra.ID.String(), nil
}

// UpdateProgress appends a log entry to the provision log
func (t *Tracker) UpdateProgress(ctx context.Context, infraID, logEntry string) error {
	if logEntry == "" {
		return nil // Skip empty log entries
	}

	log.Debug().
		Str("infraID", infraID).
		Msg("Updating provisioning progress")

	id, err := uuid.Parse(infraID)
	if err != nil {
		return fmt.Errorf("invalid infrastructure ID: %w", err)
	}

	return t.repo.AppendProvisionLog(ctx, id, logEntry)
}

// CompleteProvisioning marks provisioning as complete and updates infrastructure details
func (t *Tracker) CompleteProvisioning(ctx context.Context, infraID string, result *ProvisionResult) error {
	log.Info().
		Str("infraID", infraID).
		Str("clusterName", result.ClusterName).
		Dur("duration", result.Duration).
		Msg("Completing infrastructure provisioning")

	id, err := uuid.Parse(infraID)
	if err != nil {
		return fmt.Errorf("invalid infrastructure ID: %w", err)
	}

	// Get current infrastructure
	infra, err := t.repo.GetInfrastructureByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get infrastructure: %w", err)
	}

	// Update with provisioning results
	infra.Status = "READY"
	infra.ClusterName = result.ClusterName
	infra.ClusterEndpoint = result.ClusterEndpoint
	infra.ClusterCACert = result.ClusterCACert
	infra.ClusterLocation = result.ClusterLocation
	infra.VPCName = result.VPCName
	infra.VPCNetwork = result.VPCNetwork
	infra.SubnetName = result.SubnetName
	infra.SubnetCIDR = result.SubnetCIDR
	infra.ServiceAccountEmail = result.ServiceAccount
	infra.Namespace = result.Namespace
	infra.ProvisionLog += result.ProvisionLog

	if err := t.repo.UpdateInfrastructure(ctx, infra); err != nil {
		return fmt.Errorf("failed to update infrastructure: %w", err)
	}

	// Update deployment status to DEPLOYING (ready for K8s deployment phase)
	if err := t.repo.UpdateDeploymentStatus(ctx, infra.DeploymentID, "DEPLOYING"); err != nil {
		log.Warn().Err(err).Msg("Failed to update deployment status to DEPLOYING")
	}

	log.Info().
		Str("infraID", infraID).
		Str("deploymentID", infra.DeploymentID.String()).
		Msg("Infrastructure provisioning completed successfully")

	return nil
}

// FailProvisioning marks provisioning as failed and stores error
func (t *Tracker) FailProvisioning(ctx context.Context, infraID string, provisionErr error) error {
	log.Error().
		Err(provisionErr).
		Str("infraID", infraID).
		Msg("Infrastructure provisioning failed")

	id, err := uuid.Parse(infraID)
	if err != nil {
		return fmt.Errorf("invalid infrastructure ID: %w", err)
	}

	// Get current infrastructure
	infra, err := t.repo.GetInfrastructureByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get infrastructure: %w", err)
	}

	// Update with error details
	infra.Status = "FAILED"
	infra.LastError = provisionErr.Error()
	infra.ProvisionLog += fmt.Sprintf("\n\nPROVISIONING FAILED: %s\n", provisionErr.Error())

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
		Msg("Infrastructure failure recorded")

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
