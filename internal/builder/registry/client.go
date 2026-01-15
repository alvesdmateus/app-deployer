package registry

import (
	"fmt"
)

// RegistryType defines the type of container registry
type RegistryType string

const (
	RegistryTypeGCPArtifact RegistryType = "artifact-registry"
	RegistryTypeHarbor      RegistryType = "harbor"
	RegistryTypeECR         RegistryType = "ecr"
	RegistryTypeACR         RegistryType = "acr"
)

// ClientFactory creates registry clients based on configuration
type ClientFactory struct {
	// Future: Add caching for client instances
}

// NewClientFactory creates a new registry client factory
func NewClientFactory() *ClientFactory {
	return &ClientFactory{}
}

// CreateClient creates a registry client based on configuration
func (f *ClientFactory) CreateClient(config Config) (Client, error) {
	registryType := RegistryType(config.Type)

	switch registryType {
	case RegistryTypeGCPArtifact:
		return NewGCPArtifactRegistryClient(config)
	case RegistryTypeHarbor:
		// Future implementation
		return nil, ErrRegistryNotImplemented{Type: registryType}
	case RegistryTypeECR:
		// Future implementation
		return nil, ErrRegistryNotImplemented{Type: registryType}
	case RegistryTypeACR:
		// Future implementation
		return nil, ErrRegistryNotImplemented{Type: registryType}
	default:
		return nil, ErrUnknownRegistry{Type: registryType}
	}
}

// ErrRegistryNotImplemented is returned when a registry type is not yet implemented
type ErrRegistryNotImplemented struct {
	Type RegistryType
}

func (e ErrRegistryNotImplemented) Error() string {
	return "registry not implemented: " + string(e.Type)
}

// ErrUnknownRegistry is returned when an unknown registry type is requested
type ErrUnknownRegistry struct {
	Type RegistryType
}

func (e ErrUnknownRegistry) Error() string {
	return "unknown registry type: " + string(e.Type)
}

// ErrAuthenticationFailed is returned when registry authentication fails
type ErrAuthenticationFailed struct {
	Registry string
	Err      error
}

func (e ErrAuthenticationFailed) Error() string {
	return fmt.Sprintf("authentication failed for registry %s: %v", e.Registry, e.Err)
}

// ErrPushFailed is returned when image push fails
type ErrPushFailed struct {
	ImageTag string
	Err      error
}

func (e ErrPushFailed) Error() string {
	return fmt.Sprintf("failed to push image %s: %v", e.ImageTag, e.Err)
}

// ErrPullFailed is returned when image pull fails
type ErrPullFailed struct {
	ImageTag string
	Err      error
}

func (e ErrPullFailed) Error() string {
	return fmt.Sprintf("failed to pull image %s: %v", e.ImageTag, e.Err)
}
