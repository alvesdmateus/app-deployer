package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestNewMetrics(t *testing.T) {
	// Reset the default registry for clean test
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	metrics := NewMetrics("test")

	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics.DeploymentsTotal)
	assert.NotNil(t, metrics.DeploymentsActive)
	assert.NotNil(t, metrics.DeploymentDuration)
	assert.NotNil(t, metrics.BuildsTotal)
	assert.NotNil(t, metrics.BuildDuration)
	assert.NotNil(t, metrics.HTTPRequestsTotal)
	assert.NotNil(t, metrics.HTTPRequestDuration)
	assert.NotNil(t, metrics.QueueDepth)
	assert.NotNil(t, metrics.ScanTotal)
}

func TestMetrics_RecordDeployment(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	metrics := NewMetrics("test_deploy")

	// Should not panic
	metrics.RecordDeployment("success", "gcp", "us-central1")
	metrics.RecordDeployment("failed", "aws", "us-east-1")
}

func TestMetrics_RecordDeploymentDuration(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	metrics := NewMetrics("test_duration")

	// Should not panic
	metrics.RecordDeploymentDuration("build", "success", 120.5)
	metrics.RecordDeploymentDuration("provision", "failed", 300.0)
}

func TestMetrics_ActiveDeployments(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	metrics := NewMetrics("test_active")

	metrics.SetActiveDeployments(10)
	metrics.SetActiveDeployments(5)
}

func TestMetrics_BuildMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	metrics := NewMetrics("test_build")

	metrics.RecordBuild("success", "nodejs")
	metrics.RecordBuildDuration("python", "success", 180.0)
	metrics.IncBuildsInProgress()
	metrics.DecBuildsInProgress()
}

func TestMetrics_ProvisioningMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	metrics := NewMetrics("test_prov")

	metrics.RecordProvisioning("success", "gcp", "create")
	metrics.RecordProvisioningDuration("gcp", "create", "success", 600.0)
}

func TestMetrics_HTTPMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	metrics := NewMetrics("test_http")

	metrics.RecordHTTPRequest("GET", "/api/v1/deployments", "200")
	metrics.RecordHTTPRequestDuration("POST", "/api/v1/deployments", 0.5)
	metrics.IncHTTPRequestsInFlight()
	metrics.DecHTTPRequestsInFlight()
}

func TestMetrics_QueueMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	metrics := NewMetrics("test_queue")

	metrics.SetQueueDepth("deployments", 5)
	metrics.RecordQueueLatency("deployments", "build", 10.5)
	metrics.SetWorkersActive(3)
}

func TestMetrics_ScanMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	metrics := NewMetrics("test_scan")

	metrics.RecordScan("pass")
	metrics.RecordScanDuration("pass", 45.0)
	metrics.SetVulnerabilitiesFound("critical", 0)
	metrics.SetVulnerabilitiesFound("high", 3)
}

func TestMetrics_DefaultNamespace(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	// Empty namespace should default to "app_deployer"
	metrics := NewMetrics("")
	assert.NotNil(t, metrics)
}
