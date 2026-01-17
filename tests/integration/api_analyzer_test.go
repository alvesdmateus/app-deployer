//go:build integration

package integration

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/alvesdmateus/app-deployer/internal/analyzer"
	"github.com/alvesdmateus/app-deployer/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzerEndpoints_AnalyzeSourceCode(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("POST /api/v1/analyze analyzes Node.js project", func(t *testing.T) {
		// Create a temporary Node.js project
		tempDir := createTempNodeJSProject(t)

		req := api.AnalyzeRequest{
			Path: tempDir,
		}

		resp := env.POST("/api/v1/analyze", req)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result analyzer.AnalysisResult
		env.DecodeResponse(resp, &result)

		assert.Equal(t, analyzer.LanguageNodeJS, result.Language)
		assert.NotEmpty(t, result.Dependencies)
		assert.True(t, result.Confidence > 0)
	})

	t.Run("POST /api/v1/analyze analyzes Python project", func(t *testing.T) {
		// Create a temporary Python project
		tempDir := createTempPythonProject(t)

		req := api.AnalyzeRequest{
			Path: tempDir,
		}

		resp := env.POST("/api/v1/analyze", req)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result analyzer.AnalysisResult
		env.DecodeResponse(resp, &result)

		assert.Equal(t, analyzer.LanguagePython, result.Language)
	})

	t.Run("POST /api/v1/analyze analyzes Go project", func(t *testing.T) {
		// Create a temporary Go project
		tempDir := createTempGoProject(t)

		req := api.AnalyzeRequest{
			Path: tempDir,
		}

		resp := env.POST("/api/v1/analyze", req)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result analyzer.AnalysisResult
		env.DecodeResponse(resp, &result)

		assert.Equal(t, analyzer.LanguageGo, result.Language)
		assert.Equal(t, analyzer.BuildToolGo, result.BuildTool)
	})

	t.Run("POST /api/v1/analyze with missing path returns 400", func(t *testing.T) {
		req := api.AnalyzeRequest{
			Path: "",
		}

		resp := env.POST("/api/v1/analyze", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("POST /api/v1/analyze with non-existent path returns 500", func(t *testing.T) {
		req := api.AnalyzeRequest{
			Path: "/non/existent/path",
		}

		resp := env.POST("/api/v1/analyze", req)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("POST /api/v1/analyze with invalid JSON returns 400", func(t *testing.T) {
		resp := env.MakeRequest(http.MethodPost, "/api/v1/analyze", "invalid json")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestAnalyzerEndpoints_GetSupportedLanguages(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("GET /api/v1/analyze/languages returns supported languages", func(t *testing.T) {
		resp := env.GET("/api/v1/analyze/languages")
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		env.DecodeResponse(resp, &result)

		languages, ok := result["languages"].([]interface{})
		require.True(t, ok, "Expected languages array in response")
		require.NotEmpty(t, languages)

		// Check for expected languages
		languageIDs := make([]string, 0)
		for _, lang := range languages {
			langMap, ok := lang.(map[string]interface{})
			if ok {
				if id, ok := langMap["id"].(string); ok {
					languageIDs = append(languageIDs, id)
				}
			}
		}

		assert.Contains(t, languageIDs, "go")
		assert.Contains(t, languageIDs, "nodejs")
		assert.Contains(t, languageIDs, "python")
		assert.Contains(t, languageIDs, "java")
	})
}

func TestAnalyzerEndpoints_DetectsDockerfile(t *testing.T) {
	env := SetupAPITestEnvironment(t)

	t.Run("POST /api/v1/analyze detects Dockerfile", func(t *testing.T) {
		// Create a project with Dockerfile
		tempDir := createTempNodeJSProjectWithDockerfile(t)

		req := api.AnalyzeRequest{
			Path: tempDir,
		}

		resp := env.POST("/api/v1/analyze", req)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result analyzer.AnalysisResult
		env.DecodeResponse(resp, &result)

		assert.True(t, result.HasDockerfile)
	})
}

// Helper functions to create temporary projects

func createTempNodeJSProject(t *testing.T) string {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "api-test-nodejs-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	packageJSON := `{
		"name": "test-app",
		"version": "1.0.0",
		"main": "index.js",
		"scripts": {
			"start": "node index.js"
		},
		"dependencies": {
			"express": "^4.18.0"
		}
	}`

	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	indexJS := `const express = require('express');
const app = express();
app.get('/', (req, res) => res.send('Hello'));
app.listen(3000);`

	err = os.WriteFile(filepath.Join(tempDir, "index.js"), []byte(indexJS), 0644)
	require.NoError(t, err)

	return tempDir
}

func createTempPythonProject(t *testing.T) string {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "api-test-python-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	requirements := `flask==2.3.0
gunicorn==21.0.0`

	err = os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte(requirements), 0644)
	require.NoError(t, err)

	appPy := `from flask import Flask
app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello, World!'

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)`

	err = os.WriteFile(filepath.Join(tempDir, "app.py"), []byte(appPy), 0644)
	require.NoError(t, err)

	return tempDir
}

func createTempGoProject(t *testing.T) string {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "api-test-go-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	goMod := `module example.com/test-app

go 1.21

require github.com/gin-gonic/gin v1.9.0`

	err = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0644)
	require.NoError(t, err)

	mainGo := `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}`

	err = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(mainGo), 0644)
	require.NoError(t, err)

	return tempDir
}

func createTempNodeJSProjectWithDockerfile(t *testing.T) string {
	t.Helper()

	tempDir := createTempNodeJSProject(t)

	dockerfile := `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 3000
CMD ["node", "index.js"]`

	err := os.WriteFile(filepath.Join(tempDir, "Dockerfile"), []byte(dockerfile), 0644)
	require.NoError(t, err)

	return tempDir
}
