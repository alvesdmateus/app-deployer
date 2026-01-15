package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types/image"
	dockerregistry "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog/log"
)

// GCPArtifactRegistryClient implements Client for GCP Artifact Registry
type GCPArtifactRegistryClient struct {
	config       Config
	dockerClient *client.Client
	authToken    string
}

// NewGCPArtifactRegistryClient creates a new GCP Artifact Registry client
func NewGCPArtifactRegistryClient(config Config) (*GCPArtifactRegistryClient, error) {
	// Create Docker client for push/pull operations
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &GCPArtifactRegistryClient{
		config:       config,
		dockerClient: cli,
	}, nil
}

// Authenticate authenticates with GCP Artifact Registry using gcloud
func (c *GCPArtifactRegistryClient) Authenticate(ctx context.Context) error {
	log.Info().
		Str("registry", c.config.Host).
		Str("project", c.config.Project).
		Msg("Authenticating with GCP Artifact Registry")

	// Use gcloud to configure Docker authentication
	cmd := exec.CommandContext(ctx,
		"gcloud", "auth", "configure-docker",
		c.getRegistryHost(),
		"--quiet",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return ErrAuthenticationFailed{
			Registry: c.config.Host,
			Err:      fmt.Errorf("gcloud auth failed: %w, output: %s", err, string(output)),
		}
	}

	// Get access token for programmatic access
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return ErrAuthenticationFailed{
			Registry: c.config.Host,
			Err:      fmt.Errorf("failed to get access token: %w", err),
		}
	}

	c.authToken = token

	log.Info().Msg("Successfully authenticated with GCP Artifact Registry")
	return nil
}

// getAccessToken retrieves a GCP access token
func (c *GCPArtifactRegistryClient) getAccessToken(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "gcloud", "auth", "print-access-token")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	token := strings.TrimSpace(string(output))
	return token, nil
}

// VerifyAccess verifies registry access and permissions
func (c *GCPArtifactRegistryClient) VerifyAccess(ctx context.Context) error {
	log.Info().Msg("Verifying GCP Artifact Registry access")

	// Verify gcloud is installed
	if err := c.verifyGCloudInstalled(ctx); err != nil {
		return fmt.Errorf("gcloud CLI not available: %w", err)
	}

	// Verify authentication
	if err := c.Authenticate(ctx); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Verify project access
	if err := c.verifyProjectAccess(ctx); err != nil {
		return fmt.Errorf("project access verification failed: %w", err)
	}

	log.Info().Msg("GCP Artifact Registry access verified successfully")
	return nil
}

// verifyGCloudInstalled checks if gcloud CLI is installed
func (c *GCPArtifactRegistryClient) verifyGCloudInstalled(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "gcloud", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gcloud CLI not found: %w", err)
	}
	return nil
}

// verifyProjectAccess verifies access to the GCP project
func (c *GCPArtifactRegistryClient) verifyProjectAccess(ctx context.Context) error {
	cmd := exec.CommandContext(ctx,
		"gcloud", "projects", "describe", c.config.Project,
		"--format=value(projectId)",
	)

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("cannot access project %s: %w", c.config.Project, err)
	}

	projectID := strings.TrimSpace(string(output))
	if projectID != c.config.Project {
		return fmt.Errorf("project mismatch: expected %s, got %s", c.config.Project, projectID)
	}

	return nil
}

// GetImageTag generates a full image tag for GCP Artifact Registry
func (c *GCPArtifactRegistryClient) GetImageTag(appName, version string) string {
	// Format: LOCATION-docker.pkg.dev/PROJECT/REPOSITORY/IMAGE:TAG
	// Example: us-central1-docker.pkg.dev/my-project/app-deployer/myapp:v1.0.0
	repository := "app-deployer" // Default repository name

	return fmt.Sprintf("%s/%s/%s/%s:%s",
		c.getRegistryHost(),
		c.config.Project,
		repository,
		strings.ToLower(appName),
		version,
	)
}

// getRegistryHost returns the full registry host
func (c *GCPArtifactRegistryClient) getRegistryHost() string {
	if c.config.Host != "" {
		return c.config.Host
	}

	// Default format: LOCATION-docker.pkg.dev
	return fmt.Sprintf("%s-docker.pkg.dev", c.config.Location)
}

// Push pushes an image to GCP Artifact Registry
func (c *GCPArtifactRegistryClient) Push(ctx context.Context, imageTag string) error {
	log.Info().Str("imageTag", imageTag).Msg("Pushing image to GCP Artifact Registry")

	// Ensure authentication
	if c.authToken == "" {
		if err := c.Authenticate(ctx); err != nil {
			return err
		}
	}

	// Prepare auth config
	authConfig := dockerregistry.AuthConfig{
		Username:      "oauth2accesstoken",
		Password:      c.authToken,
		ServerAddress: c.getRegistryHost(),
	}

	encodedAuth, err := c.encodeAuthConfig(authConfig)
	if err != nil {
		return fmt.Errorf("failed to encode auth config: %w", err)
	}

	// Push options
	pushOptions := image.PushOptions{
		RegistryAuth: encodedAuth,
	}

	// Execute push
	pushResponse, err := c.dockerClient.ImagePush(ctx, imageTag, pushOptions)
	if err != nil {
		return ErrPushFailed{
			ImageTag: imageTag,
			Err:      err,
		}
	}
	defer pushResponse.Close()

	// Stream push output
	if err := c.streamPushOutput(ctx, pushResponse); err != nil {
		return ErrPushFailed{
			ImageTag: imageTag,
			Err:      err,
		}
	}

	log.Info().Str("imageTag", imageTag).Msg("Image pushed successfully")
	return nil
}

// Pull pulls an image from GCP Artifact Registry
func (c *GCPArtifactRegistryClient) Pull(ctx context.Context, imageTag string) error {
	log.Info().Str("imageTag", imageTag).Msg("Pulling image from GCP Artifact Registry")

	// Ensure authentication
	if c.authToken == "" {
		if err := c.Authenticate(ctx); err != nil {
			return err
		}
	}

	// Prepare auth config
	authConfig := dockerregistry.AuthConfig{
		Username:      "oauth2accesstoken",
		Password:      c.authToken,
		ServerAddress: c.getRegistryHost(),
	}

	encodedAuth, err := c.encodeAuthConfig(authConfig)
	if err != nil {
		return fmt.Errorf("failed to encode auth config: %w", err)
	}

	// Pull options
	pullOptions := image.PullOptions{
		RegistryAuth: encodedAuth,
	}

	// Execute pull
	pullResponse, err := c.dockerClient.ImagePull(ctx, imageTag, pullOptions)
	if err != nil {
		return ErrPullFailed{
			ImageTag: imageTag,
			Err:      err,
		}
	}
	defer pullResponse.Close()

	// Stream pull output
	if err := c.streamPullOutput(ctx, pullResponse); err != nil {
		return ErrPullFailed{
			ImageTag: imageTag,
			Err:      err,
		}
	}

	log.Info().Str("imageTag", imageTag).Msg("Image pulled successfully")
	return nil
}

// encodeAuthConfig encodes auth config for Docker registry authentication
func (c *GCPArtifactRegistryClient) encodeAuthConfig(authConfig dockerregistry.AuthConfig) (string, error) {
	authJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(authJSON), nil
}

// streamPushOutput streams push output (similar to docker.go implementation)
func (c *GCPArtifactRegistryClient) streamPushOutput(ctx context.Context, reader io.ReadCloser) error {
	decoder := json.NewDecoder(reader)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var msg struct {
			Status   string `json:"status"`
			Progress string `json:"progress"`
			Error    string `json:"error"`
		}

		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("failed to decode push output: %w", err)
		}

		if msg.Error != "" {
			return fmt.Errorf("push error: %s", msg.Error)
		}

		if msg.Status != "" {
			log.Debug().
				Str("status", msg.Status).
				Str("progress", msg.Progress).
				Msg("Push progress")
		}
	}
}

// streamPullOutput streams pull output
func (c *GCPArtifactRegistryClient) streamPullOutput(ctx context.Context, reader io.ReadCloser) error {
	decoder := json.NewDecoder(reader)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var msg struct {
			Status   string `json:"status"`
			Progress string `json:"progress"`
			Error    string `json:"error"`
		}

		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("failed to decode pull output: %w", err)
		}

		if msg.Error != "" {
			return fmt.Errorf("pull error: %s", msg.Error)
		}

		if msg.Status != "" {
			log.Debug().
				Str("status", msg.Status).
				Str("progress", msg.Progress).
				Msg("Pull progress")
		}
	}
}

// Close closes the Docker client connection
func (c *GCPArtifactRegistryClient) Close() error {
	if c.dockerClient != nil {
		return c.dockerClient.Close()
	}
	return nil
}
