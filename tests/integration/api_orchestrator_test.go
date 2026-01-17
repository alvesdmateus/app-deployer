//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/alvesdmateus/app-deployer/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestOrchestratorEndpoints_GetQueueStats(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("GET /api/v1/orchestrator/stats returns queue statistics", func(t *testing.T) {
		resp := env.GET("/api/v1/orchestrator/stats")
		defer resp.Body.Close()

		// Note: This endpoint requires Redis to be configured
		// In our test environment, Redis may not be available,
		// so we expect either success or a graceful error

		if resp.StatusCode == http.StatusOK {
			var result api.QueueStatsResponse
			env.DecodeResponse(resp, &result)

			// Verify response structure
			assert.GreaterOrEqual(t, result.Provision, int64(0))
			assert.GreaterOrEqual(t, result.Deploy, int64(0))
			assert.GreaterOrEqual(t, result.Destroy, int64(0))
			assert.GreaterOrEqual(t, result.Rollback, int64(0))
		} else {
			// Redis not configured - expect 500 or appropriate error
			assert.True(t, resp.StatusCode == http.StatusInternalServerError ||
				resp.StatusCode == http.StatusServiceUnavailable,
				"Expected error status when Redis is not configured")
		}
	})
}

func TestOrchestratorEndpoints_StartDeployment(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("POST /api/v1/deployments/{id}/deploy with missing image_tag returns 400", func(t *testing.T) {
		// Create a test deployment first
		deployment := env.CreateTestDeployment("deploy-test", "pending")

		req := api.StartDeploymentRequest{
			ImageTag: "",
		}

		resp := env.POST("/api/v1/deployments/"+deployment.ID.String()+"/deploy", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("POST /api/v1/deployments/{id}/deploy with non-existent deployment returns 404", func(t *testing.T) {
		req := api.StartDeploymentRequest{
			ImageTag: "test-image:latest",
		}

		resp := env.POST("/api/v1/deployments/00000000-0000-0000-0000-000000000000/deploy", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("POST /api/v1/deployments/{id}/deploy with invalid deployment ID returns 400", func(t *testing.T) {
		req := api.StartDeploymentRequest{
			ImageTag: "test-image:latest",
		}

		resp := env.POST("/api/v1/deployments/invalid-uuid/deploy", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestOrchestratorEndpoints_TriggerRollback(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("POST /api/v1/deployments/{id}/rollback with missing target_version returns 400", func(t *testing.T) {
		deployment := env.CreateTestDeployment("rollback-test", "healthy")

		req := api.TriggerRollbackRequest{
			TargetVersion: "",
		}

		resp := env.POST("/api/v1/deployments/"+deployment.ID.String()+"/rollback", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("POST /api/v1/deployments/{id}/rollback with non-existent deployment returns 404", func(t *testing.T) {
		req := api.TriggerRollbackRequest{
			TargetVersion: "1.0.0",
		}

		resp := env.POST("/api/v1/deployments/00000000-0000-0000-0000-000000000000/rollback", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("POST /api/v1/deployments/{id}/rollback with invalid deployment ID returns 400", func(t *testing.T) {
		req := api.TriggerRollbackRequest{
			TargetVersion: "1.0.0",
		}

		resp := env.POST("/api/v1/deployments/invalid-uuid/rollback", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
