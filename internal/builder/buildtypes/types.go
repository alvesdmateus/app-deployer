package buildtypes

import (
	"time"

	"github.com/alvesdmateus/app-deployer/internal/analyzer"
)

// BuildContext contains all information needed for building a container image
type BuildContext struct {
	DeploymentID string
	AppName      string
	Version      string
	SourcePath   string
	Analysis     *analyzer.AnalysisResult
	RegistryType string
	RegistryHost string
	BuildID      string
}

// BuildResult contains the output of a build operation
type BuildResult struct {
	ImageTag      string
	ImageDigest   string
	BuildDuration time.Duration
	BuildLog      string
	Success       bool
	Error         error
}
