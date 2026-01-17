package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alvesdmateus/app-deployer/internal/observability"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// MetricsMiddleware creates a middleware that records HTTP request metrics
func MetricsMiddleware(metrics *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Track in-flight requests
			metrics.IncHTTPRequestsInFlight()
			defer metrics.DecHTTPRequestsInFlight()

			// Wrap response writer to capture status code
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Process request
			next.ServeHTTP(ww, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			statusCode := strconv.Itoa(ww.Status())
			path := normalizePath(r)

			metrics.RecordHTTPRequest(r.Method, path, statusCode)
			metrics.RecordHTTPRequestDuration(r.Method, path, duration)
		})
	}
}

// normalizePath normalizes the request path for metrics
// This prevents high cardinality by replacing dynamic segments with placeholders
func normalizePath(r *http.Request) string {
	// Try to get the route pattern from chi
	rctx := chi.RouteContext(r.Context())
	if rctx != nil && rctx.RoutePattern() != "" {
		return rctx.RoutePattern()
	}

	// Fallback: manually normalize common patterns
	path := r.URL.Path

	// Normalize deployment IDs (UUIDs)
	path = normalizeUUIDs(path)

	// Normalize build IDs
	path = normalizeSegments(path, "/builds/", "/builds/{buildID}")
	path = normalizeSegments(path, "/deployments/", "/deployments/{id}")

	return path
}

// normalizeUUIDs replaces UUID patterns with a placeholder
func normalizeUUIDs(path string) string {
	// UUID pattern: 8-4-4-4-12 hex characters
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if isUUID(segment) {
			segments[i] = "{id}"
		}
	}
	return strings.Join(segments, "/")
}

// isUUID checks if a string looks like a UUID
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	// Check format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else {
			if !isHexChar(byte(c)) {
				return false
			}
		}
	}
	return true
}

// isHexChar checks if a character is a hex digit
func isHexChar(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// normalizeSegments normalizes path segments after a prefix
func normalizeSegments(path, prefix, replacement string) string {
	idx := strings.Index(path, prefix)
	if idx == -1 {
		return path
	}

	// Find the end of the dynamic segment
	start := idx + len(prefix)
	end := strings.Index(path[start:], "/")
	if end == -1 {
		end = len(path) - start
	}

	// Check if segment looks like an ID
	segment := path[start : start+end]
	if isUUID(segment) || isNumeric(segment) {
		return path[:idx] + replacement + path[start+end:]
	}

	return path
}

// isNumeric checks if a string is all digits
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
