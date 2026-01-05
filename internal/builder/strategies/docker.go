package strategies

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog/log"

	"github.com/alvesdmateus/app-deployer/internal/builder/buildtypes"
)

// DockerStrategy implements BuildStrategy using Docker
type DockerStrategy struct {
	client *client.Client
}

// NewDockerStrategy creates a new Docker build strategy
func NewDockerStrategy() (*DockerStrategy, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &DockerStrategy{
		client: cli,
	}, nil
}

// Name returns the strategy name
func (s *DockerStrategy) Name() string {
	return string(StrategyTypeDocker)
}

// Build builds a container image using Docker
func (s *DockerStrategy) Build(ctx context.Context, buildCtx *buildtypes.BuildContext, dockerfile string) (*buildtypes.BuildResult, error) {
	startTime := time.Now()
	result := &buildtypes.BuildResult{
		Success: false,
	}

	// Verify Docker is accessible
	if err := s.verifyDockerAccess(ctx); err != nil {
		result.Error = fmt.Errorf("docker not accessible: %w", err)
		return result, result.Error
	}

	// Generate image tag
	imageTag := s.generateImageTag(buildCtx)
	log.Info().
		Str("imageTag", imageTag).
		Str("deploymentID", buildCtx.DeploymentID).
		Msg("Building Docker image")

	// Write Dockerfile to source directory
	dockerfilePath := filepath.Join(buildCtx.SourcePath, "Dockerfile.generated")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
		result.Error = fmt.Errorf("failed to write Dockerfile: %w", err)
		return result, result.Error
	}
	defer os.Remove(dockerfilePath) // Clean up generated Dockerfile

	// Create build context tar
	buildContextTar, err := s.createBuildContext(buildCtx.SourcePath, "Dockerfile.generated")
	if err != nil {
		result.Error = fmt.Errorf("failed to create build context: %w", err)
		return result, result.Error
	}
	defer buildContextTar.Close()

	// Build options
	buildOptions := types.ImageBuildOptions{
		Tags:       []string{imageTag},
		Dockerfile: "Dockerfile.generated",
		Remove:     true,        // Remove intermediate containers
		ForceRemove: true,       // Always remove intermediate containers
		PullParent: true,        // Pull parent images
		NoCache:    false,       // Use cache for faster builds
		Labels: map[string]string{
			"app.deployer.deployment": buildCtx.DeploymentID,
			"app.deployer.app":        buildCtx.AppName,
			"app.deployer.version":    buildCtx.Version,
			"app.deployer.build-id":   buildCtx.BuildID,
		},
	}

	// Execute build
	buildResponse, err := s.client.ImageBuild(ctx, buildContextTar, buildOptions)
	if err != nil {
		result.Error = fmt.Errorf("docker build failed: %w", err)
		return result, result.Error
	}
	defer buildResponse.Body.Close()

	// Capture build logs
	var buildLog strings.Builder
	if err := s.streamBuildOutput(ctx, buildResponse.Body, &buildLog); err != nil {
		result.Error = fmt.Errorf("failed to stream build output: %w", err)
		result.BuildLog = buildLog.String()
		return result, result.Error
	}

	// Get image details including digest
	imageInspect, _, err := s.client.ImageInspectWithRaw(ctx, imageTag)
	if err != nil {
		result.Error = fmt.Errorf("failed to inspect built image: %w", err)
		result.BuildLog = buildLog.String()
		return result, result.Error
	}

	// Successful build
	result.Success = true
	result.ImageTag = imageTag
	result.ImageDigest = imageInspect.ID
	result.BuildDuration = time.Since(startTime)
	result.BuildLog = buildLog.String()

	log.Info().
		Str("imageTag", imageTag).
		Str("digest", result.ImageDigest).
		Dur("duration", result.BuildDuration).
		Msg("Docker build completed successfully")

	return result, nil
}

// verifyDockerAccess checks if Docker daemon is accessible
func (s *DockerStrategy) verifyDockerAccess(ctx context.Context) error {
	_, err := s.client.Ping(ctx)
	if err != nil {
		return fmt.Errorf("docker daemon not accessible: %w", err)
	}
	return nil
}

// generateImageTag generates a full image tag
func (s *DockerStrategy) generateImageTag(buildCtx *buildtypes.BuildContext) string {
	// For local builds, use simple tag
	// In production, this will be the full registry path
	return fmt.Sprintf("%s:%s",
		strings.ToLower(buildCtx.AppName),
		buildCtx.Version,
	)
}

// createBuildContext creates a tar archive of the build context
func (s *DockerStrategy) createBuildContext(sourcePath, dockerfileName string) (io.ReadCloser, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	// Patterns to exclude
	excludePatterns := map[string]bool{
		".git":         true,
		".github":      true,
		"node_modules": true,
		"vendor":       true,
		".env":         true,
	}

	// Walk through source directory
	err := filepath.Walk(sourcePath, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(sourcePath, file)
		if err != nil {
			return err
		}

		// Skip excluded directories
		for pattern := range excludePatterns {
			if strings.Contains(relPath, pattern) {
				if fi.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Skip log files
		if strings.HasSuffix(relPath, ".log") {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		// Use relative path as name
		header.Name = filepath.ToSlash(relPath)

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If not a directory, write file content
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			defer data.Close()

			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create tar archive: %w", err)
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// streamBuildOutput streams and parses Docker build output
func (s *DockerStrategy) streamBuildOutput(ctx context.Context, reader io.Reader, buildLog *strings.Builder) error {
	decoder := json.NewDecoder(reader)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var msg struct {
			Stream      string `json:"stream"`
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}

		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("failed to decode build output: %w", err)
		}

		// Handle errors
		if msg.Error != "" {
			buildLog.WriteString(msg.Error)
			return fmt.Errorf("build error: %s", msg.ErrorDetail.Message)
		}

		// Write stream output
		if msg.Stream != "" {
			buildLog.WriteString(msg.Stream)
			// Log to console as well
			log.Debug().Str("output", strings.TrimSpace(msg.Stream)).Msg("Build output")
		}
	}
}

// TagImage tags an existing image with a new tag
func (s *DockerStrategy) TagImage(ctx context.Context, sourceTag, targetTag string) error {
	log.Info().
		Str("source", sourceTag).
		Str("target", targetTag).
		Msg("Tagging Docker image")

	if err := s.client.ImageTag(ctx, sourceTag, targetTag); err != nil {
		return fmt.Errorf("failed to tag image: %w", err)
	}

	return nil
}

// RemoveImage removes an image from local Docker daemon
func (s *DockerStrategy) RemoveImage(ctx context.Context, imageTag string) error {
	log.Info().Str("imageTag", imageTag).Msg("Removing Docker image")

	_, err := s.client.ImageRemove(ctx, imageTag, image.RemoveOptions{
		Force:         true,
		PruneChildren: true,
	})

	if err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}

	return nil
}

// PushImage pushes an image to a registry
// Note: This is handled by RegistryClient, but provided here for convenience
func (s *DockerStrategy) PushImage(ctx context.Context, imageTag string) error {
	log.Info().Str("imageTag", imageTag).Msg("Pushing Docker image")

	pushOptions := image.PushOptions{}

	pushResponse, err := s.client.ImagePush(ctx, imageTag, pushOptions)
	if err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}
	defer pushResponse.Close()

	// Stream push output
	var pushLog strings.Builder
	if err := s.streamPushOutput(ctx, pushResponse, &pushLog); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	log.Info().Str("imageTag", imageTag).Msg("Image pushed successfully")
	return nil
}

// streamPushOutput streams Docker push output
func (s *DockerStrategy) streamPushOutput(ctx context.Context, reader io.ReadCloser, pushLog *strings.Builder) error {
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
			pushLog.WriteString(fmt.Sprintf("%s %s\n", msg.Status, msg.Progress))
			log.Debug().Str("status", msg.Status).Msg("Push progress")
		}
	}
}

// Close closes the Docker client connection
func (s *DockerStrategy) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
