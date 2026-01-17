package builder

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/alvesdmateus/app-deployer/internal/builder/dockerfile"
	"github.com/alvesdmateus/app-deployer/internal/builder/registry"
	"github.com/alvesdmateus/app-deployer/internal/builder/scanner"
	"github.com/alvesdmateus/app-deployer/internal/builder/strategies"
)

// Service implements BuildService interface
type Service struct {
	dockerfileGenerator DockerfileGenerator
	buildStrategy       BuildStrategy
	registryClient      RegistryClient
	tracker             BuildTracker
	scanner             scanner.Scanner
}

// ServiceConfig contains configuration for the build service
type ServiceConfig struct {
	RegistryConfig registry.Config
	StrategyType   strategies.StrategyType
	ScannerConfig  scanner.ScanConfig
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

	// Create vulnerability scanner
	scanConfig := config.ScannerConfig
	if scanConfig.Timeout == 0 {
		scanConfig.Timeout = 10 * time.Minute
	}
	trivyScanner := scanner.NewTrivyScanner(scanConfig)

	return &Service{
		dockerfileGenerator: generator,
		buildStrategy:       strategy,
		registryClient:      registryClient,
		tracker:             tracker,
		scanner:             trivyScanner,
	}, nil
}

// BuildImage orchestrates the entire build process:
// 1. Start build tracking
// 2. Generate Dockerfile
// 3. Build container image
// 4. Tag image for registry
// 5. Scan for vulnerabilities
// 6. Push to registry
// 7. Update build status
func (s *Service) BuildImage(ctx context.Context, buildCtx *BuildContext) (*BuildResult, error) {
	log.Info().
		Str("deploymentID", buildCtx.DeploymentID).
		Str("appName", buildCtx.AppName).
		Str("version", buildCtx.Version).
		Msg("Starting container image build")

	// Step 1: Start build tracking
	build, err := s.tracker.StartBuild(ctx, buildCtx.DeploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to start build tracking: %w", err)
	}

	buildCtx.BuildID = build.ID.String()

	// Ensure build failure is tracked if we encounter an error
	defer func() {
		if err != nil {
			if trackErr := s.tracker.FailBuild(ctx, buildCtx.BuildID, err); trackErr != nil {
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

	// Step 5: Scan for vulnerabilities
	if s.scanner != nil && s.scanner.IsAvailable() {
		log.Info().
			Str("imageTag", registryTag).
			Msg("Scanning image for vulnerabilities")

		progressMsg = fmt.Sprintf("Scanning image for vulnerabilities: %s\n", registryTag)
		_ = s.tracker.UpdateProgress(ctx, buildCtx.BuildID, progressMsg)

		scanResult, scanErr := s.scanner.Scan(ctx, registryTag)
		if scanErr != nil {
			result.BuildLog += fmt.Sprintf("\nVulnerability scan failed: %v\n", scanErr)
			_ = s.tracker.UpdateProgress(ctx, buildCtx.BuildID, result.BuildLog)
			return nil, fmt.Errorf("vulnerability scan failed: %w", scanErr)
		}

		// Add scan results to build log
		scanSummary := scanResult.FormatSummary()
		result.BuildLog += "\n" + scanSummary
		_ = s.tracker.UpdateProgress(ctx, buildCtx.BuildID, scanSummary)

		if !scanResult.Pass {
			return nil, fmt.Errorf("vulnerability scan failed: %s", scanResult.FailureReason)
		}

		progressMsg = fmt.Sprintf("Vulnerability scan passed (Critical: %d, High: %d, Medium: %d, Low: %d)\n",
			scanResult.VulnCounts.Critical, scanResult.VulnCounts.High,
			scanResult.VulnCounts.Medium, scanResult.VulnCounts.Low)
		_ = s.tracker.UpdateProgress(ctx, buildCtx.BuildID, progressMsg)
	} else {
		progressMsg = "Vulnerability scanning skipped (scanner not available)\n"
		_ = s.tracker.UpdateProgress(ctx, buildCtx.BuildID, progressMsg)
	}

	// Step 6: Push to registry
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

	// Step 7: Complete build tracking
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
