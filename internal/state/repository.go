package state

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository provides database operations for deployments
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new state repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// CreateDeployment creates a new deployment record
func (r *Repository) CreateDeployment(ctx context.Context, deployment *Deployment) error {
	if deployment.ID == uuid.Nil {
		deployment.ID = uuid.New()
	}

	if err := r.db.WithContext(ctx).Create(deployment).Error; err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	return nil
}

// GetDeployment retrieves a deployment by ID
func (r *Repository) GetDeployment(ctx context.Context, id uuid.UUID) (*Deployment, error) {
	var deployment Deployment

	if err := r.db.WithContext(ctx).
		Preload("Infrastructure").
		Preload("Builds").
		First(&deployment, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("deployment not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	return &deployment, nil
}

// ListDeployments retrieves all deployments with optional filters
func (r *Repository) ListDeployments(ctx context.Context, limit, offset int) ([]Deployment, error) {
	var deployments []Deployment

	query := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset)

	if err := query.Find(&deployments).Error; err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	return deployments, nil
}

// UpdateDeployment updates a deployment record
func (r *Repository) UpdateDeployment(ctx context.Context, deployment *Deployment) error {
	if err := r.db.WithContext(ctx).Save(deployment).Error; err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	return nil
}

// UpdateDeploymentStatus updates only the status of a deployment
func (r *Repository) UpdateDeploymentStatus(ctx context.Context, id uuid.UUID, status string) error {
	if err := r.db.WithContext(ctx).
		Model(&Deployment{}).
		Where("id = ?", id).
		Update("status", status).Error; err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	return nil
}

// DeleteDeployment deletes a deployment and related records
func (r *Repository) DeleteDeployment(ctx context.Context, id uuid.UUID) error {
	// Delete related infrastructure and builds (cascade)
	if err := r.db.WithContext(ctx).
		Where("deployment_id = ?", id).
		Delete(&Infrastructure{}).Error; err != nil {
		return fmt.Errorf("failed to delete infrastructure: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Where("deployment_id = ?", id).
		Delete(&Build{}).Error; err != nil {
		return fmt.Errorf("failed to delete builds: %w", err)
	}

	// Delete deployment
	if err := r.db.WithContext(ctx).Delete(&Deployment{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

// CreateInfrastructure creates an infrastructure record
func (r *Repository) CreateInfrastructure(ctx context.Context, infra *Infrastructure) error {
	if infra.ID == uuid.Nil {
		infra.ID = uuid.New()
	}

	if err := r.db.WithContext(ctx).Create(infra).Error; err != nil {
		return fmt.Errorf("failed to create infrastructure: %w", err)
	}

	return nil
}

// GetInfrastructure retrieves infrastructure by deployment ID
func (r *Repository) GetInfrastructure(ctx context.Context, deploymentID uuid.UUID) (*Infrastructure, error) {
	var infra Infrastructure

	if err := r.db.WithContext(ctx).
		First(&infra, "deployment_id = ?", deploymentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("infrastructure not found for deployment: %s", deploymentID)
		}
		return nil, fmt.Errorf("failed to get infrastructure: %w", err)
	}

	return &infra, nil
}

// CreateBuild creates a build record
func (r *Repository) CreateBuild(ctx context.Context, build *Build) error {
	if build.ID == uuid.Nil {
		build.ID = uuid.New()
	}

	if err := r.db.WithContext(ctx).Create(build).Error; err != nil {
		return fmt.Errorf("failed to create build: %w", err)
	}

	return nil
}

// UpdateBuild updates a build record
func (r *Repository) UpdateBuild(ctx context.Context, build *Build) error {
	if err := r.db.WithContext(ctx).Save(build).Error; err != nil {
		return fmt.Errorf("failed to update build: %w", err)
	}

	return nil
}

// GetLatestBuild retrieves the most recent build for a deployment
func (r *Repository) GetLatestBuild(ctx context.Context, deploymentID uuid.UUID) (*Build, error) {
	var build Build

	if err := r.db.WithContext(ctx).
		Where("deployment_id = ?", deploymentID).
		Order("started_at DESC").
		First(&build).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no builds found for deployment: %s", deploymentID)
		}
		return nil, fmt.Errorf("failed to get latest build: %w", err)
	}

	return &build, nil
}

// GetDeploymentsByStatus retrieves deployments by status
func (r *Repository) GetDeploymentsByStatus(ctx context.Context, status string) ([]Deployment, error) {
	var deployments []Deployment

	if err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Order("created_at DESC").
		Find(&deployments).Error; err != nil {
		return nil, fmt.Errorf("failed to get deployments by status: %w", err)
	}

	return deployments, nil
}

// CountDeploymentsByStatus counts deployments by status
func (r *Repository) CountDeploymentsByStatus(ctx context.Context, status string) (int64, error) {
	var count int64

	if err := r.db.WithContext(ctx).
		Model(&Deployment{}).
		Where("status = ?", status).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count deployments: %w", err)
	}

	return count, nil
}

// MarkDeploymentAsDeployed marks a deployment as deployed and sets the timestamp
func (r *Repository) MarkDeploymentAsDeployed(ctx context.Context, id uuid.UUID, externalIP, externalURL string) error {
	now := time.Now()

	if err := r.db.WithContext(ctx).
		Model(&Deployment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       "EXPOSED",
			"external_ip":  externalIP,
			"external_url": externalURL,
			"deployed_at":  now,
		}).Error; err != nil {
		return fmt.Errorf("failed to mark deployment as deployed: %w", err)
	}

	return nil
}

// GetRecentDeployments retrieves the most recent N deployments
func (r *Repository) GetRecentDeployments(ctx context.Context, limit int) ([]Deployment, error) {
	var deployments []Deployment

	if err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Find(&deployments).Error; err != nil {
		return nil, fmt.Errorf("failed to get recent deployments: %w", err)
	}

	return deployments, nil
}
