package builder

import (
	"context"
	"io"
	"time"

	"github.com/alvesdmateus/app-deployer/internal/analyzer"
	"github.com/alvesdmateus/app-deployer/internal/builder/buildtypes"
	"github.com/alvesdmateus/app-deployer/internal/builder/registry"
	"github.com/alvesdmateus/app-deployer/internal/state"
)

// BuildContext is an alias for buildtypes.BuildContext
type BuildContext = buildtypes.BuildContext

// BuildResult is an alias for buildtypes.BuildResult
type BuildResult = buildtypes.BuildResult

// DockerfileGenerator generates optimized Dockerfiles from analysis results
type DockerfileGenerator interface {
	// Generate creates a Dockerfile content based on analysis results
	Generate(ctx context.Context, analysis *analyzer.AnalysisResult) (string, error)
}

// BuildStrategy is an alias for strategies.Strategy - needed to avoid import cycle
type BuildStrategy interface {
	// Build builds a container image from source
	Build(ctx context.Context, buildCtx *BuildContext, dockerfile string) (*BuildResult, error)

	// Name returns the strategy name (e.g., "docker", "buildpack", "nixpack")
	Name() string
}

// RegistryClient is an alias for registry.Client
type RegistryClient = registry.Client

// BuildTracker tracks build status in the database
type BuildTracker interface {
	// StartBuild creates a new build record
	StartBuild(ctx context.Context, deploymentID string) (*state.Build, error)

	// UpdateProgress updates build progress
	UpdateProgress(ctx context.Context, buildID string, log string) error

	// CompleteBuild marks a build as completed
	CompleteBuild(ctx context.Context, buildID string, result *BuildResult) error

	// FailBuild marks a build as failed
	FailBuild(ctx context.Context, buildID string, err error) error

	// GetBuildByID retrieves a build by its ID
	GetBuildByID(ctx context.Context, buildID string) (*state.Build, error)
}

// BuildService orchestrates the entire build process
type BuildService interface {
	// BuildImage builds and pushes a container image
	BuildImage(ctx context.Context, buildCtx *BuildContext) (*BuildResult, error)

	// GetBuildLogs streams build logs
	GetBuildLogs(ctx context.Context, buildID string) (io.Reader, error)

	// CancelBuild cancels an ongoing build
	CancelBuild(ctx context.Context, buildID string) error
}

// BuildOptions contains optional build parameters
type BuildOptions struct {
	UseCache         bool
	CustomDockerfile string
	BuildArgs        map[string]string
	Labels           map[string]string
	Timeout          time.Duration
}
