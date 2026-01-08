package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// RedisQueue implements a job queue using Redis
type RedisQueue struct {
	client *redis.Client
}

// NewRedisQueue creates a new Redis-based job queue
func NewRedisQueue(url, password string, db int) (*RedisQueue, error) {
	// Parse Redis URL if needed (for now, assume simple host:port format)
	client := redis.NewClient(&redis.Options{
		Addr:     url,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	log.Info().
		Str("addr", url).
		Int("db", db).
		Msg("Redis queue connected successfully")

	return &RedisQueue{client: client}, nil
}

// Enqueue adds a job to the queue
func (q *RedisQueue) Enqueue(ctx context.Context, job *Job) error {
	// Serialize job to JSON
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Queue key based on job type for prioritization
	queueKey := fmt.Sprintf("queue:%s", job.Type)

	// Push to the end of the list (FIFO)
	if err := q.client.RPush(ctx, queueKey, data).Err(); err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	log.Info().
		Str("jobID", job.ID).
		Str("type", string(job.Type)).
		Str("deploymentID", job.DeploymentID).
		Msg("Job enqueued")

	return nil
}

// Dequeue retrieves and removes a job from the queue (blocking)
func (q *RedisQueue) Dequeue(ctx context.Context, jobType JobType, timeout time.Duration) (*Job, error) {
	queueKey := fmt.Sprintf("queue:%s", jobType)

	// Blocking pop from the queue (BLPOP)
	result, err := q.client.BLPop(ctx, timeout, queueKey).Result()
	if err != nil {
		if err == redis.Nil {
			// No job available within timeout - this is normal
			return nil, nil
		}
		return nil, fmt.Errorf("failed to dequeue job: %w", err)
	}

	// BLPOP returns [key, value]
	if len(result) < 2 {
		return nil, fmt.Errorf("unexpected redis response: %v", result)
	}

	// Parse job from JSON
	var job Job
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	log.Debug().
		Str("jobID", job.ID).
		Str("type", string(job.Type)).
		Str("deploymentID", job.DeploymentID).
		Msg("Job dequeued")

	return &job, nil
}

// MarkProcessing marks a job as being processed (for tracking)
func (q *RedisQueue) MarkProcessing(ctx context.Context, jobID string) error {
	key := fmt.Sprintf("job:processing:%s", jobID)
	timestamp := time.Now().Unix()

	// Set with 1-hour expiration (TTL)
	if err := q.client.Set(ctx, key, timestamp, 1*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to mark job as processing: %w", err)
	}

	return nil
}

// MarkComplete removes the processing marker for a job
func (q *RedisQueue) MarkComplete(ctx context.Context, jobID string) error {
	key := fmt.Sprintf("job:processing:%s", jobID)

	if err := q.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to mark job as complete: %w", err)
	}

	return nil
}

// GetProcessingJobs retrieves all jobs currently being processed
func (q *RedisQueue) GetProcessingJobs(ctx context.Context) ([]string, error) {
	pattern := "job:processing:*"

	keys, err := q.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get processing jobs: %w", err)
	}

	// Extract job IDs from keys
	jobIDs := make([]string, 0, len(keys))
	for _, key := range keys {
		// Remove "job:processing:" prefix
		jobID := key[len("job:processing:"):]
		jobIDs = append(jobIDs, jobID)
	}

	return jobIDs, nil
}

// GetQueueLength returns the number of jobs in a queue
func (q *RedisQueue) GetQueueLength(ctx context.Context, jobType JobType) (int64, error) {
	queueKey := fmt.Sprintf("queue:%s", jobType)

	length, err := q.client.LLen(ctx, queueKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get queue length: %w", err)
	}

	return length, nil
}

// Close closes the Redis connection
func (q *RedisQueue) Close() error {
	if err := q.client.Close(); err != nil {
		return fmt.Errorf("failed to close redis connection: %w", err)
	}

	log.Info().Msg("Redis queue connection closed")
	return nil
}

// Ping checks if the Redis connection is alive
func (q *RedisQueue) Ping(ctx context.Context) error {
	if err := q.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	return nil
}
