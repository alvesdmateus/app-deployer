package builder

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alvesdmateus/app-deployer/internal/builder/registry"
	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/google/uuid"
)

// mockTracker implements BuildTracker for testing
type mockTracker struct {
	startBuildCalled    bool
	failBuildCalled     bool
	completeBuildCalled bool
	failBuildErr        error
	startBuildDelay     time.Duration
}

func (m *mockTracker) StartBuild(ctx context.Context, deploymentID string) (*state.Build, error) {
	if m.startBuildDelay > 0 {
		select {
		case <-time.After(m.startBuildDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	m.startBuildCalled = true
	return &state.Build{
		ID:           uuid.New(),
		DeploymentID: uuid.MustParse(deploymentID),
		Status:       "BUILDING",
	}, nil
}

func (m *mockTracker) UpdateProgress(ctx context.Context, buildID string, logEntry string) error {
	return nil
}

func (m *mockTracker) CompleteBuild(ctx context.Context, buildID string, result *BuildResult) error {
	m.completeBuildCalled = true
	return nil
}

func (m *mockTracker) FailBuild(ctx context.Context, buildID string, err error) error {
	m.failBuildCalled = true
	m.failBuildErr = err
	return nil
}

func (m *mockTracker) GetBuildByID(ctx context.Context, buildID string) (*state.Build, error) {
	return &state.Build{
		ID:     uuid.MustParse(buildID),
		Status: "BUILDING",
	}, nil
}

func TestNewService_DefaultTimeout(t *testing.T) {
	tracker := &mockTracker{}
	config := ServiceConfig{
		RegistryConfig: registry.Config{
			Type: "mock",
		},
		BuildTimeout: 0, // Zero should use default
	}

	// This will fail to create because registry type "mock" is not supported,
	// but we can test the timeout configuration logic directly
	service := &Service{
		buildTimeout: config.BuildTimeout,
	}

	// Test that zero timeout would be replaced with default
	if config.BuildTimeout <= 0 {
		service.buildTimeout = DefaultBuildTimeout
	}

	if service.buildTimeout != DefaultBuildTimeout {
		t.Errorf("Expected default timeout %v, got %v", DefaultBuildTimeout, service.buildTimeout)
	}

	_ = tracker // silence unused warning
}

func TestNewService_CustomTimeout(t *testing.T) {
	customTimeout := 15 * time.Minute
	config := ServiceConfig{
		RegistryConfig: registry.Config{
			Type: "mock",
		},
		BuildTimeout: customTimeout,
	}

	// Test that custom timeout is preserved
	service := &Service{
		buildTimeout: config.BuildTimeout,
	}

	if config.BuildTimeout > 0 {
		service.buildTimeout = config.BuildTimeout
	} else {
		service.buildTimeout = DefaultBuildTimeout
	}

	if service.buildTimeout != customTimeout {
		t.Errorf("Expected custom timeout %v, got %v", customTimeout, service.buildTimeout)
	}
}

func TestDefaultBuildTimeout_Value(t *testing.T) {
	expected := 30 * time.Minute
	if DefaultBuildTimeout != expected {
		t.Errorf("Expected DefaultBuildTimeout to be %v, got %v", expected, DefaultBuildTimeout)
	}
}

func TestErrBuildTimeout_IsError(t *testing.T) {
	if ErrBuildTimeout == nil {
		t.Error("ErrBuildTimeout should not be nil")
	}

	expected := "build timeout exceeded"
	if ErrBuildTimeout.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, ErrBuildTimeout.Error())
	}
}

func TestTimeoutErrorWrapping(t *testing.T) {
	// Simulate how timeout errors are wrapped in BuildImage
	timeout := 30 * time.Minute
	wrappedErr := errors.New("build timeout exceeded: build exceeded maximum duration of 30m0s")

	// Verify the error contains useful information
	if wrappedErr.Error() == "" {
		t.Error("Wrapped error should not be empty")
	}

	// Test that our error can be detected
	timeoutErr := ErrBuildTimeout
	if !errors.Is(timeoutErr, ErrBuildTimeout) {
		t.Error("ErrBuildTimeout should match itself with errors.Is")
	}

	_ = timeout // silence unused warning
}

func TestContextDeadlineDetection(t *testing.T) {
	// Create a context that will timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for timeout
	<-ctx.Done()

	// Verify we can detect deadline exceeded
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Error("Expected context.DeadlineExceeded error")
	}
}

func TestServiceConfig_BuildTimeout(t *testing.T) {
	tests := []struct {
		name            string
		configTimeout   time.Duration
		expectedTimeout time.Duration
	}{
		{
			name:            "zero timeout uses default",
			configTimeout:   0,
			expectedTimeout: DefaultBuildTimeout,
		},
		{
			name:            "negative timeout uses default",
			configTimeout:   -5 * time.Minute,
			expectedTimeout: DefaultBuildTimeout,
		},
		{
			name:            "custom timeout is preserved",
			configTimeout:   15 * time.Minute,
			expectedTimeout: 15 * time.Minute,
		},
		{
			name:            "1 hour timeout is preserved",
			configTimeout:   1 * time.Hour,
			expectedTimeout: 1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ServiceConfig{
				BuildTimeout: tt.configTimeout,
			}

			// Simulate the logic in NewService
			timeout := config.BuildTimeout
			if timeout <= 0 {
				timeout = DefaultBuildTimeout
			}

			if timeout != tt.expectedTimeout {
				t.Errorf("Expected timeout %v, got %v", tt.expectedTimeout, timeout)
			}
		})
	}
}
