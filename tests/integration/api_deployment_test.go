//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/alvesdmateus/app-deployer/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentEndpoints_Create(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("POST /api/v1/deployments creates a new deployment", func(t *testing.T) {
		req := api.CreateDeploymentRequest{
			Name:    "test-deployment",
			AppName: "my-app",
			Version: "1.0.0",
			Cloud:   "gcp",
			Region:  "us-central1",
		}

		resp := env.POST("/api/v1/deployments", req)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var result api.DeploymentResponse
		env.DecodeResponse(resp, &result)

		assert.NotEmpty(t, result.ID)
		assert.Equal(t, "test-deployment", result.Name)
		assert.Equal(t, "my-app", result.AppName)
		assert.Equal(t, "1.0.0", result.Version)
		assert.Equal(t, "gcp", result.Cloud)
		assert.Equal(t, "us-central1", result.Region)
		assert.Equal(t, "PENDING", result.Status)
	})

	t.Run("POST /api/v1/deployments with invalid request returns 400", func(t *testing.T) {
		// Missing required fields
		req := map[string]string{
			"name": "test",
		}

		resp := env.POST("/api/v1/deployments", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("POST /api/v1/deployments accepts any cloud value", func(t *testing.T) {
		// Note: API currently doesn't validate cloud provider values
		req := api.CreateDeploymentRequest{
			Name:    "test-deployment-custom-cloud",
			AppName: "my-app",
			Version: "1.0.0",
			Cloud:   "custom-cloud",
			Region:  "us-central1",
		}

		resp := env.POST("/api/v1/deployments", req)
		defer resp.Body.Close()

		// API accepts any cloud value - validation would need to be added
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})
}

func TestDeploymentEndpoints_Get(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("GET /api/v1/deployments/{id} returns deployment", func(t *testing.T) {
		// Create a test deployment
		deployment := env.CreateTestDeployment("get-test", "PENDING")

		resp := env.GET(fmt.Sprintf("/api/v1/deployments/%s", deployment.ID))
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.DeploymentResponse
		env.DecodeResponse(resp, &result)

		assert.Equal(t, deployment.ID, result.ID)
		assert.Equal(t, "get-test", result.Name)
	})

	t.Run("GET /api/v1/deployments/{id} with invalid ID returns 400", func(t *testing.T) {
		resp := env.GET("/api/v1/deployments/invalid-uuid")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("GET /api/v1/deployments/{id} with non-existent ID returns 404", func(t *testing.T) {
		resp := env.GET("/api/v1/deployments/00000000-0000-0000-0000-000000000000")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestDeploymentEndpoints_List(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("GET /api/v1/deployments returns empty list initially", func(t *testing.T) {
		resp := env.GET("/api/v1/deployments")
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.ListDeploymentsResponse
		env.DecodeResponse(resp, &result)

		assert.Empty(t, result.Deployments)
		assert.Equal(t, 0, result.Total)
	})

	t.Run("GET /api/v1/deployments returns all deployments", func(t *testing.T) {
		// Create test deployments
		env.CreateTestDeployment("list-test-1", "PENDING")
		env.CreateTestDeployment("list-test-2", "HEALTHY")
		env.CreateTestDeployment("list-test-3", "FAILED")

		resp := env.GET("/api/v1/deployments")
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.ListDeploymentsResponse
		env.DecodeResponse(resp, &result)

		assert.Len(t, result.Deployments, 3)
		assert.Equal(t, 3, result.Total)
	})

	t.Run("GET /api/v1/deployments supports pagination", func(t *testing.T) {
		resp := env.GET("/api/v1/deployments?limit=2&offset=0")
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.ListDeploymentsResponse
		env.DecodeResponse(resp, &result)

		assert.LessOrEqual(t, len(result.Deployments), 2)
	})
}

func TestDeploymentEndpoints_ListByStatus(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	// Create deployments with different statuses
	env.CreateTestDeployment("PENDING-1", "PENDING")
	env.CreateTestDeployment("PENDING-2", "PENDING")
	env.CreateTestDeployment("HEALTHY-1", "HEALTHY")

	t.Run("GET /api/v1/deployments/status/{status} filters by status", func(t *testing.T) {
		resp := env.GET("/api/v1/deployments/status/PENDING")
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		// API returns array of deployments directly
		var result []api.DeploymentResponse
		env.DecodeResponse(resp, &result)

		assert.Len(t, result, 2)
		for _, d := range result {
			assert.Equal(t, "PENDING", d.Status)
		}
	})

	t.Run("GET /api/v1/deployments/status/{status} returns empty for no matches", func(t *testing.T) {
		resp := env.GET("/api/v1/deployments/status/FAILED")
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		// API returns array of deployments directly
		var result []api.DeploymentResponse
		env.DecodeResponse(resp, &result)

		assert.Empty(t, result)
	})
}

func TestDeploymentEndpoints_UpdateStatus(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("PATCH /api/v1/deployments/{id}/status updates status", func(t *testing.T) {
		deployment := env.CreateTestDeployment("status-test", "PENDING")

		req := api.UpdateDeploymentStatusRequest{
			Status: "QUEUED",
		}

		resp := env.PATCH(fmt.Sprintf("/api/v1/deployments/%s/status", deployment.ID), req)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		// API returns SuccessResponse, not DeploymentResponse
		var result api.SuccessResponse
		env.DecodeResponse(resp, &result)

		assert.Equal(t, "Deployment status updated", result.Message)

		// Verify status was actually updated by fetching deployment
		getResp := env.GET(fmt.Sprintf("/api/v1/deployments/%s", deployment.ID))
		defer getResp.Body.Close()

		var updatedDeployment api.DeploymentResponse
		env.DecodeResponse(getResp, &updatedDeployment)
		assert.Equal(t, "QUEUED", updatedDeployment.Status)
	})

	t.Run("PATCH /api/v1/deployments/{id}/status accepts any status value", func(t *testing.T) {
		// Note: API currently doesn't validate status values
		deployment := env.CreateTestDeployment("any-status-test", "PENDING")

		req := api.UpdateDeploymentStatusRequest{
			Status: "CUSTOM_STATUS",
		}

		resp := env.PATCH(fmt.Sprintf("/api/v1/deployments/%s/status", deployment.ID), req)
		defer resp.Body.Close()

		// API accepts any status value - validation would need to be added
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("PATCH /api/v1/deployments/{id}/status with empty status returns 400", func(t *testing.T) {
		deployment := env.CreateTestDeployment("empty-status-test", "PENDING")

		req := api.UpdateDeploymentStatusRequest{
			Status: "",
		}

		resp := env.PATCH(fmt.Sprintf("/api/v1/deployments/%s/status", deployment.ID), req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestDeploymentEndpoints_Delete(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("DELETE /api/v1/deployments/{id} deletes deployment", func(t *testing.T) {
		deployment := env.CreateTestDeployment("delete-test", "PENDING")

		resp := env.DELETE(fmt.Sprintf("/api/v1/deployments/%s", deployment.ID))
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify deletion
		getResp := env.GET(fmt.Sprintf("/api/v1/deployments/%s", deployment.ID))
		defer getResp.Body.Close()

		assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
	})

	t.Run("DELETE /api/v1/deployments/{id} with non-existent ID returns 404", func(t *testing.T) {
		resp := env.DELETE("/api/v1/deployments/00000000-0000-0000-0000-000000000000")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestDeploymentEndpoints_Infrastructure(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("GET /api/v1/deployments/{id}/infrastructure returns infrastructure", func(t *testing.T) {
		deployment := env.CreateTestDeployment("infra-test", "HEALTHY")
		infra := env.CreateTestInfrastructure(deployment.ID, "READY")

		resp := env.GET(fmt.Sprintf("/api/v1/deployments/%s/infrastructure", deployment.ID))
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.InfrastructureResponse
		env.DecodeResponse(resp, &result)

		assert.Equal(t, infra.ID, result.ID)
		assert.Equal(t, deployment.ID, result.DeploymentID)
		assert.Equal(t, "test-cluster", result.ClusterName)
	})

	t.Run("GET /api/v1/deployments/{id}/infrastructure returns 404 when no infrastructure", func(t *testing.T) {
		deployment := env.CreateTestDeployment("no-infra-test", "PENDING")

		resp := env.GET(fmt.Sprintf("/api/v1/deployments/%s/infrastructure", deployment.ID))
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestDeploymentEndpoints_Builds(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("GET /api/v1/deployments/{id}/builds/latest returns latest build", func(t *testing.T) {
		deployment := env.CreateTestDeployment("build-test", "HEALTHY")
		build := env.CreateTestBuild(deployment.ID, "SUCCESS")

		resp := env.GET(fmt.Sprintf("/api/v1/deployments/%s/builds/latest", deployment.ID))
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.BuildResponse
		env.DecodeResponse(resp, &result)

		assert.Equal(t, build.ID, result.ID)
		assert.Equal(t, "test-image:latest", result.ImageTag)
	})

	t.Run("GET /api/v1/deployments/{id}/builds/latest returns 404 when no builds", func(t *testing.T) {
		deployment := env.CreateTestDeployment("no-build-test", "PENDING")

		resp := env.GET(fmt.Sprintf("/api/v1/deployments/%s/builds/latest", deployment.ID))
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestDeploymentEndpoints_Logs(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("GET /api/v1/deployments/{id}/logs returns empty list initially", func(t *testing.T) {
		deployment := env.CreateTestDeployment("logs-test", "PENDING")

		resp := env.GET(fmt.Sprintf("/api/v1/deployments/%s/logs", deployment.ID))
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.ListDeploymentLogsResponse
		env.DecodeResponse(resp, &result)

		assert.Empty(t, result.Logs)
	})

	t.Run("GET /api/v1/deployments/{id}/logs supports phase filter", func(t *testing.T) {
		deployment := env.CreateTestDeployment("logs-phase-test", "PENDING")

		resp := env.GET(fmt.Sprintf("/api/v1/deployments/%s/logs?phase=provisioning", deployment.ID))
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
