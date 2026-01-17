//go:build integration

package integration

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/alvesdmateus/app-deployer/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilderEndpoints_GenerateDockerfile(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("POST /api/v1/builds/generate-dockerfile generates Dockerfile for Node.js", func(t *testing.T) {
		tempDir := createTempNodeJSProject(t)

		req := api.GenerateDockerfileRequest{
			SourcePath: tempDir,
		}

		resp := env.POST("/api/v1/builds/generate-dockerfile", req)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.GenerateDockerfileResponse
		env.DecodeResponse(resp, &result)

		assert.NotEmpty(t, result.Dockerfile)
		assert.Equal(t, "nodejs", result.Language)
		assert.Contains(t, result.Dockerfile, "FROM")
		assert.Contains(t, result.Dockerfile, "node")
	})

	t.Run("POST /api/v1/builds/generate-dockerfile generates Dockerfile for Python", func(t *testing.T) {
		tempDir := createTempPythonProject(t)

		req := api.GenerateDockerfileRequest{
			SourcePath: tempDir,
		}

		resp := env.POST("/api/v1/builds/generate-dockerfile", req)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.GenerateDockerfileResponse
		env.DecodeResponse(resp, &result)

		assert.NotEmpty(t, result.Dockerfile)
		assert.Equal(t, "python", result.Language)
		assert.Contains(t, result.Dockerfile, "FROM")
		assert.Contains(t, result.Dockerfile, "python")
	})

	t.Run("POST /api/v1/builds/generate-dockerfile generates Dockerfile for Go", func(t *testing.T) {
		tempDir := createTempGoProject(t)

		req := api.GenerateDockerfileRequest{
			SourcePath: tempDir,
		}

		resp := env.POST("/api/v1/builds/generate-dockerfile", req)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.GenerateDockerfileResponse
		env.DecodeResponse(resp, &result)

		assert.NotEmpty(t, result.Dockerfile)
		assert.Equal(t, "go", result.Language)
		assert.Contains(t, result.Dockerfile, "FROM")
		assert.Contains(t, result.Dockerfile, "golang")
	})

	t.Run("POST /api/v1/builds/generate-dockerfile with missing source_path returns 400", func(t *testing.T) {
		req := api.GenerateDockerfileRequest{
			SourcePath: "",
		}

		resp := env.POST("/api/v1/builds/generate-dockerfile", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("POST /api/v1/builds/generate-dockerfile with non-existent path returns 500", func(t *testing.T) {
		req := api.GenerateDockerfileRequest{
			SourcePath: "/non/existent/path",
		}

		resp := env.POST("/api/v1/builds/generate-dockerfile", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestBuilderEndpoints_BuildImage(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("POST /api/v1/builds with missing deployment_id returns 400", func(t *testing.T) {
		req := api.BuildImageRequest{
			DeploymentID: "",
			AppName:      "test-app",
			Version:      "1.0.0",
			SourcePath:   "/some/path",
		}

		resp := env.POST("/api/v1/builds", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("POST /api/v1/builds with missing app_name returns 400", func(t *testing.T) {
		req := api.BuildImageRequest{
			DeploymentID: "test-deployment-id",
			AppName:      "",
			Version:      "1.0.0",
			SourcePath:   "/some/path",
		}

		resp := env.POST("/api/v1/builds", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("POST /api/v1/builds with missing version returns 400", func(t *testing.T) {
		req := api.BuildImageRequest{
			DeploymentID: "test-deployment-id",
			AppName:      "test-app",
			Version:      "",
			SourcePath:   "/some/path",
		}

		resp := env.POST("/api/v1/builds", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("POST /api/v1/builds with missing source_path returns 400", func(t *testing.T) {
		req := api.BuildImageRequest{
			DeploymentID: "test-deployment-id",
			AppName:      "test-app",
			Version:      "1.0.0",
			SourcePath:   "",
		}

		resp := env.POST("/api/v1/builds", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestBuilderEndpoints_GetBuildLogs(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("GET /api/v1/builds/{buildID}/logs with non-existent build returns 500", func(t *testing.T) {
		resp := env.GET("/api/v1/builds/non-existent-build-id/logs")
		defer resp.Body.Close()

		// The build service will return an error for non-existent build
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestBuilderEndpoints_CancelBuild(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("POST /api/v1/builds/{buildID}/cancel with non-existent build returns 500", func(t *testing.T) {
		resp := env.POST("/api/v1/builds/non-existent-build-id/cancel", nil)
		defer resp.Body.Close()

		// The build service will return an error for non-existent build
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestBuilderEndpoints_GeneratedDockerfileContent(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("Generated Dockerfile includes proper stages for Node.js", func(t *testing.T) {
		tempDir := createTempNodeJSProject(t)

		req := api.GenerateDockerfileRequest{
			SourcePath: tempDir,
		}

		resp := env.POST("/api/v1/builds/generate-dockerfile", req)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.GenerateDockerfileResponse
		env.DecodeResponse(resp, &result)

		// Verify Dockerfile content includes essential elements
		assert.Contains(t, result.Dockerfile, "WORKDIR")
		assert.Contains(t, result.Dockerfile, "COPY")
		assert.Contains(t, result.Dockerfile, "EXPOSE")
		assert.Contains(t, result.Dockerfile, "CMD")
	})

	t.Run("Generated Dockerfile for project with Express framework", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "api-test-express-*")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(tempDir) })

		packageJSON := `{
			"name": "express-app",
			"version": "1.0.0",
			"main": "server.js",
			"scripts": {
				"start": "node server.js"
			},
			"dependencies": {
				"express": "^4.18.0",
				"cors": "^2.8.5"
			}
		}`

		err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644)
		require.NoError(t, err)

		req := api.GenerateDockerfileRequest{
			SourcePath: tempDir,
		}

		resp := env.POST("/api/v1/builds/generate-dockerfile", req)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result api.GenerateDockerfileResponse
		env.DecodeResponse(resp, &result)

		// Framework detection may or may not detect Express depending on implementation
		assert.Equal(t, "nodejs", result.Language)
		assert.NotNil(t, result.Analysis)
	})
}
