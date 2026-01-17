//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// IntegrationConfig holds configuration for integration tests
type IntegrationConfig struct {
	// Skip flags for specific test categories
	SkipAnalyzer bool
	SkipBuilder  bool
	SkipDeployer bool

	// Docker settings
	DockerAvailable bool

	// Kind cluster settings
	KindClusterName string
	KindAvailable   bool

	// Timeouts
	CloneTimeout time.Duration
	BuildTimeout time.Duration
	DeployTimeout time.Duration
}

// LoadIntegrationConfig loads configuration from environment variables
func LoadIntegrationConfig() *IntegrationConfig {
	cfg := &IntegrationConfig{
		SkipAnalyzer:    os.Getenv("SKIP_ANALYZER_TESTS") == "true",
		SkipBuilder:     os.Getenv("SKIP_BUILDER_TESTS") == "true",
		SkipDeployer:    os.Getenv("SKIP_DEPLOYER_TESTS") == "true",
		KindClusterName: getEnvOrDefault("KIND_CLUSTER_NAME", "deployer-test"),
		CloneTimeout:    2 * time.Minute,
		BuildTimeout:    10 * time.Minute,
		DeployTimeout:   5 * time.Minute,
	}

	// Check if Docker is available
	cfg.DockerAvailable = checkDockerAvailable()

	// Check if kind cluster is available
	cfg.KindAvailable = checkKindAvailable(cfg.KindClusterName)

	return cfg
}

// checkDockerAvailable checks if Docker daemon is running
func checkDockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	err := cmd.Run()
	return err == nil
}

// checkKindAvailable checks if a kind cluster exists
func checkKindAvailable(clusterName string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kind", "get", "clusters")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if cluster name is in the output
	return containsLine(string(output), clusterName)
}

// containsLine checks if a string contains a specific line
func containsLine(s, line string) bool {
	for _, l := range splitLines(s) {
		if l == line {
			return true
		}
	}
	return false
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		line := s[start:]
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// CloneTestRepo clones a public repository for testing
func CloneTestRepo(t *testing.T, repoURL, branch string) string {
	t.Helper()

	if branch == "" {
		branch = "main"
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cloneOpts := &git.CloneOptions{
		URL:           repoURL,
		Depth:         1,
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
	}

	_, err = git.PlainCloneContext(ctx, tempDir, false, cloneOpts)
	if err != nil {
		t.Fatalf("Failed to clone repository %s: %v", repoURL, err)
	}

	return tempDir
}

// TestRepos contains URLs of public repositories for testing
var TestRepos = struct {
	NodeJSExpress string
	PythonFlask   string
	GoFiber       string
	NodeJSSimple  string
}{
	// Using small, well-known public repos for testing
	NodeJSExpress: "https://github.com/expressjs/express.git",
	PythonFlask:   "https://github.com/pallets/flask.git",
	GoFiber:       "https://github.com/gofiber/fiber.git",
	NodeJSSimple:  "https://github.com/nodejs/node-addon-examples.git",
}

// RequireDocker skips the test if Docker is not available
func RequireDocker(t *testing.T, cfg *IntegrationConfig) {
	t.Helper()
	if !cfg.DockerAvailable {
		t.Skip("Docker is not available, skipping test")
	}
}

// RequireKind skips the test if kind cluster is not available
func RequireKind(t *testing.T, cfg *IntegrationConfig) {
	t.Helper()
	if !cfg.KindAvailable {
		t.Skipf("Kind cluster '%s' is not available, skipping test", cfg.KindClusterName)
	}
}

// CreateTestDockerfile creates a simple Dockerfile for testing
func CreateTestDockerfile(t *testing.T, dir, content string) string {
	t.Helper()

	dockerfilePath := fmt.Sprintf("%s/Dockerfile", dir)
	if err := os.WriteFile(dockerfilePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	return dockerfilePath
}

// SimpleNodeDockerfile is a minimal Node.js Dockerfile for testing
const SimpleNodeDockerfile = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install --production
COPY . .
EXPOSE 3000
CMD ["node", "index.js"]
`

// SimplePythonDockerfile is a minimal Python Dockerfile for testing
const SimplePythonDockerfile = `FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 5000
CMD ["python", "app.py"]
`

// SimpleGoDockerfile is a minimal Go Dockerfile for testing
const SimpleGoDockerfile = `FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o main .

FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]
`
