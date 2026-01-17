package builder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/alvesdmateus/app-deployer/internal/builder/dockerfile"
	"github.com/alvesdmateus/app-deployer/internal/builder/registry"
	"github.com/alvesdmateus/app-deployer/internal/builder/strategies"
)

// ErrBuildTimeout is returned when a build exceeds the configured timeout
var ErrBuildTimeout = errors.New("build timeout exceeded")

// DefaultBuildTimeout is the default maximum time allowed for a build
const DefaultBuildTimeout = 30 * time.Minute

// Service implements BuildService interface
type Service struct {
	dockerfileGenerator DockerfileGenerator
	buildStrategy       BuildStrategy
	registryClient      RegistryClient
	tracker             BuildTracker
	buildTimeout        time.Duration
}

// ServiceConfig contains configuration for the build service
type ServiceConfig struct {
	RegistryConfig registry.Config
	StrategyType   strategies.StrategyType
	BuildTimeout   time.Duration
}

// NewService creates a new build service
func NewService(config ServiceConfig, tracker BuildTracker) (*Service, error) {
	// Create Dockerfile generator
	generator := dockerfile.NewGenerator()

	// Create build strategy
	strategyFactory := strategies.NewStrategyFactory()
	strategy, err := strategyFactory.CreateStrategy(config.StrategyType)
	if err != nil {
		return nil, fmt.Errorf("failed to create build strategy: %w", err)
	}

	// Create registry client
	registryFactory := registry.NewClientFactory()
	registryClient, err := registryFactory.CreateClient(config.RegistryConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	// Use configured timeout or default
	buildTimeout := config.BuildTimeout
	if buildTimeout <= 0 {
		buildTimeout = DefaultBuildTimeout
	}

	return &Service{
		dockerfileGenerator: generator,
		buildStrategy:       strategy,
		registryClient:      registryClient,
		tracker:             tracker,
		buildTimeout:        buildTimeout,
	}, nil
}

// BuildImage orchestrates the entire build process:
// 1. Start build tracking
// 2. Generate Dockerfile
// 3. Build container image
// 4. Tag image for registry
// 5. Push to registry
// 6. Update build status
func (s *Service) BuildImage(ctx context.Context, buildCtx *BuildContext) (*BuildResult, error) {
	log.Info().
		Str("deploymentID", buildCtx.DeploymentID).
		Str("appName", buildCtx.AppName).
		Str("version", buildCtx.Version).
		Dur("timeout", s.buildTimeout).
		Msg("Starting container image build")

	// Wrap context with build timeout
	ctx, cancel := context.WithTimeout(ctx, s.buildTimeout)
	defer cancel()

	// Step 1: Start build tracking
	build, err := s.tracker.StartBuild(ctx, buildCtx.DeploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to start build tracking: %w", err)
	}

	buildCtx.BuildID = build.ID.String()

	// Ensure build failure is tracked if we encounter an error
	defer func() {
		if err != nil {
			// Check if this is a timeout error and wrap it appropriately
			failErr := err
			if errors.Is(err, context.DeadlineExceeded) {
				failErr = fmt.Errorf("%w: build exceeded maximum duration of %v", ErrBuildTimeout, s.buildTimeout)
				log.Error().
					Str("buildID", buildCtx.BuildID).
					Str("deploymentID", buildCtx.DeploymentID).
					Dur("timeout", s.buildTimeout).
					Msg("Build timeout exceeded")
			}
			if trackErr := s.tracker.FailBuild(context.Background(), buildCtx.BuildID, failErr); trackErr != nil {
				log.Error().
					Err(trackErr).
					Str("buildID", buildCtx.BuildID).
					Msg("Failed to track build failure")
			}
		}
	}()

	// Step 2: Generate optimized Dockerfile
	log.Info().
		Str("language", string(buildCtx.Analysis.Language)).
		Str("framework", string(buildCtx.Analysis.Framework)).
		Msg("Generating Dockerfile")

	dockerfileContent, err := s.dockerfileGenerator.Generate(ctx, buildCtx.Analysis)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	// Update build progress
	progressMsg := fmt.Sprintf("Generated Dockerfile for %s application\n", buildCtx.Analysis.Language)
	_ = s.tracker.UpdateProgress(ctx, buildCtx.BuildID, progressMsg)

	// Step 3: Build container image
	log.Info().
		Str("buildID", buildCtx.BuildID).
		Str("strategy", s.buildStrategy.Name()).
		Msg("Building container image")

	result, err := s.buildStrategy.Build(ctx, buildCtx, dockerfileContent)
	if err != nil {
		result.BuildLog += fmt.Sprintf("\nBuild failed: %v\n", err)
		_ = s.tracker.UpdateProgress(ctx, buildCtx.BuildID, result.BuildLog)
		return nil, fmt.Errorf("build failed: %w", err)
	}

	if !result.Success {
		_ = s.tracker.UpdateProgress(ctx, buildCtx.BuildID, result.BuildLog)
		return nil, fmt.Errorf("build unsuccessful: %v", result.Error)
	}

	// Update build progress
	progressMsg = fmt.Sprintf("Container image built successfully: %s\n", result.ImageTag)
	_ = s.tracker.UpdateProgress(ctx, buildCtx.BuildID, progressMsg)

	// Step 4: Tag image for registry
	registryTag := s.registryClient.GetImageTag(buildCtx.AppName, buildCtx.Version)

	log.Info().
		Str("sourceTag", result.ImageTag).
		Str("registryTag", registryTag).
		Msg("Tagging image for registry")

	// Tag the local image with the registry tag
	if dockerStrategy, ok := s.buildStrategy.(*strategies.DockerStrategy); ok {
		if err := dockerStrategy.TagImage(ctx, result.ImageTag, registryTag); err != nil {
			return nil, fmt.Errorf("failed to tag image: %w", err)
		}
	}

	result.ImageTag = registryTag

	// Step 5: Push to registry
	log.Info().
		Str("imageTag", registryTag).
		Msg("Pushing image to registry")

	progressMsg = fmt.Sprintf("Pushing image to registry: %s\n", registryTag)
	_ = s.tracker.UpdateProgress(ctx, buildCtx.BuildID, progressMsg)

	if err := s.registryClient.Push(ctx, registryTag); err != nil {
		return nil, fmt.Errorf("failed to push image to registry: %w", err)
	}

	progressMsg = "Image pushed successfully to registry\n"
	_ = s.tracker.UpdateProgress(ctx, buildCtx.BuildID, progressMsg)

	// Step 6: Complete build tracking
	if err := s.tracker.CompleteBuild(ctx, buildCtx.BuildID, result); err != nil {
		log.Error().
			Err(err).
			Str("buildID", buildCtx.BuildID).
			Msg("Failed to complete build tracking")
		// Don't fail the build, just log the error
	}

	log.Info().
		Str("buildID", buildCtx.BuildID).
		Str("imageTag", registryTag).
		Dur("duration", result.BuildDuration).
		Msg("Container image build completed successfully")

	return result, nil
}

// GetBuildLogs retrieves build logs for streaming
func (s *Service) GetBuildLogs(ctx context.Context, buildID string) (io.Reader, error) {
	log.Debug().Str("buildID", buildID).Msg("Retrieving build logs")

	build, err := s.tracker.GetBuildByID(ctx, buildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get build: %w", err)
	}

	if build == nil {
		return nil, fmt.Errorf("build not found: %s", buildID)
	}

	return strings.NewReader(build.BuildLog), nil
}

// CancelBuild cancels an ongoing build
func (s *Service) CancelBuild(ctx context.Context, buildID string) error {
	log.Warn().Str("buildID", buildID).Msg("Cancelling build")

	// Mark build as failed with cancellation message
	cancelErr := fmt.Errorf("build cancelled by user")
	if err := s.tracker.FailBuild(ctx, buildID, cancelErr); err != nil {
		return fmt.Errorf("failed to cancel build: %w", err)
	}

	// TODO: Actually terminate the running build process
	// This would require tracking build processes and sending cancellation signals
	// For now, we just mark it as failed in the database

	log.Info().Str("buildID", buildID).Msg("Build cancelled successfully")
	return nil
}

// VerifyRegistryAccess verifies access to the container registry
func (s *Service) VerifyRegistryAccess(ctx context.Context) error {
	log.Info().Msg("Verifying registry access")

	if err := s.registryClient.VerifyAccess(ctx); err != nil {
		return fmt.Errorf("registry access verification failed: %w", err)
	}

	log.Info().Msg("Registry access verified successfully")
	return nil
}

// Close cleans up resources
func (s *Service) Close() error {
	// Close registry client if it implements io.Closer
	if closer, ok := s.registryClient.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close registry client")
		}
	}

	// Close build strategy if it implements io.Closer
	if closer, ok := s.buildStrategy.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close build strategy")
		}
	}

	return nil
}
