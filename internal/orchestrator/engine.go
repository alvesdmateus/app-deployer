package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alvesdmateus/app-deployer/internal/builder"
	"github.com/alvesdmateus/app-deployer/internal/deployer"
	"github.com/alvesdmateus/app-deployer/internal/provisioner"
	"github.com/alvesdmateus/app-deployer/internal/queue"
	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Engine orchestrates the deployment pipeline by coordinating queue, provisioner, and deployer
type Engine struct {
	queue       *queue.RedisQueue
	repo        *state.Repository
	builder     builder.BuildService
	provisioner provisioner.Provisioner
	deployer    deployer.Deployer
	logger      zerolog.Logger
}

// NewEngine creates a new orchestrator engine
func NewEngine(
	queue *queue.RedisQueue,
	repo *state.Repository,
	builder builder.BuildService,
	provisioner provisioner.Provisioner,
	deployer deployer.Deployer,
	logger zerolog.Logger,
) *Engine {
	return &Engine{
		queue:       queue,
		repo:        repo,
		builder:     builder,
		provisioner: provisioner,
		deployer:    deployer,
		logger:      logger.With().Str("component", "orchestrator").Logger(),
	}
}

// EnqueueBuildJob enqueues a build job to the queue
func (e *Engine) EnqueueBuildJob(ctx context.Context, payload *queue.BuildPayload) error {
	e.logger.Info().
		Str("deployment_id", payload.DeploymentID).
		Str("app_name", payload.AppName).
		Str("repo_url", payload.RepoURL).
		Msg("Enqueueing build job")

	payloadMap := map[string]interface{}{
		"deployment_id":  payload.DeploymentID,
		"app_name":       payload.AppName,
		"version":        payload.Version,
		"repo_url":       payload.RepoURL,
		"branch":         payload.Branch,
		"commit_sha":     payload.CommitSHA,
		"build_strategy": payload.BuildStrategy,
		"dockerfile":     payload.Dockerfile,
		"cloud":          payload.Cloud,
		"region":         payload.Region,
	}

	job := &queue.Job{
		ID:           uuid.New().String(),
		Type:         queue.JobTypeBuild,
		DeploymentID: payload.DeploymentID,
		Payload:      payloadMap,
		MaxAttempts:  3,
	}

	if err := e.queue.Enqueue(ctx, job); err != nil {
		e.logger.Error().
			Err(err).
			Str("deployment_id", payload.DeploymentID).
			Msg("Failed to enqueue build job")
		return fmt.Errorf("enqueue build job: %w", err)
	}

	e.logger.Info().
		Str("job_id", job.ID).
		Str("deployment_id", payload.DeploymentID).
		Msg("Build job enqueued successfully")

	return nil
}

// EnqueueProvisionJob enqueues a provision job to the queue
func (e *Engine) EnqueueProvisionJob(ctx context.Context, payload *queue.ProvisionPayload) error {
	e.logger.Info().
		Str("deployment_id", payload.DeploymentID).
		Str("app_name", payload.AppName).
		Str("cloud", payload.Cloud).
		Msg("Enqueueing provision job")

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

	if err := e.queue.Enqueue(ctx, job); err != nil {
		e.logger.Error().
			Err(err).
			Str("deployment_id", payload.DeploymentID).
			Msg("Failed to enqueue provision job")
		return fmt.Errorf("enqueue provision job: %w", err)
	}

	e.logger.Info().
		Str("job_id", job.ID).
		Str("deployment_id", payload.DeploymentID).
		Msg("Provision job enqueued successfully")

	return nil
}

// EnqueueDeployJob enqueues a deploy job to the queue
func (e *Engine) EnqueueDeployJob(ctx context.Context, payload *queue.DeployPayload) error {
	e.logger.Info().
		Str("deployment_id", payload.DeploymentID).
		Str("infrastructure_id", payload.InfrastructureID).
		Str("image_tag", payload.ImageTag).
		Msg("Enqueueing deploy job")

	payloadMap := map[string]interface{}{
		"deployment_id":     payload.DeploymentID,
		"infrastructure_id": payload.InfrastructureID,
		"image_tag":         payload.ImageTag,
		"port":              payload.Port,
		"replicas":          payload.Replicas,
	}

	job := &queue.Job{
		ID:           uuid.New().String(),
		Type:         queue.JobTypeDeploy,
		DeploymentID: payload.DeploymentID,
		Payload:      payloadMap,
		MaxAttempts:  3,
	}

	if err := e.queue.Enqueue(ctx, job); err != nil {
		e.logger.Error().
			Err(err).
			Str("deployment_id", payload.DeploymentID).
			Msg("Failed to enqueue deploy job")
		return fmt.Errorf("enqueue deploy job: %w", err)
	}

	e.logger.Info().
		Str("job_id", job.ID).
		Str("deployment_id", payload.DeploymentID).
		Msg("Deploy job enqueued successfully")

	return nil
}

// EnqueueDestroyJob enqueues a destroy job to the queue
func (e *Engine) EnqueueDestroyJob(ctx context.Context, payload *queue.DestroyPayload) error {
	e.logger.Info().
		Str("deployment_id", payload.DeploymentID).
		Str("infrastructure_id", payload.InfrastructureID).
		Msg("Enqueueing destroy job")

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

	if err := e.queue.Enqueue(ctx, job); err != nil {
		e.logger.Error().
			Err(err).
			Str("deployment_id", payload.DeploymentID).
			Msg("Failed to enqueue destroy job")
		return fmt.Errorf("enqueue destroy job: %w", err)
	}

	e.logger.Info().
		Str("job_id", job.ID).
		Str("deployment_id", payload.DeploymentID).
		Msg("Destroy job enqueued successfully")

	return nil
}

// EnqueueRollbackJob enqueues a rollback job to the queue
func (e *Engine) EnqueueRollbackJob(ctx context.Context, payload *queue.RollbackPayload) error {
	e.logger.Info().
		Str("deployment_id", payload.DeploymentID).
		Str("target_version", payload.TargetVersion).
		Msg("Enqueueing rollback job")

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

	if err := e.queue.Enqueue(ctx, job); err != nil {
		e.logger.Error().
			Err(err).
			Str("deployment_id", payload.DeploymentID).
			Msg("Failed to enqueue rollback job")
		return fmt.Errorf("enqueue rollback job: %w", err)
	}

	e.logger.Info().
		Str("job_id", job.ID).
		Str("deployment_id", payload.DeploymentID).
		Msg("Rollback job enqueued successfully")

	return nil
}

// parseBuildPayload parses a build job payload
func parseBuildPayload(job *queue.Job) (*queue.BuildPayload, error) {
	data, err := json.Marshal(job.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	var payload queue.BuildPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	return &payload, nil
}

// parseProvisionPayload parses a provision job payload
func parseProvisionPayload(job *queue.Job) (*queue.ProvisionPayload, error) {
	data, err := json.Marshal(job.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	var payload queue.ProvisionPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	return &payload, nil
}

// parseDeployPayload parses a deploy job payload
func parseDeployPayload(job *queue.Job) (*queue.DeployPayload, error) {
	data, err := json.Marshal(job.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	var payload queue.DeployPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	return &payload, nil
}

// parseDestroyPayload parses a destroy job payload
func parseDestroyPayload(job *queue.Job) (*queue.DestroyPayload, error) {
	data, err := json.Marshal(job.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	var payload queue.DestroyPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	return &payload, nil
}

// parseRollbackPayload parses a rollback job payload
func parseRollbackPayload(job *queue.Job) (*queue.RollbackPayload, error) {
	data, err := json.Marshal(job.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	var payload queue.RollbackPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	return &payload, nil
}
