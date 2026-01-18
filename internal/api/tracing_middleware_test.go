package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alvesdmateus/app-deployer/internal/observability"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracingMiddleware(t *testing.T) {
	// Create a disabled tracer for testing
	config := observability.TracingConfig{
		Enabled:     false,
		ServiceName: "test-service",
	}
	tracer, err := observability.NewTracer(context.Background(), config)
	require.NoError(t, err)

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with tracing middleware
	wrapped := TracingMiddleware(tracer)(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	rec := httptest.NewRecorder()

	// Execute
	wrapped.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

func TestTracingMiddleware_WithError(t *testing.T) {
	config := observability.TracingConfig{
		Enabled:     false,
		ServiceName: "test-service",
	}
	tracer, err := observability.NewTracer(context.Background(), config)
	require.NoError(t, err)

	// Create handler that returns 500
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})

	wrapped := TracingMiddleware(tracer)(handler)

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTracingMiddleware_WithChiRouter(t *testing.T) {
	config := observability.TracingConfig{
		Enabled:     false,
		ServiceName: "test-service",
	}
	tracer, err := observability.NewTracer(context.Background(), config)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(TracingMiddleware(tracer))
	r.Get("/api/v1/deployments/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/api/v1/deployments/123", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTracingResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	wrapper := &tracingResponseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
	}

	// Test WriteHeader
	wrapper.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, wrapper.statusCode)

	// Test Write
	n, err := wrapper.Write([]byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, 4, wrapper.bytesWritten)

	// Test Flush
	wrapper.Flush() // Should not panic
}

func TestGetScheme(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*http.Request)
		expected string
	}{
		{
			name:     "default http",
			setup:    func(r *http.Request) {},
			expected: "http",
		},
		{
			name: "X-Forwarded-Proto https",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-Proto", "https")
			},
			expected: "https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			tt.setup(req)
			result := getScheme(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*http.Request)
		expected string
	}{
		{
			name:     "default RemoteAddr",
			setup:    func(r *http.Request) {},
			expected: "192.0.2.1:1234",
		},
		{
			name: "X-Forwarded-For",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "10.0.0.1")
			},
			expected: "10.0.0.1",
		},
		{
			name: "X-Real-IP",
			setup: func(r *http.Request) {
				r.Header.Set("X-Real-IP", "10.0.0.2")
			},
			expected: "10.0.0.2",
		},
		{
			name: "X-Forwarded-For takes precedence",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "10.0.0.1")
				r.Header.Set("X-Real-IP", "10.0.0.2")
			},
			expected: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			tt.setup(req)
			result := getClientIP(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetTraceID(t *testing.T) {
	// Without a valid span, should return empty string
	traceID := GetTraceID(context.Background())
	assert.Empty(t, traceID)
}

func TestGetSpanID(t *testing.T) {
	// Without a valid span, should return empty string
	spanID := GetSpanID(context.Background())
	assert.Empty(t, spanID)
}

func TestAddTraceIDToResponse(t *testing.T) {
	rec := httptest.NewRecorder()

	// Without trace context, should not set header
	AddTraceIDToResponse(rec, context.Background())
	assert.Empty(t, rec.Header().Get("X-Trace-ID"))
}

func TestInjectTraceContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	// Should not panic even without valid trace context
	InjectTraceContext(context.Background(), req)
}

func TestTraceHTTPClient(t *testing.T) {
	config := observability.TracingConfig{
		Enabled:     false,
		ServiceName: "test-service",
	}
	tracer, err := observability.NewTracer(context.Background(), config)
	require.NoError(t, err)

	// Test with nil client
	client := TraceHTTPClient(nil, tracer)
	assert.NotNil(t, client)
	assert.NotNil(t, client.Transport)

	// Test with existing client
	existingClient := &http.Client{}
	tracedClient := TraceHTTPClient(existingClient, tracer)
	assert.NotNil(t, tracedClient.Transport)
}
