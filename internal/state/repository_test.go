package state

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupTestDB creates a PostgreSQL database connection for testing
func setupTestDB(t *testing.T) *gorm.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=deployer password=test_password dbname=app_deployer_test sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("Skipping test - PostgreSQL not available: %v", err)
	}

	// Run migrations
	err = db.AutoMigrate(&Deployment{}, &Infrastructure{}, &Build{}, &DeploymentLog{})
	require.NoError(t, err, "failed to run migrations")

	t.Cleanup(func() {
		db.Exec("TRUNCATE TABLE deployment_logs, builds, infrastructures, deployments CASCADE")
	})

	return db
}

func TestCreateDeployment(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	deployment := &Deployment{
		Name:    "test-deployment",
		AppName: "test-app",
		Version: "v1.0.0",
		Status:  "PENDING",
		Cloud:   "gcp",
		Region:  "us-central1",
	}

	err := repo.CreateDeployment(ctx, deployment)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, deployment.ID, "ID should be generated")
}

func TestGetDeployment(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create deployment
	deployment := &Deployment{
		ID:      uuid.New(),
		Name:    "test-deployment",
		AppName: "test-app",
		Version: "v1.0.0",
		Status:  "PENDING",
		Cloud:   "gcp",
		Region:  "us-central1",
	}

	err := repo.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Retrieve deployment
	retrieved, err := repo.GetDeployment(ctx, deployment.ID)
	assert.NoError(t, err)
	assert.Equal(t, deployment.ID, retrieved.ID)
	assert.Equal(t, deployment.Name, retrieved.Name)
	assert.Equal(t, deployment.Status, retrieved.Status)
}

func TestGetDeploymentNotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	_, err := repo.GetDeployment(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deployment not found")
}

func TestListDeployments(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create multiple deployments
	for i := 0; i < 5; i++ {
		deployment := &Deployment{
			Name:    "test-deployment",
			AppName: "test-app",
			Version: "v1.0.0",
			Status:  "PENDING",
			Cloud:   "gcp",
			Region:  "us-central1",
		}
		err := repo.CreateDeployment(ctx, deployment)
		require.NoError(t, err)
	}

	// List all deployments
	deployments, err := repo.ListDeployments(ctx, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, deployments, 5)
}

func TestUpdateDeploymentStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create deployment
	deployment := &Deployment{
		Name:    "test-deployment",
		AppName: "test-app",
		Version: "v1.0.0",
		Status:  "PENDING",
		Cloud:   "gcp",
		Region:  "us-central1",
	}
	err := repo.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Update status
	err = repo.UpdateDeploymentStatus(ctx, deployment.ID, "BUILDING")
	assert.NoError(t, err)

	// Verify update
	updated, err := repo.GetDeployment(ctx, deployment.ID)
	assert.NoError(t, err)
	assert.Equal(t, "BUILDING", updated.Status)
}

func TestCreateInfrastructure(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create deployment first
	deployment := &Deployment{
		Name:    "test-deployment",
		AppName: "test-app",
		Version: "v1.0.0",
		Status:  "PROVISIONING",
		Cloud:   "gcp",
		Region:  "us-central1",
	}
	err := repo.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create infrastructure
	infra := &Infrastructure{
		DeploymentID: deployment.ID,
		ClusterName:  "test-cluster",
		Namespace:    "default",
		ServiceName:  "test-service",
		Status:       "PROVISIONING",
		Config:       `{"type":"kubernetes"}`,
	}

	err = repo.CreateInfrastructure(ctx, infra)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, infra.ID)
}

func TestCreateBuild(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create deployment first
	deployment := &Deployment{
		Name:    "test-deployment",
		AppName: "test-app",
		Version: "v1.0.0",
		Status:  "BUILDING",
		Cloud:   "gcp",
		Region:  "us-central1",
	}
	err := repo.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Create build
	build := &Build{
		DeploymentID: deployment.ID,
		ImageTag:     "v1.0.0",
		Status:       "BUILDING",
		BuildLog:     "Building...",
		StartedAt:    time.Now(),
	}

	err = repo.CreateBuild(ctx, build)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, build.ID)
}

func TestMarkDeploymentAsDeployed(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create deployment
	deployment := &Deployment{
		Name:    "test-deployment",
		AppName: "test-app",
		Version: "v1.0.0",
		Status:  "DEPLOYING",
		Cloud:   "gcp",
		Region:  "us-central1",
	}
	err := repo.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	// Mark as deployed
	err = repo.MarkDeploymentAsDeployed(ctx, deployment.ID, "203.0.113.42", "http://example.com")
	assert.NoError(t, err)

	// Verify update
	updated, err := repo.GetDeployment(ctx, deployment.ID)
	assert.NoError(t, err)
	assert.Equal(t, "EXPOSED", updated.Status)
	assert.Equal(t, "203.0.113.42", updated.ExternalIP)
	assert.Equal(t, "http://example.com", updated.ExternalURL)
	assert.NotNil(t, updated.DeployedAt)
}
