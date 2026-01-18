package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingConfig holds configuration for distributed tracing
type TracingConfig struct {
	// Enabled determines if tracing is enabled
	Enabled bool
	// ServiceName is the name of the service for tracing
	ServiceName string
	// ServiceVersion is the version of the service
	ServiceVersion string
	// Environment is the deployment environment (dev, staging, prod)
	Environment string
	// OTLPEndpoint is the OpenTelemetry collector endpoint (e.g., "localhost:4318")
	OTLPEndpoint string
	// SampleRate is the sampling rate (0.0 to 1.0, where 1.0 = 100%)
	SampleRate float64
	// Insecure disables TLS for the OTLP exporter
	Insecure bool
}

// DefaultTracingConfig returns a default tracing configuration
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		Enabled:        false,
		ServiceName:    "app-deployer",
		ServiceVersion: "1.0.0",
		Environment:    "development",
		OTLPEndpoint:   "localhost:4318",
		SampleRate:     1.0,
		Insecure:       true,
	}
}

// Tracer wraps OpenTelemetry tracing functionality
type Tracer struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	config   TracingConfig
}

// NewTracer creates a new tracer with the given configuration
func NewTracer(ctx context.Context, config TracingConfig) (*Tracer, error) {
	if !config.Enabled {
		// Return a no-op tracer when disabled
		return &Tracer{
			tracer: otel.Tracer(config.ServiceName),
			config: config,
		}, nil
	}

	// Create OTLP exporter options
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(config.OTLPEndpoint),
	}
	if config.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	// Create OTLP exporter
	client := otlptracehttp.NewClient(opts...)
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	// Use resource.New instead of resource.Merge to avoid schema URL conflicts
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			semconv.DeploymentEnvironment(config.Environment),
		),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create sampler based on sample rate
	var sampler sdktrace.Sampler
	if config.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if config.SampleRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(config.SampleRate)
	}

	// Create trace provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global trace provider and propagator
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Tracer{
		provider: provider,
		tracer:   provider.Tracer(config.ServiceName),
		config:   config,
	}, nil
}

// Shutdown gracefully shuts down the tracer
func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.provider != nil {
		return t.provider.Shutdown(ctx)
	}
	return nil
}

// StartSpan starts a new span with the given name
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// SpanFromContext returns the current span from the context
func (t *Tracer) SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddEvent adds an event to the current span
func (t *Tracer) AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetAttributes sets attributes on the current span
func (t *Tracer) SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordError records an error on the current span
func (t *Tracer) RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err, opts...)
}

// IsEnabled returns whether tracing is enabled
func (t *Tracer) IsEnabled() bool {
	return t.config.Enabled
}

// Common attribute keys for app-deployer
var (
	// Deployment attributes
	AttrDeploymentID     = attribute.Key("deployment.id")
	AttrDeploymentStatus = attribute.Key("deployment.status")
	AttrDeploymentPhase  = attribute.Key("deployment.phase")

	// Build attributes
	AttrBuildID       = attribute.Key("build.id")
	AttrBuildStatus   = attribute.Key("build.status")
	AttrBuildLanguage = attribute.Key("build.language")
	AttrBuildImage    = attribute.Key("build.image")

	// Infrastructure attributes
	AttrCloudProvider = attribute.Key("cloud.provider")
	AttrCloudRegion   = attribute.Key("cloud.region")
	AttrClusterName   = attribute.Key("cluster.name")

	// User attributes
	AttrUserID    = attribute.Key("user.id")
	AttrUserEmail = attribute.Key("user.email")

	// Repository attributes
	AttrRepoURL    = attribute.Key("repo.url")
	AttrRepoBranch = attribute.Key("repo.branch")
	AttrRepoCommit = attribute.Key("repo.commit")

	// HTTP attributes (in addition to semconv)
	AttrHTTPRoute = attribute.Key("http.route")

	// Queue attributes
	AttrQueueName = attribute.Key("queue.name")
	AttrJobID     = attribute.Key("job.id")
	AttrJobType   = attribute.Key("job.type")
)

// DeploymentSpanAttributes returns common attributes for deployment spans
func DeploymentSpanAttributes(deploymentID, status, phase string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrDeploymentID.String(deploymentID),
		AttrDeploymentStatus.String(status),
		AttrDeploymentPhase.String(phase),
	}
}

// BuildSpanAttributes returns common attributes for build spans
func BuildSpanAttributes(buildID, status, language, image string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrBuildID.String(buildID),
		AttrBuildStatus.String(status),
		AttrBuildLanguage.String(language),
		AttrBuildImage.String(image),
	}
}

// InfrastructureSpanAttributes returns common attributes for infrastructure spans
func InfrastructureSpanAttributes(provider, region, cluster string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrCloudProvider.String(provider),
		AttrCloudRegion.String(region),
		AttrClusterName.String(cluster),
	}
}

// JobSpanAttributes returns common attributes for job spans
func JobSpanAttributes(queueName, jobID, jobType string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrQueueName.String(queueName),
		AttrJobID.String(jobID),
		AttrJobType.String(jobType),
	}
}

// Global tracer instance
var globalTracer *Tracer

// InitGlobalTracer initializes the global tracer
func InitGlobalTracer(ctx context.Context, config TracingConfig) error {
	tracer, err := NewTracer(ctx, config)
	if err != nil {
		return err
	}
	globalTracer = tracer
	return nil
}

// GetGlobalTracer returns the global tracer instance
func GetGlobalTracer() *Tracer {
	if globalTracer == nil {
		// Return a no-op tracer if not initialized
		return &Tracer{
			tracer: otel.Tracer("app-deployer"),
			config: DefaultTracingConfig(),
		}
	}
	return globalTracer
}

// ShutdownGlobalTracer shuts down the global tracer
func ShutdownGlobalTracer(ctx context.Context) error {
	if globalTracer != nil {
		return globalTracer.Shutdown(ctx)
	}
	return nil
}
