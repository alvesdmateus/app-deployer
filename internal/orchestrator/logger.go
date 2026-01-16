package orchestrator

import (
	"context"
	"encoding/json"
	"time"

	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Log levels
const (
	LogLevelDebug = "DEBUG"
	LogLevelInfo  = "INFO"
	LogLevelWarn  = "WARN"
	LogLevelError = "ERROR"
)

// Deployment phases
const (
	PhaseQueued       = "QUEUED"
	PhaseProvisioning = "PROVISIONING"
	PhaseDeploying    = "DEPLOYING"
	PhaseDestroying   = "DESTROYING"
	PhaseRollingBack  = "ROLLING_BACK"
)

// DeploymentLogger writes logs to the database for deployment tracking
type DeploymentLogger struct {
	repo         *state.Repository
	deploymentID uuid.UUID
	jobID        string
	phase        string
	logger       zerolog.Logger // Also log to stdout for debugging
}

// NewDeploymentLogger creates a new deployment logger
func NewDeploymentLogger(repo *state.Repository, deploymentID uuid.UUID, jobID, phase string, logger zerolog.Logger) *DeploymentLogger {
	return &DeploymentLogger{
		repo:         repo,
		deploymentID: deploymentID,
		jobID:        jobID,
		phase:        phase,
		logger:       logger,
	}
}

// LogEntry represents the details that can be attached to a log
type LogEntry struct {
	Level   string
	Message string
	Details map[string]interface{}
}

// write writes a log entry to the database
func (l *DeploymentLogger) write(ctx context.Context, level, message string, details map[string]interface{}) {
	var detailsJSON string
	if details != nil {
		if jsonBytes, err := json.Marshal(details); err == nil {
			detailsJSON = string(jsonBytes)
		}
	}

	logEntry := &state.DeploymentLog{
		DeploymentID: l.deploymentID,
		JobID:        l.jobID,
		Phase:        l.phase,
		Level:        level,
		Message:      message,
		Details:      detailsJSON,
		Timestamp:    time.Now(),
	}

	// Write to database (don't fail the operation if logging fails)
	if err := l.repo.CreateDeploymentLog(ctx, logEntry); err != nil {
		l.logger.Warn().
			Err(err).
			Str("deployment_id", l.deploymentID.String()).
			Str("message", message).
			Msg("Failed to write deployment log to database")
	}
}

// Debug logs a debug message
func (l *DeploymentLogger) Debug(ctx context.Context, message string, details map[string]interface{}) {
	l.write(ctx, LogLevelDebug, message, details)
	l.logger.Debug().
		Str("deployment_id", l.deploymentID.String()).
		Str("phase", l.phase).
		Interface("details", details).
		Msg(message)
}

// Info logs an info message
func (l *DeploymentLogger) Info(ctx context.Context, message string, details map[string]interface{}) {
	l.write(ctx, LogLevelInfo, message, details)
	l.logger.Info().
		Str("deployment_id", l.deploymentID.String()).
		Str("phase", l.phase).
		Interface("details", details).
		Msg(message)
}

// Warn logs a warning message
func (l *DeploymentLogger) Warn(ctx context.Context, message string, details map[string]interface{}) {
	l.write(ctx, LogLevelWarn, message, details)
	l.logger.Warn().
		Str("deployment_id", l.deploymentID.String()).
		Str("phase", l.phase).
		Interface("details", details).
		Msg(message)
}

// Error logs an error message
func (l *DeploymentLogger) Error(ctx context.Context, message string, err error, details map[string]interface{}) {
	if details == nil {
		details = make(map[string]interface{})
	}
	if err != nil {
		details["error"] = err.Error()
	}

	l.write(ctx, LogLevelError, message, details)
	l.logger.Error().
		Err(err).
		Str("deployment_id", l.deploymentID.String()).
		Str("phase", l.phase).
		Interface("details", details).
		Msg(message)
}

// SetPhase updates the current phase
func (l *DeploymentLogger) SetPhase(phase string) {
	l.phase = phase
}

// WithDetails creates a helper for logging with common details
func (l *DeploymentLogger) WithDetails(key string, value interface{}) map[string]interface{} {
	return map[string]interface{}{key: value}
}

// Details is a helper function to create a details map
func Details(pairs ...interface{}) map[string]interface{} {
	details := make(map[string]interface{})
	for i := 0; i+1 < len(pairs); i += 2 {
		if key, ok := pairs[i].(string); ok {
			details[key] = pairs[i+1]
		}
	}
	return details
}
