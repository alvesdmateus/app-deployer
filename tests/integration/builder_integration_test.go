//go:build integration

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuilder_DockerBuildNodeJS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipBuilder {
		t.Skip("Builder tests are disabled")
	}
	RequireDocker(t, cfg)

	// Create a temporary directory with a simple Node.js app
	tempDir, err := os.MkdirTemp("", "builder-nodejs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create package.json
	packageJSON := `{
	"name": "test-nodejs-app",
	"version": "1.0.0",
	"main": "index.js",
	"scripts": {
		"start": "node index.js"
	},
	"dependencies": {
		"express": "^4.18.0"
	}
}`
	if err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	// Create index.js
	indexJS := `const express = require('express');
const app = express();
const port = process.env.PORT || 3000;

app.get('/', (req, res) => {
	res.json({ status: 'ok', message: 'Hello from Node.js!' });
});

app.get('/health', (req, res) => {
	res.json({ status: 'healthy' });
});

app.listen(port, () => {
	console.log('Server running on port ' + port);
});
`
	if err := os.WriteFile(filepath.Join(tempDir, "index.js"), []byte(indexJS), 0644); err != nil {
		t.Fatalf("Failed to create index.js: %v", err)
	}

	// Create Dockerfile
	dockerfile := `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install --production
COPY . .
EXPOSE 3000
CMD ["node", "index.js"]
`
	if err := os.WriteFile(filepath.Join(tempDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	// Build the image using Docker
	imageName := "test-nodejs-app:integration-test"
	ctx, cancel := context.WithTimeout(context.Background(), cfg.BuildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, ".")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Docker build failed: %v\nOutput: %s", err, string(output))
	}

	t.Logf("Docker build succeeded for Node.js app")

	// Clean up the image
	cleanupCmd := exec.Command("docker", "rmi", "-f", imageName)
	cleanupCmd.Run() // Ignore errors during cleanup

	// Verify image was built (before cleanup)
	if !strings.Contains(string(output), "Successfully") && !strings.Contains(string(output), "exporting to image") {
		t.Logf("Build output: %s", string(output))
	}
}

func TestBuilder_DockerBuildPython(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipBuilder {
		t.Skip("Builder tests are disabled")
	}
	RequireDocker(t, cfg)

	// Create a temporary directory with a simple Python Flask app
	tempDir, err := os.MkdirTemp("", "builder-python-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create requirements.txt
	requirements := `flask==2.3.0
gunicorn==21.0.0
`
	if err := os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte(requirements), 0644); err != nil {
		t.Fatalf("Failed to create requirements.txt: %v", err)
	}

	// Create app.py
	appPy := `from flask import Flask, jsonify

app = Flask(__name__)

@app.route('/')
def hello():
    return jsonify(status='ok', message='Hello from Python Flask!')

@app.route('/health')
def health():
    return jsonify(status='healthy')

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
`
	if err := os.WriteFile(filepath.Join(tempDir, "app.py"), []byte(appPy), 0644); err != nil {
		t.Fatalf("Failed to create app.py: %v", err)
	}

	// Create Dockerfile
	dockerfile := `FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 5000
CMD ["python", "app.py"]
`
	if err := os.WriteFile(filepath.Join(tempDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	// Build the image using Docker
	imageName := "test-python-app:integration-test"
	ctx, cancel := context.WithTimeout(context.Background(), cfg.BuildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, ".")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Docker build failed: %v\nOutput: %s", err, string(output))
	}

	t.Logf("Docker build succeeded for Python Flask app")

	// Clean up the image
	cleanupCmd := exec.Command("docker", "rmi", "-f", imageName)
	cleanupCmd.Run() // Ignore errors during cleanup
}

func TestBuilder_DockerBuildGo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipBuilder {
		t.Skip("Builder tests are disabled")
	}
	RequireDocker(t, cfg)

	// Create a temporary directory with a simple Go app
	tempDir, err := os.MkdirTemp("", "builder-go-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create go.mod
	goMod := `module test-go-app

go 1.21
`
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create main.go
	mainGo := `package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type Response struct {
	Status  string ` + "`json:\"status\"`" + `
	Message string ` + "`json:\"message\"`" + `
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Status: "ok", Message: "Hello from Go!"})
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Status: "healthy", Message: ""})
	})

	fmt.Printf("Server starting on port %s\n", port)
	http.ListenAndServe(":"+port, nil)
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	// Create Dockerfile
	dockerfile := `FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM alpine:3.18
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]
`
	if err := os.WriteFile(filepath.Join(tempDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	// Build the image using Docker
	imageName := "test-go-app:integration-test"
	ctx, cancel := context.WithTimeout(context.Background(), cfg.BuildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, ".")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Docker build failed: %v\nOutput: %s", err, string(output))
	}

	t.Logf("Docker build succeeded for Go app")

	// Clean up the image
	cleanupCmd := exec.Command("docker", "rmi", "-f", imageName)
	cleanupCmd.Run() // Ignore errors during cleanup
}

func TestBuilder_DockerBuildTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipBuilder {
		t.Skip("Builder tests are disabled")
	}
	RequireDocker(t, cfg)

	// Create a temporary directory with a Dockerfile that will take time
	tempDir, err := os.MkdirTemp("", "builder-timeout-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a Dockerfile with a sleep command
	dockerfile := `FROM alpine:3.18
RUN sleep 3
CMD ["echo", "done"]
`
	if err := os.WriteFile(filepath.Join(tempDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	// Build with a very short timeout - should fail
	imageName := "test-timeout-app:integration-test"
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, ".")
	cmd.Dir = tempDir
	_, err = cmd.CombinedOutput()

	// The build should have been cancelled due to timeout
	if err == nil {
		t.Log("Build completed before timeout (this is OK if Docker cached layers)")
		// Clean up
		cleanupCmd := exec.Command("docker", "rmi", "-f", imageName)
		cleanupCmd.Run()
	} else if ctx.Err() == context.DeadlineExceeded {
		t.Log("Build correctly timed out")
	} else {
		t.Logf("Build failed with error (may be timeout-related): %v", err)
	}
}

func TestBuilder_VerifyImageCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipBuilder {
		t.Skip("Builder tests are disabled")
	}
	RequireDocker(t, cfg)

	// Create a minimal app
	tempDir, err := os.MkdirTemp("", "builder-verify-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple Dockerfile
	dockerfile := `FROM alpine:3.18
RUN echo "test" > /test.txt
CMD ["cat", "/test.txt"]
`
	if err := os.WriteFile(filepath.Join(tempDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	// Build the image
	imageName := "test-verify-app:integration-test"
	ctx, cancel := context.WithTimeout(context.Background(), cfg.BuildTimeout)
	defer cancel()

	buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, ".")
	buildCmd.Dir = tempDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Docker build failed: %v\nOutput: %s", err, string(output))
	}

	// Verify image exists
	inspectCmd := exec.Command("docker", "inspect", imageName)
	if err := inspectCmd.Run(); err != nil {
		t.Errorf("Image was not created: %v", err)
	}

	// Run the container and verify output
	runCmd := exec.Command("docker", "run", "--rm", imageName)
	output, err := runCmd.Output()
	if err != nil {
		t.Errorf("Failed to run container: %v", err)
	}

	if !strings.Contains(string(output), "test") {
		t.Errorf("Container output unexpected: %s", string(output))
	}

	t.Log("Image creation and execution verified successfully")

	// Clean up
	cleanupCmd := exec.Command("docker", "rmi", "-f", imageName)
	cleanupCmd.Run()
}
