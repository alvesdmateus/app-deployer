package strategies

import (
	"context"

	"github.com/alvesdmateus/app-deployer/internal/builder/buildtypes"
)

// Strategy defines how to build container images
type Strategy interface {
	// Build builds a container image from source
	Build(ctx context.Context, buildCtx *buildtypes.BuildContext, dockerfile string) (*buildtypes.BuildResult, error)

	// Name returns the strategy name (e.g., "docker", "buildpack", "nixpack")
	Name() string
}

// StrategyType defines the type of build strategy
type StrategyType string

const (
	StrategyTypeDocker    StrategyType = "docker"
	StrategyTypeBuildpack StrategyType = "buildpack"
	StrategyTypeNixpack   StrategyType = "nixpack"
)

// StrategyFactory creates build strategies based on type
type StrategyFactory struct {
	// Future: Add configuration, registry for custom strategies
}

// NewStrategyFactory creates a new strategy factory
func NewStrategyFactory() *StrategyFactory {
	return &StrategyFactory{}
}

// CreateStrategy creates a build strategy based on the specified type
func (f *StrategyFactory) CreateStrategy(strategyType StrategyType) (Strategy, error) {
	switch strategyType {
	case StrategyTypeDocker:
		return NewDockerStrategy()
	case StrategyTypeBuildpack:
		// Future implementation
		return nil, ErrStrategyNotImplemented{Type: strategyType}
	case StrategyTypeNixpack:
		// Future implementation
		return nil, ErrStrategyNotImplemented{Type: strategyType}
	default:
		return nil, ErrUnknownStrategy{Type: strategyType}
	}
}

// GetDefaultStrategy returns the default build strategy
func (f *StrategyFactory) GetDefaultStrategy() Strategy {
	strategy, _ := f.CreateStrategy(StrategyTypeDocker)
	return strategy
}

// ErrStrategyNotImplemented is returned when a strategy is not yet implemented
type ErrStrategyNotImplemented struct {
	Type StrategyType
}

func (e ErrStrategyNotImplemented) Error() string {
	return "strategy not implemented: " + string(e.Type)
}

// ErrUnknownStrategy is returned when an unknown strategy type is requested
type ErrUnknownStrategy struct {
	Type StrategyType
}

func (e ErrUnknownStrategy) Error() string {
	return "unknown strategy type: " + string(e.Type)
}

// BuildHook is a function that can be called during the build process
type BuildHook func(ctx context.Context, stage string, output string) error

// BuildContextWithHooks extends buildtypes.BuildContext with strategy-specific options
type BuildContextWithHooks struct {
	*buildtypes.BuildContext
	OnProgress BuildHook
	OnStage    BuildHook
	OnComplete BuildHook
}
