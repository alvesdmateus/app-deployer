package orchestrator

import (
	"context"
	"fmt"

	"github.com/alvesdmateus/app-deployer/internal/queue"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Client provides orchestration capabilities for the API layer
// It only requires a queue connection, not the full worker dependencies
type Client struct {
	queue  *queue.RedisQueue
	logger zerolog.Logger
}

// NewClient creates a new orchestrator client for API use
func NewClient(q *queue.RedisQueue, logger zerolog.Logger) *Client {
	return &Client{
		queue:  q,
		logger: logger.With().Str("component", "orchestrator-client").Logger(),
	}
}

// TriggerProvision enqueues a provision job to start infrastructure provisioning
func (c *Client) TriggerProvision(ctx context.Context, payload *queue.ProvisionPayload) error {
	c.logger.Info().
		Str("deployment_id", payload.DeploymentID).
		Str("app_name", payload.AppName).
		Str("cloud", payload.Cloud).
		Str("region", payload.Region).
		Msg("Triggering provision job")

	payloadMap := map[string]interface{}{
		"deployment_id": payload.DeploymentID,
		"app_name":      payload.AppName,
		"version":       payload.Version,
		"cloud":         payload.Cloud,
		"region":        payload.Region,
		"image_tag":     payload.ImageTag,
		"build_id":      payload.BuildID,
	}

	job := &queue.Job{
		ID:           uuid.New().String(),
		Type:         queue.JobTypeProvision,
		DeploymentID: payload.DeploymentID,
		Payload:      payloadMap,
		MaxAttempts:  3,
	}

	if err := c.queue.Enqueue(ctx, job); err != nil {
		c.logger.Error().
			Err(err).
			Str("deployment_id", payload.DeploymentID).
			Msg("Failed to enqueue provision job")
		return fmt.Errorf("enqueue provision job: %w", err)
	}

	c.logger.Info().
		Str("job_id", job.ID).
		Str("deployment_id", payload.DeploymentID).
		Msg("Provision job enqueued successfully")

	return nil
}

// TriggerDestroy enqueues a destroy job to tear down infrastructure
func (c *Client) TriggerDestroy(ctx context.Context, payload *queue.DestroyPayload) error {
	c.logger.Info().
		Str("deployment_id", payload.DeploymentID).
		Str("infrastructure_id", payload.InfrastructureID).
		Msg("Triggering destroy job")

	payloadMap := map[string]interface{}{
		"deployment_id":     payload.DeploymentID,
		"infrastructure_id": payload.InfrastructureID,
	}

	job := &queue.Job{
		ID:           uuid.New().String(),
		Type:         queue.JobTypeDestroy,
		DeploymentID: payload.DeploymentID,
		Payload:      payloadMap,
		MaxAttempts:  3,
	}

	if err := c.queue.Enqueue(ctx, job); err != nil {
		c.logger.Error().
			Err(err).
			Str("deployment_id", payload.DeploymentID).
			Msg("Failed to enqueue destroy job")
		return fmt.Errorf("enqueue destroy job: %w", err)
	}

	c.logger.Info().
		Str("job_id", job.ID).
		Str("deployment_id", payload.DeploymentID).
		Msg("Destroy job enqueued successfully")

	return nil
}

// TriggerRollback enqueues a rollback job
func (c *Client) TriggerRollback(ctx context.Context, payload *queue.RollbackPayload) error {
	c.logger.Info().
		Str("deployment_id", payload.DeploymentID).
		Str("target_version", payload.TargetVersion).
		Msg("Triggering rollback job")

	payloadMap := map[string]interface{}{
		"deployment_id":  payload.DeploymentID,
		"target_version": payload.TargetVersion,
		"target_tag":     payload.TargetTag,
	}

	job := &queue.Job{
		ID:           uuid.New().String(),
		Type:         queue.JobTypeRollback,
		DeploymentID: payload.DeploymentID,
		Payload:      payloadMap,
		MaxAttempts:  3,
	}

	if err := c.queue.Enqueue(ctx, job); err != nil {
		c.logger.Error().
			Err(err).
			Str("deployment_id", payload.DeploymentID).
			Msg("Failed to enqueue rollback job")
		return fmt.Errorf("enqueue rollback job: %w", err)
	}

	c.logger.Info().
		Str("job_id", job.ID).
		Str("deployment_id", payload.DeploymentID).
		Msg("Rollback job enqueued successfully")

	return nil
}

// GetQueueStats returns statistics about the job queues
func (c *Client) GetQueueStats(ctx context.Context) (map[string]int64, error) {
	stats := make(map[string]int64)

	jobTypes := []queue.JobType{
		queue.JobTypeProvision,
		queue.JobTypeDeploy,
		queue.JobTypeDestroy,
		queue.JobTypeRollback,
	}

	for _, jt := range jobTypes {
		length, err := c.queue.GetQueueLength(ctx, jt)
		if err != nil {
			return nil, fmt.Errorf("get queue length for %s: %w", jt, err)
		}
		stats[string(jt)] = length
	}

	return stats, nil
}

// Ping checks if the queue connection is alive
func (c *Client) Ping(ctx context.Context) error {
	return c.queue.Ping(ctx)
}
