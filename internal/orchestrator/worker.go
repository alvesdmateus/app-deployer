package orchestrator

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/alvesdmateus/app-deployer/internal/queue"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Worker processes jobs from the queue with configurable concurrency
type Worker struct {
	engine      *Engine
	concurrency int
	pollTimeout time.Duration
	logger      zerolog.Logger
}

// NewWorker creates a new worker
func NewWorker(engine *Engine, concurrency int, logger zerolog.Logger) *Worker {
	if concurrency < 1 {
		concurrency = 1
	}

	return &Worker{
		engine:      engine,
		concurrency: concurrency,
		pollTimeout: 5 * time.Second, // Blocking poll timeout
		logger:      logger.With().Str("component", "worker").Logger(),
	}
}

// Start starts the worker with N concurrent job processors
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info().
		Int("concurrency", w.concurrency).
		Msg("Starting orchestrator worker")

	var wg sync.WaitGroup

	// Start N worker goroutines
	for i := 0; i < w.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			w.processJobs(ctx, workerID)
		}(i)
	}

	// Wait for all workers to finish (when context is cancelled)
	wg.Wait()

	w.logger.Info().Msg("Orchestrator worker stopped")
	return nil
}

// processJobs is the main worker loop that processes jobs from the queue
func (w *Worker) processJobs(ctx context.Context, workerID int) {
	logger := w.logger.With().Int("worker_id", workerID).Logger()
	logger.Info().Msg("Worker goroutine started")

	// Round-robin between job types for fair processing
	jobTypes := []queue.JobType{
		queue.JobTypeBuild,
		queue.JobTypeProvision,
		queue.JobTypeDeploy,
		queue.JobTypeDestroy,
		queue.JobTypeRollback,
	}
	currentTypeIndex := 0

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Worker goroutine stopped (context cancelled)")
			return
		default:
			// Try to dequeue from current job type
			jobType := jobTypes[currentTypeIndex]
			job, err := w.engine.queue.Dequeue(ctx, jobType, w.pollTimeout)

			if err != nil {
				// Log non-timeout errors
				if err.Error() != "redis: nil" {
					logger.Error().
						Err(err).
						Str("job_type", string(jobType)).
						Msg("Failed to dequeue job")
				}

				// Move to next job type for round-robin
				currentTypeIndex = (currentTypeIndex + 1) % len(jobTypes)
				continue
			}

			if job == nil {
				// No job available, move to next job type
				currentTypeIndex = (currentTypeIndex + 1) % len(jobTypes)
				continue
			}

			// Process the job
			logger.Info().
				Str("job_id", job.ID).
				Str("job_type", string(job.Type)).
				Str("deployment_id", job.DeploymentID).
				Int("attempt", job.Attempts).
				Msg("Processing job")

			if err := w.handleJob(ctx, job); err != nil {
				logger.Error().
					Err(err).
					Str("job_id", job.ID).
					Str("job_type", string(job.Type)).
					Str("deployment_id", job.DeploymentID).
					Msg("Job processing failed")

				// Handle job retry or failure
				if job.Attempts < job.MaxAttempts {
					job.Attempts++
					job.LastError = err.Error()

					// Calculate backoff delay
					backoffDelay := calculateBackoff(job.Attempts)
					job.NextRetryAt = time.Now().Add(backoffDelay)

					logger.Warn().
						Str("job_id", job.ID).
						Int("attempt", job.Attempts).
						Int("max_attempts", job.MaxAttempts).
						Dur("backoff_delay", backoffDelay).
						Time("next_retry_at", job.NextRetryAt).
						Msg("Requeueing failed job for retry with backoff")

					// Wait for backoff before requeueing (simple approach)
					// For production, use a delayed queue or scheduler
					go func(j *queue.Job, delay time.Duration) {
						time.Sleep(delay)
						if requeueErr := w.engine.queue.Enqueue(context.Background(), j); requeueErr != nil {
							logger.Error().
								Err(requeueErr).
								Str("job_id", j.ID).
								Msg("Failed to requeue job after backoff")
						}
					}(job, backoffDelay)
				} else {
					logger.Error().
						Str("job_id", job.ID).
						Int("attempts", job.Attempts).
						Str("last_error", err.Error()).
						Msg("Job failed after max attempts, marking deployment as failed")

					// Mark job as permanently failed
					if markErr := w.engine.queue.MarkFailed(ctx, job.ID, err); markErr != nil {
						logger.Error().
							Err(markErr).
							Str("job_id", job.ID).
							Msg("Failed to mark job as failed")
					}

					// Trigger automatic rollback for failed deploy jobs
					if job.Type == queue.JobTypeDeploy {
						w.triggerAutoRollback(ctx, job, logger)
					}
				}
			} else {
				logger.Info().
					Str("job_id", job.ID).
					Str("job_type", string(job.Type)).
					Str("deployment_id", job.DeploymentID).
					Msg("Job processed successfully")

				// Mark job as complete
				if err := w.engine.queue.MarkComplete(ctx, job.ID); err != nil {
					logger.Error().
						Err(err).
						Str("job_id", job.ID).
						Msg("Failed to mark job as complete")
				}
			}

			// Move to next job type for round-robin
			currentTypeIndex = (currentTypeIndex + 1) % len(jobTypes)
		}
	}
}

// handleJob routes a job to the appropriate handler based on job type
func (w *Worker) handleJob(ctx context.Context, job *queue.Job) error {
	switch job.Type {
	case queue.JobTypeBuild:
		return w.handleBuildJob(ctx, job)
	case queue.JobTypeProvision:
		return w.handleProvisionJob(ctx, job)
	case queue.JobTypeDeploy:
		return w.handleDeployJob(ctx, job)
	case queue.JobTypeDestroy:
		return w.handleDestroyJob(ctx, job)
	case queue.JobTypeRollback:
		return w.handleRollbackJob(ctx, job)
	default:
		return fmt.Errorf("unknown job type: %s", job.Type)
	}
}

// triggerAutoRollback attempts to rollback a failed deployment to the previous version
func (w *Worker) triggerAutoRollback(ctx context.Context, failedJob *queue.Job, logger zerolog.Logger) {
	logger.Info().
		Str("job_id", failedJob.ID).
		Str("deployment_id", failedJob.DeploymentID).
		Msg("Attempting automatic rollback after deploy failure")

	// Parse the failed deploy job payload
	payload, err := parseDeployPayloadFromJob(failedJob)
	if err != nil {
		logger.Error().
			Err(err).
			Str("job_id", failedJob.ID).
			Msg("Failed to parse deploy payload for rollback")
		return
	}

	// Get deployment to check if rollback is possible
	deploymentID, err := uuid.Parse(payload.DeploymentID)
	if err != nil {
		logger.Error().
			Err(err).
			Str("deployment_id", payload.DeploymentID).
			Msg("Failed to parse deployment ID for rollback")
		return
	}

	deployment, err := w.engine.repo.GetDeploymentByID(ctx, deploymentID)
	if err != nil {
		logger.Error().
			Err(err).
			Str("deployment_id", payload.DeploymentID).
			Msg("Failed to get deployment for rollback")
		return
	}

	// Check if there's infrastructure to rollback on
	if deployment.InfrastructureID == nil {
		logger.Warn().
			Str("deployment_id", payload.DeploymentID).
			Msg("No infrastructure found, skipping rollback")
		return
	}

	// Get infrastructure to get Helm release info
	infra, err := w.engine.repo.GetInfrastructureByID(ctx, *deployment.InfrastructureID)
	if err != nil {
		logger.Error().
			Err(err).
			Str("deployment_id", payload.DeploymentID).
			Msg("Failed to get infrastructure for rollback")
		return
	}

	// Only rollback if there's a Helm release
	if infra.HelmReleaseName == "" {
		logger.Warn().
			Str("deployment_id", payload.DeploymentID).
			Msg("No Helm release found, skipping rollback")
		return
	}

	// Enqueue rollback job
	rollbackPayload := &queue.RollbackPayload{
		DeploymentID:  payload.DeploymentID,
		TargetVersion: "previous", // Helm will rollback to previous revision
		TargetTag:     "",         // Not needed for Helm rollback
	}

	if err := w.engine.EnqueueRollbackJob(ctx, rollbackPayload); err != nil {
		logger.Error().
			Err(err).
			Str("deployment_id", payload.DeploymentID).
			Msg("Failed to enqueue rollback job")
		return
	}

	logger.Info().
		Str("deployment_id", payload.DeploymentID).
		Msg("Automatic rollback job enqueued successfully")
}

// parseDeployPayloadFromJob extracts deploy payload from a job (simplified for rollback)
func parseDeployPayloadFromJob(job *queue.Job) (*queue.DeployPayload, error) {
	deploymentID, _ := job.Payload["deployment_id"].(string)
	infraID, _ := job.Payload["infrastructure_id"].(string)
	imageTag, _ := job.Payload["image_tag"].(string)
	port, _ := job.Payload["port"].(float64)
	replicas, _ := job.Payload["replicas"].(float64)

	return &queue.DeployPayload{
		DeploymentID:     deploymentID,
		InfrastructureID: infraID,
		ImageTag:         imageTag,
		Port:             int(port),
		Replicas:         int(replicas),
	}, nil
}

// calculateBackoff calculates the backoff delay with exponential growth and jitter
func calculateBackoff(attempt int) time.Duration {
	// Calculate exponential delay: base * multiplier^attempt
	delay := float64(queue.BaseBackoffDelay) * math.Pow(queue.BackoffMultiplier, float64(attempt-1))

	// Cap at max delay
	if delay > float64(queue.MaxBackoffDelay) {
		delay = float64(queue.MaxBackoffDelay)
	}

	// Add jitter (±10%)
	jitter := delay * queue.BackoffJitterPercent * (2*rand.Float64() - 1)
	delay += jitter

	return time.Duration(delay)
}
