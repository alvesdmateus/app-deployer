package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func TestDefaultTracingConfig(t *testing.T) {
	config := DefaultTracingConfig()

	assert.False(t, config.Enabled)
	assert.Equal(t, "app-deployer", config.ServiceName)
	assert.Equal(t, "1.0.0", config.ServiceVersion)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, "localhost:4318", config.OTLPEndpoint)
	assert.Equal(t, 1.0, config.SampleRate)
	assert.True(t, config.Insecure)
}

func TestNewTracer_Disabled(t *testing.T) {
	config := TracingConfig{
		Enabled:     false,
		ServiceName: "test-service",
	}

	tracer, err := NewTracer(context.Background(), config)
	require.NoError(t, err)
	assert.NotNil(t, tracer)
	assert.False(t, tracer.IsEnabled())

	// Should be able to create spans even when disabled (no-op)
	ctx, span := tracer.StartSpan(context.Background(), "test-span")
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	span.End()

	// Shutdown should work
	err = tracer.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestNewTracer_EnabledWithInvalidEndpoint(t *testing.T) {
	// When enabled but no collector is running, tracer creation should still succeed
	// (errors occur during export, not initialization)
	config := TracingConfig{
		Enabled:        true,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		OTLPEndpoint:   "invalid-endpoint:9999",
		SampleRate:     1.0,
		Insecure:       true,
	}

	tracer, err := NewTracer(context.Background(), config)
	require.NoError(t, err)
	assert.NotNil(t, tracer)
	assert.True(t, tracer.IsEnabled())

	// Shutdown should work
	err = tracer.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestTracer_StartSpan(t *testing.T) {
	config := TracingConfig{
		Enabled:     false, // Use no-op tracer for testing
		ServiceName: "test-service",
	}

	tracer, err := NewTracer(context.Background(), config)
	require.NoError(t, err)

	ctx, span := tracer.StartSpan(context.Background(), "test-operation")
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)

	// End the span
	span.End()
}

func TestTracer_SpanFromContext(t *testing.T) {
	config := TracingConfig{
		Enabled:     false,
		ServiceName: "test-service",
	}

	tracer, err := NewTracer(context.Background(), config)
	require.NoError(t, err)

	ctx, span := tracer.StartSpan(context.Background(), "test-span")
	defer span.End()

	// Get span from context
	retrievedSpan := tracer.SpanFromContext(ctx)
	assert.NotNil(t, retrievedSpan)
}

func TestTracer_AddEvent(t *testing.T) {
	config := TracingConfig{
		Enabled:     false,
		ServiceName: "test-service",
	}

	tracer, err := NewTracer(context.Background(), config)
	require.NoError(t, err)

	ctx, span := tracer.StartSpan(context.Background(), "test-span")
	defer span.End()

	// Should not panic
	tracer.AddEvent(ctx, "test-event", attribute.String("key", "value"))
}

func TestTracer_SetAttributes(t *testing.T) {
	config := TracingConfig{
		Enabled:     false,
		ServiceName: "test-service",
	}

	tracer, err := NewTracer(context.Background(), config)
	require.NoError(t, err)

	ctx, span := tracer.StartSpan(context.Background(), "test-span")
	defer span.End()

	// Should not panic
	tracer.SetAttributes(ctx, attribute.String("key", "value"))
}

func TestTracer_RecordError(t *testing.T) {
	config := TracingConfig{
		Enabled:     false,
		ServiceName: "test-service",
	}

	tracer, err := NewTracer(context.Background(), config)
	require.NoError(t, err)

	ctx, span := tracer.StartSpan(context.Background(), "test-span")
	defer span.End()

	// Should not panic
	tracer.RecordError(ctx, assert.AnError)
}

func TestDeploymentSpanAttributes(t *testing.T) {
	attrs := DeploymentSpanAttributes("dep-123", "running", "build")

	assert.Len(t, attrs, 3)
	assert.Equal(t, "deployment.id", string(attrs[0].Key))
	assert.Equal(t, "dep-123", attrs[0].Value.AsString())
	assert.Equal(t, "deployment.status", string(attrs[1].Key))
	assert.Equal(t, "running", attrs[1].Value.AsString())
	assert.Equal(t, "deployment.phase", string(attrs[2].Key))
	assert.Equal(t, "build", attrs[2].Value.AsString())
}

func TestBuildSpanAttributes(t *testing.T) {
	attrs := BuildSpanAttributes("build-456", "success", "go", "myapp:latest")

	assert.Len(t, attrs, 4)
	assert.Equal(t, "build.id", string(attrs[0].Key))
	assert.Equal(t, "build-456", attrs[0].Value.AsString())
	assert.Equal(t, "build.status", string(attrs[1].Key))
	assert.Equal(t, "success", attrs[1].Value.AsString())
	assert.Equal(t, "build.language", string(attrs[2].Key))
	assert.Equal(t, "go", attrs[2].Value.AsString())
	assert.Equal(t, "build.image", string(attrs[3].Key))
	assert.Equal(t, "myapp:latest", attrs[3].Value.AsString())
}

func TestInfrastructureSpanAttributes(t *testing.T) {
	attrs := InfrastructureSpanAttributes("gcp", "us-central1", "my-cluster")

	assert.Len(t, attrs, 3)
	assert.Equal(t, "cloud.provider", string(attrs[0].Key))
	assert.Equal(t, "gcp", attrs[0].Value.AsString())
	assert.Equal(t, "cloud.region", string(attrs[1].Key))
	assert.Equal(t, "us-central1", attrs[1].Value.AsString())
	assert.Equal(t, "cluster.name", string(attrs[2].Key))
	assert.Equal(t, "my-cluster", attrs[2].Value.AsString())
}

func TestJobSpanAttributes(t *testing.T) {
	attrs := JobSpanAttributes("deployments", "job-789", "build")

	assert.Len(t, attrs, 3)
	assert.Equal(t, "queue.name", string(attrs[0].Key))
	assert.Equal(t, "deployments", attrs[0].Value.AsString())
	assert.Equal(t, "job.id", string(attrs[1].Key))
	assert.Equal(t, "job-789", attrs[1].Value.AsString())
	assert.Equal(t, "job.type", string(attrs[2].Key))
	assert.Equal(t, "build", attrs[2].Value.AsString())
}

func TestGlobalTracer(t *testing.T) {
	// Reset global tracer
	globalTracer = nil

	// Get global tracer without initialization (should return no-op)
	tracer := GetGlobalTracer()
	assert.NotNil(t, tracer)
	assert.False(t, tracer.IsEnabled())

	// Initialize global tracer
	config := TracingConfig{
		Enabled:     false,
		ServiceName: "test-global",
	}
	err := InitGlobalTracer(context.Background(), config)
	require.NoError(t, err)

	// Get initialized tracer
	tracer = GetGlobalTracer()
	assert.NotNil(t, tracer)

	// Shutdown global tracer
	err = ShutdownGlobalTracer(context.Background())
	assert.NoError(t, err)

	// Reset for other tests
	globalTracer = nil
}

func TestSamplerConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
	}{
		{"always sample", 1.0},
		{"never sample", 0.0},
		{"50% sample", 0.5},
		{"above 100%", 1.5},     // Should clamp to always sample
		{"negative rate", -0.5}, // Should clamp to never sample
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := TracingConfig{
				Enabled:        true,
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Environment:    "test",
				OTLPEndpoint:   "localhost:4318",
				SampleRate:     tt.sampleRate,
				Insecure:       true,
			}

			tracer, err := NewTracer(context.Background(), config)
			require.NoError(t, err)
			assert.NotNil(t, tracer)

			err = tracer.Shutdown(context.Background())
			assert.NoError(t, err)
		})
	}
}
