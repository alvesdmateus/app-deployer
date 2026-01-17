//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alvesdmateus/app-deployer/internal/api"
	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// APITestEnvironment holds test server and database for API integration tests
type APITestEnvironment struct {
	Server   *httptest.Server
	DB       *gorm.DB
	Repo     *state.Repository
	t        *testing.T
	cleanups []func()
}

// SetupAPITestEnvironment creates a new test environment with in-memory SQLite database
func SetupAPITestEnvironment(t *testing.T) *APITestEnvironment {
	t.Helper()

	// Create in-memory SQLite database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations
	err = db.AutoMigrate(
		&state.Deployment{},
		&state.Infrastructure{},
		&state.Build{},
		&state.DeploymentLog{},
	)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create repository
	repo := state.NewRepository(db)

	// Create API server
	server := api.NewServer(db)

	// Create test HTTP server
	testServer := httptest.NewServer(server.Handler())

	env := &APITestEnvironment{
		Server: testServer,
		DB:     db,
		Repo:   repo,
		t:      t,
	}

	t.Cleanup(func() {
		testServer.Close()
		for _, cleanup := range env.cleanups {
			cleanup()
		}
	})

	return env
}

// AddCleanup adds a cleanup function to be called when test completes
func (e *APITestEnvironment) AddCleanup(fn func()) {
	e.cleanups = append(e.cleanups, fn)
}

// MakeRequest makes an HTTP request to the test server
func (e *APITestEnvironment) MakeRequest(method, path string, body interface{}) *http.Response {
	e.t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("Failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, e.Server.URL+path, reqBody)
	if err != nil {
		e.t.Fatalf("Failed to create request: %v", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("Failed to make request: %v", err)
	}

	return resp
}

// GET makes a GET request
func (e *APITestEnvironment) GET(path string) *http.Response {
	return e.MakeRequest(http.MethodGet, path, nil)
}

// POST makes a POST request
func (e *APITestEnvironment) POST(path string, body interface{}) *http.Response {
	return e.MakeRequest(http.MethodPost, path, body)
}

// PATCH makes a PATCH request
func (e *APITestEnvironment) PATCH(path string, body interface{}) *http.Response {
	return e.MakeRequest(http.MethodPatch, path, body)
}

// DELETE makes a DELETE request
func (e *APITestEnvironment) DELETE(path string) *http.Response {
	return e.MakeRequest(http.MethodDelete, path, nil)
}

// DecodeResponse decodes JSON response body into the provided struct
func (e *APITestEnvironment) DecodeResponse(resp *http.Response, v interface{}) {
	e.t.Helper()
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		e.t.Fatalf("Failed to decode response: %v", err)
	}
}

// CreateTestDeployment creates a deployment directly in the database for testing
func (e *APITestEnvironment) CreateTestDeployment(name string, status string) *state.Deployment {
	e.t.Helper()

	deployment := &state.Deployment{
		ID:      uuid.New(),
		Name:    name,
		AppName: "test-app",
		Version: "1.0.0",
		Status:  status,
		Cloud:   "gcp",
		Region:  "us-central1",
		Port:    8080,
	}

	if err := e.DB.Create(deployment).Error; err != nil {
		e.t.Fatalf("Failed to create test deployment: %v", err)
	}

	return deployment
}

// CreateTestInfrastructure creates infrastructure for a deployment
func (e *APITestEnvironment) CreateTestInfrastructure(deploymentID uuid.UUID, status string) *state.Infrastructure {
	e.t.Helper()

	infra := &state.Infrastructure{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ClusterName:  "test-cluster",
		Namespace:    "test-namespace",
		ServiceName:  "test-service",
		Status:       status,
	}

	if err := e.DB.Create(infra).Error; err != nil {
		e.t.Fatalf("Failed to create test infrastructure: %v", err)
	}

	return infra
}

// CreateTestBuild creates a build for a deployment
func (e *APITestEnvironment) CreateTestBuild(deploymentID uuid.UUID, status string) *state.Build {
	e.t.Helper()

	build := &state.Build{
		ID:           uuid.New(),
		DeploymentID: deploymentID,
		ImageTag:     "test-image:latest",
		Status:       status,
	}

	if err := e.DB.Create(build).Error; err != nil {
		e.t.Fatalf("Failed to create test build: %v", err)
	}

	return build
}
