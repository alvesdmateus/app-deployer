package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alvesdmateus/app-deployer/internal/queue"
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
					logger.Warn().
						Str("job_id", job.ID).
						Int("attempt", job.Attempts).
						Int("max_attempts", job.MaxAttempts).
						Msg("Requeueing failed job for retry")

					job.Attempts++
					if requeueErr := w.engine.queue.Enqueue(ctx, job); requeueErr != nil {
						logger.Error().
							Err(requeueErr).
							Str("job_id", job.ID).
							Msg("Failed to requeue job")
					}
				} else {
					logger.Error().
						Str("job_id", job.ID).
						Int("attempts", job.Attempts).
						Msg("Job failed after max attempts, marking deployment as failed")

					// Mark job as permanently failed
					if markErr := w.engine.queue.MarkFailed(ctx, job.ID, err); markErr != nil {
						logger.Error().
							Err(markErr).
							Str("job_id", job.ID).
							Msg("Failed to mark job as failed")
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
