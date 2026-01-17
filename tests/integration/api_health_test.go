//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthEndpoints(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("GET /health returns ok status", func(t *testing.T) {
		resp := env.GET("/health")
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		env.DecodeResponse(resp, &result)

		assert.Equal(t, "ok", result["status"])
		assert.Equal(t, "1.0.0", result["version"])
	})

	t.Run("GET /health/live returns alive status", func(t *testing.T) {
		resp := env.GET("/health/live")
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]string
		env.DecodeResponse(resp, &result)

		assert.Equal(t, "alive", result["status"])
	})

	t.Run("GET /health/ready returns ready status", func(t *testing.T) {
		resp := env.GET("/health/ready")
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]string
		env.DecodeResponse(resp, &result)

		assert.Equal(t, "ready", result["status"])
		assert.Equal(t, "healthy", result["database"])
	})
}
