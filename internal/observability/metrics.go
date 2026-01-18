package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the application
type Metrics struct {
	// Deployment metrics
	DeploymentsTotal    *prometheus.CounterVec
	DeploymentsActive   prometheus.Gauge
	DeploymentDuration  *prometheus.HistogramVec
	DeploymentsByStatus *prometheus.GaugeVec

	// Build metrics
	BuildsTotal      *prometheus.CounterVec
	BuildDuration    *prometheus.HistogramVec
	BuildsInProgress prometheus.Gauge

	// Infrastructure metrics
	ProvisioningTotal    *prometheus.CounterVec
	ProvisioningDuration *prometheus.HistogramVec

	// API metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge

	// Queue metrics
	QueueDepth    *prometheus.GaugeVec
	QueueLatency  *prometheus.HistogramVec
	WorkersActive prometheus.Gauge

	// Scanner metrics
	ScanTotal            *prometheus.CounterVec
	ScanDuration         *prometheus.HistogramVec
	VulnerabilitiesFound *prometheus.GaugeVec
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics(namespace string) *Metrics {
	if namespace == "" {
		namespace = "app_deployer"
	}

	m := &Metrics{
		// Deployment metrics
		DeploymentsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "deployments_total",
				Help:      "Total number of deployments processed",
			},
			[]string{"status", "cloud", "region"},
		),
		DeploymentsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "deployments_active",
				Help:      "Number of currently active deployments",
			},
		),
		DeploymentDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "deployment_duration_seconds",
				Help:      "Time taken for deployment operations",
				Buckets:   []float64{30, 60, 120, 300, 600, 900, 1200, 1800},
			},
			[]string{"phase", "status"},
		),
		DeploymentsByStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "deployments_by_status",
				Help:      "Number of deployments by status",
			},
			[]string{"status"},
		),

		// Build metrics
		BuildsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "builds_total",
				Help:      "Total number of builds processed",
			},
			[]string{"status", "language"},
		),
		BuildDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "build_duration_seconds",
				Help:      "Time taken for build operations",
				Buckets:   []float64{30, 60, 120, 300, 600, 900, 1200, 1800},
			},
			[]string{"language", "status"},
		),
		BuildsInProgress: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "builds_in_progress",
				Help:      "Number of builds currently in progress",
			},
		),

		// Infrastructure metrics
		ProvisioningTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "provisioning_total",
				Help:      "Total number of infrastructure provisioning operations",
			},
			[]string{"status", "cloud", "operation"},
		),
		ProvisioningDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "provisioning_duration_seconds",
				Help:      "Time taken for infrastructure provisioning",
				Buckets:   []float64{60, 120, 300, 600, 900, 1200, 1800, 2400, 3600},
			},
			[]string{"cloud", "operation", "status"},
		),

		// API metrics
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request latency",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		HTTPRequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "http_requests_in_flight",
				Help:      "Number of HTTP requests currently being processed",
			},
		),

		// Queue metrics
		QueueDepth: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "queue_depth",
				Help:      "Number of jobs in the queue",
			},
			[]string{"queue"},
		),
		QueueLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "queue_latency_seconds",
				Help:      "Time jobs spend waiting in queue",
				Buckets:   []float64{1, 5, 10, 30, 60, 120, 300},
			},
			[]string{"queue", "job_type"},
		),
		WorkersActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "workers_active",
				Help:      "Number of active worker goroutines",
			},
		),

		// Scanner metrics
		ScanTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "scans_total",
				Help:      "Total number of vulnerability scans",
			},
			[]string{"status"},
		),
		ScanDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "scan_duration_seconds",
				Help:      "Time taken for vulnerability scans",
				Buckets:   []float64{10, 30, 60, 120, 300, 600},
			},
			[]string{"status"},
		),
		VulnerabilitiesFound: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "vulnerabilities_found",
				Help:      "Number of vulnerabilities found in last scan",
			},
			[]string{"severity"},
		),
	}

	return m
}

// RecordDeployment records a deployment operation
func (m *Metrics) RecordDeployment(status, cloud, region string) {
	m.DeploymentsTotal.WithLabelValues(status, cloud, region).Inc()
}

// RecordDeploymentDuration records deployment phase duration
func (m *Metrics) RecordDeploymentDuration(phase, status string, seconds float64) {
	m.DeploymentDuration.WithLabelValues(phase, status).Observe(seconds)
}

// SetActiveDeployments sets the number of active deployments
func (m *Metrics) SetActiveDeployments(count float64) {
	m.DeploymentsActive.Set(count)
}

// SetDeploymentsByStatus sets the count for a specific status
func (m *Metrics) SetDeploymentsByStatus(status string, count float64) {
	m.DeploymentsByStatus.WithLabelValues(status).Set(count)
}

// RecordBuild records a build operation
func (m *Metrics) RecordBuild(status, language string) {
	m.BuildsTotal.WithLabelValues(status, language).Inc()
}

// RecordBuildDuration records build duration
func (m *Metrics) RecordBuildDuration(language, status string, seconds float64) {
	m.BuildDuration.WithLabelValues(language, status).Observe(seconds)
}

// IncBuildsInProgress increments builds in progress
func (m *Metrics) IncBuildsInProgress() {
	m.BuildsInProgress.Inc()
}

// DecBuildsInProgress decrements builds in progress
func (m *Metrics) DecBuildsInProgress() {
	m.BuildsInProgress.Dec()
}

// RecordProvisioning records an infrastructure provisioning operation
func (m *Metrics) RecordProvisioning(status, cloud, operation string) {
	m.ProvisioningTotal.WithLabelValues(status, cloud, operation).Inc()
}

// RecordProvisioningDuration records provisioning duration
func (m *Metrics) RecordProvisioningDuration(cloud, operation, status string, seconds float64) {
	m.ProvisioningDuration.WithLabelValues(cloud, operation, status).Observe(seconds)
}

// RecordHTTPRequest records an HTTP request
func (m *Metrics) RecordHTTPRequest(method, path, statusCode string) {
	m.HTTPRequestsTotal.WithLabelValues(method, path, statusCode).Inc()
}

// RecordHTTPRequestDuration records HTTP request duration
func (m *Metrics) RecordHTTPRequestDuration(method, path string, seconds float64) {
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(seconds)
}

// IncHTTPRequestsInFlight increments in-flight requests
func (m *Metrics) IncHTTPRequestsInFlight() {
	m.HTTPRequestsInFlight.Inc()
}

// DecHTTPRequestsInFlight decrements in-flight requests
func (m *Metrics) DecHTTPRequestsInFlight() {
	m.HTTPRequestsInFlight.Dec()
}

// SetQueueDepth sets the queue depth for a specific queue
func (m *Metrics) SetQueueDepth(queue string, depth float64) {
	m.QueueDepth.WithLabelValues(queue).Set(depth)
}

// RecordQueueLatency records job queue latency
func (m *Metrics) RecordQueueLatency(queue, jobType string, seconds float64) {
	m.QueueLatency.WithLabelValues(queue, jobType).Observe(seconds)
}

// SetWorkersActive sets the number of active workers
func (m *Metrics) SetWorkersActive(count float64) {
	m.WorkersActive.Set(count)
}

// RecordScan records a vulnerability scan
func (m *Metrics) RecordScan(status string) {
	m.ScanTotal.WithLabelValues(status).Inc()
}

// RecordScanDuration records scan duration
func (m *Metrics) RecordScanDuration(status string, seconds float64) {
	m.ScanDuration.WithLabelValues(status).Observe(seconds)
}

// SetVulnerabilitiesFound sets vulnerability counts by severity
func (m *Metrics) SetVulnerabilitiesFound(severity string, count float64) {
	m.VulnerabilitiesFound.WithLabelValues(severity).Set(count)
}

// Global metrics instance
var DefaultMetrics = NewMetrics("")
