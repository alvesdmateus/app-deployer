package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alvesdmateus/app-deployer/internal/observability"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware creates a middleware that adds distributed tracing to HTTP requests
func TracingMiddleware(tracer *observability.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from incoming request headers
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Get the route pattern for span naming
			routePattern := getRoutePattern(r)
			spanName := fmt.Sprintf("%s %s", r.Method, routePattern)

			// Start a new span
			ctx, span := tracer.StartSpan(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPMethod(r.Method),
					semconv.HTTPScheme(getScheme(r)),
					semconv.HTTPTarget(r.URL.Path),
					semconv.HTTPRoute(routePattern),
					semconv.NetHostName(r.Host),
					semconv.UserAgentOriginal(r.UserAgent()),
					attribute.String("http.client_ip", getClientIP(r)),
				),
			)
			defer span.End()

			// Create a response wrapper to capture status code
			wrapper := &tracingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Process request with traced context
			next.ServeHTTP(wrapper, r.WithContext(ctx))

			// Record response attributes
			span.SetAttributes(
				semconv.HTTPStatusCode(wrapper.statusCode),
				attribute.Int("http.response_content_length", wrapper.bytesWritten),
			)

			// Mark span as error if status >= 500
			if wrapper.statusCode >= 500 {
				span.SetAttributes(attribute.Bool("error", true))
			}
		})
	}
}

// tracingResponseWriter wraps http.ResponseWriter to capture status code
type tracingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (w *tracingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *tracingResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += n
	return n, err
}

// Implement http.Flusher if the underlying ResponseWriter supports it
func (w *tracingResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// getRoutePattern extracts the route pattern from chi router context
func getRoutePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx != nil && rctx.RoutePattern() != "" {
		return rctx.RoutePattern()
	}
	// Fallback to normalized path
	return normalizePath(r)
}

// getScheme returns the request scheme (http or https)
func getScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	// Check for proxy headers
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// InjectTraceContext injects trace context into outgoing HTTP request headers
func InjectTraceContext(ctx context.Context, req *http.Request) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// TraceHTTPClient wraps an HTTP client transport with tracing
func TraceHTTPClient(client *http.Client, tracer *observability.Tracer) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	client.Transport = &tracingTransport{
		base:   transport,
		tracer: tracer,
	}
	return client
}

// tracingTransport wraps an http.RoundTripper with tracing
type tracingTransport struct {
	base   http.RoundTripper
	tracer *observability.Tracer
}

func (t *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	spanName := fmt.Sprintf("HTTP %s %s", req.Method, req.URL.Host)

	ctx, span := t.tracer.StartSpan(req.Context(), spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.HTTPMethod(req.Method),
			semconv.HTTPScheme(req.URL.Scheme),
			semconv.HTTPURL(req.URL.String()),
			semconv.NetPeerName(req.URL.Host),
		),
	)
	defer span.End()

	// Inject trace context into outgoing request headers
	InjectTraceContext(ctx, req)

	// Perform the request
	resp, err := t.base.RoundTrip(req.WithContext(ctx))
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("error", true))
		return nil, err
	}

	// Record response attributes
	span.SetAttributes(
		semconv.HTTPStatusCode(resp.StatusCode),
	)

	if resp.StatusCode >= 400 {
		span.SetAttributes(attribute.Bool("error", true))
	}

	return resp, nil
}

// context key type for request-scoped values
type contextKey string

const (
	// TraceIDKey is the context key for trace ID
	TraceIDKey contextKey = "trace_id"
	// SpanIDKey is the context key for span ID
	SpanIDKey contextKey = "span_id"
)

// GetTraceID extracts the trace ID from the context
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// GetSpanID extracts the span ID from the context
func GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// AddTraceIDToResponse adds the trace ID to response headers for debugging
func AddTraceIDToResponse(w http.ResponseWriter, ctx context.Context) {
	traceID := GetTraceID(ctx)
	if traceID != "" {
		w.Header().Set("X-Trace-ID", traceID)
	}
}
