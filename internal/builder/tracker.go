package builder

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/alvesdmateus/app-deployer/internal/queue"
	"github.com/alvesdmateus/app-deployer/internal/state"
)

// Orchestrator defines the interface for enqueueing jobs
type Orchestrator interface {
	EnqueueProvisionJob(ctx context.Context, payload *queue.ProvisionPayload) error
}

// Tracker implements BuildTracker interface
type Tracker struct {
	repo         *state.Repository
	orchestrator Orchestrator
}

// NewTracker creates a new build tracker
func NewTracker(repo *state.Repository) *Tracker {
	return &Tracker{
		repo:         repo,
		orchestrator: nil, // Optional, for backward compatibility
	}
}

// NewTrackerWithOrchestrator creates a new build tracker with orchestrator integration
func NewTrackerWithOrchestrator(repo *state.Repository, orchestrator Orchestrator) *Tracker {
	return &Tracker{
		repo:         repo,
		orchestrator: orchestrator,
	}
}

// StartBuild creates a new build record in the database
func (t *Tracker) StartBuild(ctx context.Context, deploymentID string) (*state.Build, error) {
	log.Info().
		Str("deploymentID", deploymentID).
		Msg("Starting new build")

	// Parse deployment ID
	depID, err := uuid.Parse(deploymentID)
	if err != nil {
		return nil, fmt.Errorf("invalid deployment ID: %w", err)
	}

	// Verify deployment exists
	deployment, err := t.repo.GetDeployment(ctx, depID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	if deployment == nil {
		return nil, fmt.Errorf("deployment not found: %s", deploymentID)
	}

	// Create build record
	build := &state.Build{
		ID:           uuid.New(),
		DeploymentID: depID,
		Status:       "BUILDING",
		ImageTag:     "", // Will be set when build completes
		BuildLog:     "",
		StartedAt:    time.Now(),
	}

	if err := t.repo.CreateBuild(ctx, build); err != nil {
		return nil, fmt.Errorf("failed to create build record: %w", err)
	}

	// Update deployment status to BUILDING
	if err := t.repo.UpdateDeploymentStatus(ctx, depID, "BUILDING"); err != nil {
		log.Warn().
			Err(err).
			Str("deploymentID", deploymentID).
			Msg("Failed to update deployment status to BUILDING")
		// Don't fail the build start, just log warning
	}

	log.Info().
		Str("buildID", build.ID.String()).
		Str("deploymentID", deploymentID).
		Msg("Build record created successfully")

	return build, nil
}

// UpdateProgress updates build progress with new log entries
func (t *Tracker) UpdateProgress(ctx context.Context, buildID string, logEntry string) error {
	if logEntry == "" {
		return nil // Skip empty log entries
	}

	log.Debug().
		Str("buildID", buildID).
		Msg("Updating build progress")

	// Parse build ID
	bID, err := uuid.Parse(buildID)
	if err != nil {
		return fmt.Errorf("invalid build ID: %w", err)
	}

	// Get current build
	build, err := t.repo.GetBuildByID(ctx, bID)
	if err != nil {
		return fmt.Errorf("failed to get build: %w", err)
	}

	if build == nil {
		return fmt.Errorf("build not found: %s", buildID)
	}

	// Append new log entry
	updatedLog := build.BuildLog + logEntry

	// Update build with new log
	build.BuildLog = updatedLog
	if err := t.repo.UpdateBuild(ctx, build); err != nil {
		return fmt.Errorf("failed to update build progress: %w", err)
	}

	return nil
}

// CompleteBuild marks a build as completed successfully
func (t *Tracker) CompleteBuild(ctx context.Context, buildID string, result *BuildResult) error {
	log.Info().
		Str("buildID", buildID).
		Str("imageTag", result.ImageTag).
		Dur("duration", result.BuildDuration).
		Msg("Completing build")

	// Parse build ID
	bID, err := uuid.Parse(buildID)
	if err != nil {
		return fmt.Errorf("invalid build ID: %w", err)
	}

	// Get current build
	build, err := t.repo.GetBuildByID(ctx, bID)
	if err != nil {
		return fmt.Errorf("failed to get build: %w", err)
	}

	if build == nil {
		return fmt.Errorf("build not found: %s", buildID)
	}

	// Update build with results
	build.Status = "COMPLETED"
	build.ImageTag = result.ImageTag
	build.BuildLog = result.BuildLog
	completedAt := time.Now()
	build.CompletedAt = &completedAt

	if err := t.repo.UpdateBuild(ctx, build); err != nil {
		return fmt.Errorf("failed to update build: %w", err)
	}

	// Get deployment for provisioning job
	deployment, err := t.repo.GetDeploymentByID(ctx, build.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// If orchestrator is configured, enqueue provision job
	if t.orchestrator != nil {
		log.Info().
			Str("buildID", buildID).
			Str("deploymentID", build.DeploymentID.String()).
			Msg("Enqueueing provision job")

		payload := &queue.ProvisionPayload{
			DeploymentID: build.DeploymentID.String(),
			AppName:      deployment.AppName,
			Version:      deployment.Version,
			Cloud:        deployment.Cloud,
			Region:       deployment.Region,
			ImageTag:     result.ImageTag,
			BuildID:      buildID,
		}

		if err := t.orchestrator.EnqueueProvisionJob(ctx, payload); err != nil {
			log.Error().
				Err(err).
				Str("deploymentID", build.DeploymentID.String()).
				Msg("Failed to enqueue provision job")
			return fmt.Errorf("enqueue provision job: %w", err)
		}

		log.Info().
			Str("buildID", buildID).
			Str("deploymentID", build.DeploymentID.String()).
			Msg("Provision job enqueued successfully")
	} else {
		// Fallback: Update deployment status directly (backward compatibility)
		log.Warn().
			Str("deploymentID", build.DeploymentID.String()).
			Msg("No orchestrator configured, updating deployment status directly")

		if err := t.repo.UpdateDeploymentStatus(ctx, build.DeploymentID, "PROVISIONING"); err != nil {
			log.Warn().
				Err(err).
				Str("deploymentID", build.DeploymentID.String()).
				Msg("Failed to update deployment status to PROVISIONING")
		}
	}

	log.Info().
		Str("buildID", buildID).
		Str("deploymentID", build.DeploymentID.String()).
		Msg("Build completed successfully")

	return nil
}

// FailBuild marks a build as failed
func (t *Tracker) FailBuild(ctx context.Context, buildID string, buildErr error) error {
	log.Error().
		Err(buildErr).
		Str("buildID", buildID).
		Msg("Build failed")

	// Parse build ID
	bID, err := uuid.Parse(buildID)
	if err != nil {
		return fmt.Errorf("invalid build ID: %w", err)
	}

	// Get current build
	build, err := t.repo.GetBuildByID(ctx, bID)
	if err != nil {
		return fmt.Errorf("failed to get build: %w", err)
	}

	if build == nil {
		return fmt.Errorf("build not found: %s", buildID)
	}

	// Update build with error
	build.Status = "FAILED"
	errorMsg := buildErr.Error()
	build.BuildLog += fmt.Sprintf("\n\nBUILD FAILED: %s\n", errorMsg)
	completedAt := time.Now()
	build.CompletedAt = &completedAt

	if err := t.repo.UpdateBuild(ctx, build); err != nil {
		return fmt.Errorf("failed to update build: %w", err)
	}

	// Update deployment status to FAILED
	if err := t.repo.UpdateDeploymentStatus(ctx, build.DeploymentID, "FAILED"); err != nil {
		log.Warn().
			Err(err).
			Str("deploymentID", build.DeploymentID.String()).
			Msg("Failed to update deployment status to FAILED")
	}

	log.Info().
		Str("buildID", buildID).
		Str("deploymentID", build.DeploymentID.String()).
		Msg("Build failure recorded")

	return nil
}

// GetBuildByID retrieves a build by its ID (helper method)
func (t *Tracker) GetBuildByID(ctx context.Context, buildID string) (*state.Build, error) {
	bID, err := uuid.Parse(buildID)
	if err != nil {
		return nil, fmt.Errorf("invalid build ID: %w", err)
	}
	return t.repo.GetBuildByID(ctx, bID)
}

// GetBuildsByDeployment retrieves all builds for a deployment
func (t *Tracker) GetBuildsByDeployment(ctx context.Context, deploymentID string) ([]*state.Build, error) {
	depID, err := uuid.Parse(deploymentID)
	if err != nil {
		return nil, fmt.Errorf("invalid deployment ID: %w", err)
	}

	// This would need to be implemented in the repository
	// For now, return the latest build
	build, err := t.repo.GetLatestBuild(ctx, depID)
	if err != nil {
		return nil, err
	}

	if build == nil {
		return []*state.Build{}, nil
	}

	return []*state.Build{build}, nil
}
