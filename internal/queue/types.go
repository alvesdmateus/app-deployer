package queue

import (
	"time"
)

// JobType represents the type of job to be processed
type JobType string

const (
	// JobTypeProvision represents an infrastructure provisioning job
	JobTypeProvision JobType = "provision"

	// JobTypeDeploy represents a Kubernetes deployment job
	JobTypeDeploy JobType = "deploy"

	// JobTypeDestroy represents an infrastructure destroy job
	JobTypeDestroy JobType = "destroy"

	// JobTypeRollback represents a rollback job
	JobTypeRollback JobType = "rollback"
)

// Job represents a work item in the queue
type Job struct {
	ID           string                 `json:"id"`
	Type         JobType                `json:"type"`
	DeploymentID string                 `json:"deployment_id"`
	Payload      map[string]interface{} `json:"payload"`
	CreatedAt    time.Time              `json:"created_at"`
	Attempts     int                    `json:"attempts"`
	MaxAttempts  int                    `json:"max_attempts"`
}

// ProvisionPayload contains data for a provision job
type ProvisionPayload struct {
	DeploymentID string `json:"deployment_id"`
	AppName      string `json:"app_name"`
	Version      string `json:"version"`
	Cloud        string `json:"cloud"`
	Region       string `json:"region"`
	ImageTag     string `json:"image_tag"`
	BuildID      string `json:"build_id"`
}

// DeployPayload contains data for a deploy job
type DeployPayload struct {
	DeploymentID     string `json:"deployment_id"`
	InfrastructureID string `json:"infrastructure_id"`
	ImageTag         string `json:"image_tag"`
	Port             int    `json:"port"`
	Replicas         int    `json:"replicas"`
}

// DestroyPayload contains data for a destroy job
type DestroyPayload struct {
	DeploymentID     string `json:"deployment_id"`
	InfrastructureID string `json:"infrastructure_id"`
}

// RollbackPayload contains data for a rollback job
type RollbackPayload struct {
	DeploymentID     string `json:"deployment_id"`
	InfrastructureID string `json:"infrastructure_id"`
	PreviousVersion  string `json:"previous_version"`
}
