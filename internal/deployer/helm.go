package deployer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/alvesdmateus/app-deployer/internal/state"
)

// Default timeout for deployment operations (10 minutes)
const DefaultDeployTimeout = 10 * time.Minute

// HelmDeployer implements Deployer interface using Helm
type HelmDeployer struct {
	tracker         *Tracker
	chartPath       string
	defaultReplicas int
	defaultPort     int
	deployTimeout   time.Duration
}

// Config holds deployer configuration
type Config struct {
	ChartPath       string
	DefaultReplicas int
	DefaultPort     int
	DeployTimeout   time.Duration // Optional: defaults to 10 minutes
}

// NewHelmDeployer creates a new Helm-based deployer
func NewHelmDeployer(config Config, tracker *Tracker) (*HelmDeployer, error) {
	if config.ChartPath == "" {
		config.ChartPath = "templates/helm/base-app"
	}

	if config.DefaultReplicas == 0 {
		config.DefaultReplicas = 2
	}

	if config.DefaultPort == 0 {
		config.DefaultPort = 8080
	}

	if config.DeployTimeout == 0 {
		config.DeployTimeout = DefaultDeployTimeout
	}

	// Verify Helm is installed
	if err := verifyHelmInstalled(); err != nil {
		return nil, err
	}

	log.Info().
		Str("chartPath", config.ChartPath).
		Int("defaultReplicas", config.DefaultReplicas).
		Dur("deployTimeout", config.DeployTimeout).
		Msg("Helm deployer initialized")

	return &HelmDeployer{
		tracker:         tracker,
		chartPath:       config.ChartPath,
		defaultReplicas: config.DefaultReplicas,
		defaultPort:     config.DefaultPort,
		deployTimeout:   config.DeployTimeout,
	}, nil
}

// verifyHelmInstalled checks if Helm CLI is installed
func verifyHelmInstalled() error {
	cmd := exec.Command("helm", "version", "--short")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm CLI not found: %w (please install helm)", err)
	}
	return nil
}

// Deploy deploys an application using Helm
func (h *HelmDeployer) Deploy(ctx context.Context, req *DeployRequest) (*DeployResult, error) {
	startTime := time.Now()

	// Apply timeout to context
	ctx, cancel := context.WithTimeout(ctx, h.deployTimeout)
	defer cancel()

	log.Info().
		Str("deploymentID", req.DeploymentID).
		Str("appName", req.AppName).
		Str("imageTag", req.ImageTag).
		Dur("timeout", h.deployTimeout).
		Msg("Starting Helm deployment")

	// Start deployment tracking
	if err := h.tracker.StartDeployment(ctx, req.InfrastructureID); err != nil {
		return nil, fmt.Errorf("failed to start deployment tracking: %w", err)
	}

	// Get infrastructure details
	infra, err := h.tracker.GetInfrastructure(ctx, req.InfrastructureID)
	if err != nil {
		h.tracker.FailDeployment(ctx, req.InfrastructureID, err)
		return nil, fmt.Errorf("failed to get infrastructure: %w", err)
	}

	// Create Kubernetes client
	kubeClient, err := NewKubeClient(infra)
	if err != nil {
		h.tracker.FailDeployment(ctx, req.InfrastructureID, err)
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Generate namespace and release name
	namespace := infra.Namespace
	if namespace == "" {
		namespace = fmt.Sprintf("deployer-%s", req.DeploymentID[:8])
	}
	releaseName := fmt.Sprintf("app-%s", req.DeploymentID[:8])

	// Create namespace
	labels := map[string]string{
		"app":           req.AppName,
		"deployment-id": req.DeploymentID,
		"managed-by":    "app-deployer",
	}
	if err := kubeClient.CreateNamespace(ctx, namespace, labels); err != nil {
		h.tracker.FailDeployment(ctx, req.InfrastructureID, err)
		return nil, fmt.Errorf("failed to create namespace: %w", err)
	}

	// Generate Helm values
	values, err := h.generateValues(req, infra)
	if err != nil {
		h.tracker.FailDeployment(ctx, req.InfrastructureID, err)
		return nil, fmt.Errorf("failed to generate Helm values: %w", err)
	}

	// Write values to temp file
	valuesFile, err := h.writeValuesFile(values)
	if err != nil {
		h.tracker.FailDeployment(ctx, req.InfrastructureID, err)
		return nil, fmt.Errorf("failed to write values file: %w", err)
	}
	defer os.Remove(valuesFile)

	// Install or upgrade Helm release
	if err := h.installOrUpgrade(ctx, releaseName, namespace, valuesFile, infra); err != nil {
		h.tracker.FailDeployment(ctx, req.InfrastructureID, err)
		return nil, fmt.Errorf("helm install/upgrade failed: %w", err)
	}

	// Wait for pods to be ready
	labelSelector := fmt.Sprintf("app.kubernetes.io/instance=%s", releaseName)
	if err := kubeClient.WaitForPodsReady(ctx, namespace, labelSelector, 5*time.Minute); err != nil {
		h.tracker.FailDeployment(ctx, req.InfrastructureID, err)
		return nil, fmt.Errorf("pods failed to become ready: %w", err)
	}

	// Get LoadBalancer external IP
	serviceName := releaseName
	externalIP, err := kubeClient.GetLoadBalancerIP(ctx, namespace, serviceName, 5*time.Minute)
	if err != nil {
		h.tracker.FailDeployment(ctx, req.InfrastructureID, err)
		return nil, fmt.Errorf("failed to get external IP: %w", err)
	}

	// Build result
	result := &DeployResult{
		ReleaseName: releaseName,
		Namespace:   namespace,
		ExternalIP:  externalIP,
		ExternalURL: fmt.Sprintf("http://%s", externalIP),
		Status:      "deployed",
		Message:     "Application deployed successfully",
		Duration:    time.Since(startTime),
	}

	// Complete deployment tracking
	if err := h.tracker.CompleteDeployment(ctx, req.InfrastructureID, result); err != nil {
		return nil, fmt.Errorf("failed to complete deployment tracking: %w", err)
	}

	log.Info().
		Str("deploymentID", req.DeploymentID).
		Str("releaseName", releaseName).
		Str("externalIP", externalIP).
		Dur("duration", result.Duration).
		Msg("Helm deployment completed successfully")

	return result, nil
}

// Destroy removes a Helm deployment
func (h *HelmDeployer) Destroy(ctx context.Context, req *DestroyRequest) error {
	log.Info().
		Str("deploymentID", req.DeploymentID).
		Str("releaseName", req.ReleaseName).
		Str("namespace", req.Namespace).
		Msg("Destroying Helm deployment")

	// Get infrastructure details
	infra, err := h.tracker.GetInfrastructure(ctx, req.InfrastructureID)
	if err != nil {
		return fmt.Errorf("failed to get infrastructure: %w", err)
	}

	// Create Kubernetes client
	kubeClient, err := NewKubeClient(infra)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Setup kubeconfig environment
	kubeconfigPath, cleanup, err := h.setupKubeconfig(infra)
	if err != nil {
		return fmt.Errorf("failed to setup kubeconfig: %w", err)
	}
	defer cleanup()

	// Uninstall Helm release
	cmd := exec.CommandContext(ctx, "helm", "uninstall", req.ReleaseName, "-n", req.Namespace)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Warn().
			Err(err).
			Str("output", string(output)).
			Msg("Helm uninstall failed (may already be deleted)")
	}

	// Delete namespace
	if err := kubeClient.DeleteNamespace(ctx, req.Namespace); err != nil {
		log.Warn().Err(err).Msg("Failed to delete namespace (may already be deleted)")
	}

	log.Info().
		Str("deploymentID", req.DeploymentID).
		Str("releaseName", req.ReleaseName).
		Msg("Helm deployment destroyed successfully")

	return nil
}

// GetStatus gets the status of a deployment by querying Helm
func (h *HelmDeployer) GetStatus(ctx context.Context, namespace, releaseName string) (*DeploymentStatus, error) {
	log.Debug().
		Str("namespace", namespace).
		Str("release", releaseName).
		Msg("Getting Helm release status")

	// Run helm status command
	cmd := exec.CommandContext(ctx, "helm", "status", releaseName,
		"-n", namespace,
		"-o", "json",
	)

	output, err := cmd.Output()
	if err != nil {
		// Check if release doesn't exist
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "not found") {
				return &DeploymentStatus{
					ReleaseName: releaseName,
					Namespace:   namespace,
					Status:      "not_found",
					UpdatedAt:   time.Now(),
				}, nil
			}
		}
		log.Error().Err(err).Msg("Failed to get Helm status")
		return nil, fmt.Errorf("failed to get helm status: %w", err)
	}

	// Parse JSON output
	var helmStatus struct {
		Name string `json:"name"`
		Info struct {
			Status       string    `json:"status"`
			LastDeployed time.Time `json:"last_deployed"`
			Description  string    `json:"description"`
		} `json:"info"`
		Version int `json:"version"`
	}

	if err := yaml.Unmarshal(output, &helmStatus); err != nil {
		log.Error().Err(err).Msg("Failed to parse Helm status output")
		return nil, fmt.Errorf("failed to parse helm status: %w", err)
	}

	status := &DeploymentStatus{
		ReleaseName: releaseName,
		Namespace:   namespace,
		Status:      helmStatus.Info.Status,
		Revision:    helmStatus.Version,
		UpdatedAt:   helmStatus.Info.LastDeployed,
	}

	log.Debug().
		Str("status", status.Status).
		Int("revision", status.Revision).
		Msg("Got Helm release status")

	return status, nil
}

// Rollback rolls back a deployment
func (h *HelmDeployer) Rollback(ctx context.Context, req *RollbackRequest) error {
	log.Info().
		Str("deploymentID", req.DeploymentID).
		Str("releaseName", req.ReleaseName).
		Int("revision", req.Revision).
		Msg("Rolling back Helm deployment")

	// Get infrastructure
	infra, err := h.tracker.GetInfrastructure(ctx, req.InfrastructureID)
	if err != nil {
		return fmt.Errorf("failed to get infrastructure: %w", err)
	}

	// Setup kubeconfig
	kubeconfigPath, cleanup, err := h.setupKubeconfig(infra)
	if err != nil {
		return fmt.Errorf("failed to setup kubeconfig: %w", err)
	}
	defer cleanup()

	// Rollback using Helm
	args := []string{"rollback", req.ReleaseName, "-n", req.Namespace}
	if req.Revision > 0 {
		args = append(args, fmt.Sprintf("%d", req.Revision))
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm rollback failed: %w, output: %s", err, string(output))
	}

	log.Info().
		Str("deploymentID", req.DeploymentID).
		Str("releaseName", req.ReleaseName).
		Msg("Rollback completed successfully")

	return nil
}

// generateValues generates Helm values from deployment request
func (h *HelmDeployer) generateValues(req *DeployRequest, infra *state.Infrastructure) (map[string]interface{}, error) {
	replicas := req.Replicas
	if replicas == 0 {
		replicas = h.defaultReplicas
	}

	port := req.Port
	if port == 0 {
		port = h.defaultPort
	}

	// Parse image tag safely
	imageRepo := req.ImageTag
	imageTag := "latest"
	if parts := strings.SplitN(req.ImageTag, ":", 2); len(parts) == 2 {
		imageRepo = parts[0]
		imageTag = parts[1]
	}

	// Determine service type
	serviceType := "LoadBalancer"
	if req.Config != nil && req.Config.ServiceType != "" {
		serviceType = req.Config.ServiceType
	}

	values := map[string]interface{}{
		"image": map[string]interface{}{
			"repository": imageRepo,
			"tag":        imageTag,
			"pullPolicy": "Always",
		},
		"replicaCount": replicas,
		"service": map[string]interface{}{
			"type":       serviceType,
			"port":       port,
			"targetPort": port,
		},
		"labels": map[string]interface{}{
			"app":           req.AppName,
			"deployment-id": req.DeploymentID,
			"managed-by":    "app-deployer",
		},
	}

	// Add resource limits/requests if provided
	resources := make(map[string]interface{})
	limits := make(map[string]interface{})
	requests := make(map[string]interface{})

	if req.CPULimit != "" {
		limits["cpu"] = req.CPULimit
	} else {
		limits["cpu"] = "1000m"
	}
	if req.MemoryLimit != "" {
		limits["memory"] = req.MemoryLimit
	} else {
		limits["memory"] = "1Gi"
	}
	if req.CPURequest != "" {
		requests["cpu"] = req.CPURequest
	} else {
		requests["cpu"] = "100m"
	}
	if req.MemoryRequest != "" {
		requests["memory"] = req.MemoryRequest
	} else {
		requests["memory"] = "128Mi"
	}

	resources["limits"] = limits
	resources["requests"] = requests
	values["resources"] = resources

	// Configure health checks
	healthCheckPath := "/"
	if req.Config != nil && req.Config.HealthCheckPath != "" {
		healthCheckPath = req.Config.HealthCheckPath
	}

	readinessPath := healthCheckPath
	if req.Config != nil && req.Config.ReadinessPath != "" {
		readinessPath = req.Config.ReadinessPath
	}

	livenessPath := healthCheckPath
	if req.Config != nil && req.Config.LivenessPath != "" {
		livenessPath = req.Config.LivenessPath
	}

	values["healthCheck"] = map[string]interface{}{
		"enabled": true,
		"livenessProbe": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": livenessPath,
				"port": "http",
			},
			"initialDelaySeconds": 30,
			"periodSeconds":       10,
			"timeoutSeconds":      5,
			"failureThreshold":    3,
		},
		"readinessProbe": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": readinessPath,
				"port": "http",
			},
			"initialDelaySeconds": 10,
			"periodSeconds":       5,
			"timeoutSeconds":      3,
			"failureThreshold":    3,
		},
	}

	// Configure autoscaling if enabled
	if req.Config != nil && req.Config.EnableHPA {
		minReplicas := req.Config.MinReplicas
		if minReplicas == 0 {
			minReplicas = 1
		}
		maxReplicas := req.Config.MaxReplicas
		if maxReplicas == 0 {
			maxReplicas = 10
		}
		targetCPU := req.Config.TargetCPU
		if targetCPU == 0 {
			targetCPU = 80
		}

		values["autoscaling"] = map[string]interface{}{
			"enabled":                        true,
			"minReplicas":                    minReplicas,
			"maxReplicas":                    maxReplicas,
			"targetCPUUtilizationPercentage": targetCPU,
		}

		if req.Config.TargetMemory > 0 {
			values["autoscaling"].(map[string]interface{})["targetMemoryUtilizationPercentage"] = req.Config.TargetMemory
		}
	}

	// Configure ingress if enabled
	if req.Config != nil && req.Config.EnableIngress && req.Config.IngressHost != "" {
		ingressConfig := map[string]interface{}{
			"enabled":   true,
			"className": "nginx",
			"hosts": []map[string]interface{}{
				{
					"host": req.Config.IngressHost,
					"paths": []map[string]interface{}{
						{
							"path":     "/",
							"pathType": "Prefix",
						},
					},
				},
			},
		}

		if req.Config.IngressTLS {
			ingressConfig["tls"] = []map[string]interface{}{
				{
					"secretName": fmt.Sprintf("%s-tls", req.AppName),
					"hosts":      []string{req.Config.IngressHost},
				},
			}
		}

		values["ingress"] = ingressConfig
	}

	// Add environment variables if provided
	if len(req.Env) > 0 {
		envVars := make([]map[string]interface{}, 0, len(req.Env))
		for k, v := range req.Env {
			envVars = append(envVars, map[string]interface{}{
				"name":  k,
				"value": v,
			})
		}
		values["env"] = envVars
	}

	// Add custom labels if provided
	if req.Config != nil && len(req.Config.Labels) > 0 {
		for k, v := range req.Config.Labels {
			values["labels"].(map[string]interface{})[k] = v
		}
	}

	return values, nil
}

// writeValuesFile writes values to a temporary YAML file
func (h *HelmDeployer) writeValuesFile(values map[string]interface{}) (string, error) {
	tmpDir := os.TempDir()
	valuesFile := filepath.Join(tmpDir, fmt.Sprintf("helm-values-%d.yaml", time.Now().UnixNano()))

	data, err := yaml.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("failed to marshal values: %w", err)
	}

	if err := os.WriteFile(valuesFile, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write values file: %w", err)
	}

	return valuesFile, nil
}

// installOrUpgrade installs or upgrades a Helm release
func (h *HelmDeployer) installOrUpgrade(ctx context.Context, releaseName, namespace, valuesFile string, infra *state.Infrastructure) error {
	log.Info().
		Str("releaseName", releaseName).
		Str("namespace", namespace).
		Msg("Installing/upgrading Helm release")

	// Setup kubeconfig
	kubeconfigPath, cleanup, err := h.setupKubeconfig(infra)
	if err != nil {
		return fmt.Errorf("failed to setup kubeconfig: %w", err)
	}
	defer cleanup()

	// Helm upgrade --install command
	cmd := exec.CommandContext(ctx, "helm", "upgrade",
		releaseName,
		h.chartPath,
		"--install",
		"--create-namespace",
		"-n", namespace,
		"-f", valuesFile,
		"--wait",
		"--timeout", "10m",
	)

	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))

	output, err := cmd.CombinedOutput()
	log.Debug().Str("output", string(output)).Msg("Helm output")

	if err != nil {
		return fmt.Errorf("helm install/upgrade failed: %w, output: %s", err, string(output))
	}

	return nil
}

// setupKubeconfig creates a temporary kubeconfig file and returns cleanup function
func (h *HelmDeployer) setupKubeconfig(infra *state.Infrastructure) (string, func(), error) {
	// Create Kubernetes client to verify connectivity
	_, err := NewKubeClient(infra)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// For now, we'll use gcloud to get kubeconfig
	// This is a simplified approach - in production, you'd generate the kubeconfig directly
	tmpDir := os.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, fmt.Sprintf("kubeconfig-%d", time.Now().UnixNano()))

	// Get GCP project from infrastructure, fall back to environment variable
	gcpProject := infra.GCPProject
	if gcpProject == "" {
		gcpProject = os.Getenv("GCP_PROJECT")
	}
	if gcpProject == "" {
		return "", nil, fmt.Errorf("GCP project not found in infrastructure or environment")
	}

	// Use gcloud to get cluster credentials
	cmd := exec.Command("gcloud", "container", "clusters", "get-credentials",
		infra.ClusterName,
		"--region", infra.ClusterLocation,
		"--project", gcpProject,
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get cluster credentials: %w, output: %s", err, string(output))
	}

	cleanup := func() {
		os.Remove(kubeconfigPath)
	}

	return kubeconfigPath, cleanup, nil
}
