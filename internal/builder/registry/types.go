package registry

import "context"

// Config contains registry-specific configuration
type Config struct {
	Type     string // artifact-registry, harbor, ecr, acr
	Host     string // e.g., us-central1-docker.pkg.dev
	Project  string // GCP project or equivalent
	Location string // Region or location
}

// Client handles container registry operations
type Client interface {
	// Push pushes an image to the registry
	Push(ctx context.Context, imageTag string) error

	// Pull pulls an image from the registry
	Pull(ctx context.Context, imageTag string) error

	// GetImageTag generates a full image tag for the registry
	// Format: registry.host/project/app:version
	GetImageTag(appName, version string) string

	// Authenticate authenticates with the registry
	Authenticate(ctx context.Context) error

	// VerifyAccess verifies registry access and permissions
	VerifyAccess(ctx context.Context) error
}
