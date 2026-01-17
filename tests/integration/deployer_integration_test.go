//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestDeployer_KindClusterAvailable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipDeployer {
		t.Skip("Deployer tests are disabled")
	}

	if !cfg.KindAvailable {
		t.Skipf("Kind cluster '%s' is not available", cfg.KindClusterName)
	}

	// Verify kubectl can connect to the cluster
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "cluster-info")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to connect to cluster: %v\nOutput: %s", err, string(output))
	}

	t.Logf("Connected to Kubernetes cluster:\n%s", string(output))
}

func TestDeployer_HelmAvailable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipDeployer {
		t.Skip("Deployer tests are disabled")
	}

	// Check if Helm is installed
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "helm", "version", "--short")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("Helm is not available: %v", err)
	}

	t.Logf("Helm version: %s", strings.TrimSpace(string(output)))
}

func TestDeployer_CreateNamespace(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipDeployer {
		t.Skip("Deployer tests are disabled")
	}
	RequireKind(t, cfg)

	namespace := "deployer-integration-test"

	// Create namespace
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	createCmd := exec.CommandContext(ctx, "kubectl", "create", "namespace", namespace, "--dry-run=client", "-o", "yaml")
	output, err := createCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to generate namespace YAML: %v\nOutput: %s", err, string(output))
	}

	// Verify the YAML is valid
	if !strings.Contains(string(output), "kind: Namespace") {
		t.Errorf("Expected namespace YAML, got: %s", string(output))
	}

	t.Logf("Namespace YAML generated successfully")
}

func TestDeployer_HelmChartTemplate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipDeployer {
		t.Skip("Deployer tests are disabled")
	}

	// Check if the base-app chart exists
	chartPath := "templates/helm/base-app"
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		// Try from repository root
		chartPath = "../../templates/helm/base-app"
		if _, err := os.Stat(chartPath); os.IsNotExist(err) {
			t.Skip("Helm chart not found, skipping test")
		}
	}

	// Template the chart with test values
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "helm", "template", "test-release", chartPath,
		"--set", "image.repository=nginx",
		"--set", "image.tag=alpine",
		"--set", "service.port=80",
		"--set", "replicaCount=1",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Helm template failed: %v\nOutput: %s", err, string(output))
	}

	// Verify the output contains expected resources
	outputStr := string(output)
	expectedResources := []string{"Deployment", "Service"}
	for _, resource := range expectedResources {
		if !strings.Contains(outputStr, resource) {
			t.Errorf("Expected %s in template output", resource)
		}
	}

	t.Logf("Helm chart templating successful")
}

func TestDeployer_HelmDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipDeployer {
		t.Skip("Deployer tests are disabled")
	}
	RequireKind(t, cfg)

	// Check if the base-app chart exists
	chartPath := "templates/helm/base-app"
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		chartPath = "../../templates/helm/base-app"
		if _, err := os.Stat(chartPath); os.IsNotExist(err) {
			t.Skip("Helm chart not found, skipping test")
		}
	}

	namespace := "default"
	releaseName := fmt.Sprintf("test-dry-run-%d", time.Now().Unix())

	// Perform dry-run install
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "helm", "install", releaseName, chartPath,
		"--namespace", namespace,
		"--dry-run",
		"--set", "image.repository=nginx",
		"--set", "image.tag=alpine",
		"--set", "service.port=80",
		"--set", "replicaCount=1",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Helm dry-run failed: %v\nOutput: %s", err, string(output))
	}

	// Verify successful dry-run
	if !strings.Contains(string(output), "NAME:") {
		t.Errorf("Expected successful dry-run output")
	}

	t.Logf("Helm dry-run install successful")
}

func TestDeployer_FullDeploymentCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipDeployer {
		t.Skip("Deployer tests are disabled")
	}
	RequireKind(t, cfg)

	// Check if the base-app chart exists
	chartPath := "templates/helm/base-app"
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		chartPath = "../../templates/helm/base-app"
		if _, err := os.Stat(chartPath); os.IsNotExist(err) {
			t.Skip("Helm chart not found, skipping test")
		}
	}

	namespace := "deployer-test"
	releaseName := fmt.Sprintf("integration-test-%d", time.Now().Unix())

	// Create namespace
	ctx := context.Background()
	createNsCmd := exec.CommandContext(ctx, "kubectl", "create", "namespace", namespace)
	createNsCmd.Run() // Ignore error if namespace exists

	// Cleanup function
	cleanup := func() {
		uninstallCmd := exec.Command("helm", "uninstall", releaseName, "--namespace", namespace)
		uninstallCmd.Run()

		deleteNsCmd := exec.Command("kubectl", "delete", "namespace", namespace)
		deleteNsCmd.Run()
	}
	defer cleanup()

	// Step 1: Install
	t.Log("Step 1: Installing Helm release...")
	installCtx, installCancel := context.WithTimeout(ctx, cfg.DeployTimeout)
	defer installCancel()

	installCmd := exec.CommandContext(installCtx, "helm", "install", releaseName, chartPath,
		"--namespace", namespace,
		"--wait",
		"--timeout", "2m",
		"--set", "image.repository=nginx",
		"--set", "image.tag=alpine",
		"--set", "service.port=80",
		"--set", "service.type=ClusterIP",
		"--set", "replicaCount=1",
	)

	installOutput, err := installCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Helm install failed: %v\nOutput: %s", err, string(installOutput))
	}
	t.Log("Helm install successful")

	// Step 2: Verify deployment
	t.Log("Step 2: Verifying deployment...")
	verifyCtx, verifyCancel := context.WithTimeout(ctx, 60*time.Second)
	defer verifyCancel()

	verifyCmd := exec.CommandContext(verifyCtx, "kubectl", "get", "deployment",
		"-n", namespace, "-l", fmt.Sprintf("app.kubernetes.io/instance=%s", releaseName),
		"-o", "jsonpath={.items[0].status.readyReplicas}")

	verifyOutput, err := verifyCmd.CombinedOutput()
	if err != nil {
		t.Logf("Warning: Could not verify deployment: %v", err)
	} else {
		t.Logf("Ready replicas: %s", string(verifyOutput))
	}

	// Step 3: Upgrade
	t.Log("Step 3: Upgrading Helm release...")
	upgradeCtx, upgradeCancel := context.WithTimeout(ctx, cfg.DeployTimeout)
	defer upgradeCancel()

	upgradeCmd := exec.CommandContext(upgradeCtx, "helm", "upgrade", releaseName, chartPath,
		"--namespace", namespace,
		"--wait",
		"--timeout", "2m",
		"--set", "image.repository=nginx",
		"--set", "image.tag=alpine",
		"--set", "service.port=80",
		"--set", "service.type=ClusterIP",
		"--set", "replicaCount=2", // Upgrade to 2 replicas
	)

	upgradeOutput, err := upgradeCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Helm upgrade failed: %v\nOutput: %s", err, string(upgradeOutput))
	}
	t.Log("Helm upgrade successful")

	// Step 4: Rollback
	t.Log("Step 4: Rolling back Helm release...")
	rollbackCtx, rollbackCancel := context.WithTimeout(ctx, cfg.DeployTimeout)
	defer rollbackCancel()

	rollbackCmd := exec.CommandContext(rollbackCtx, "helm", "rollback", releaseName, "1",
		"--namespace", namespace,
		"--wait",
		"--timeout", "2m",
	)

	rollbackOutput, err := rollbackCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Helm rollback failed: %v\nOutput: %s", err, string(rollbackOutput))
	}
	t.Log("Helm rollback successful")

	// Step 5: Get status
	t.Log("Step 5: Getting release status...")
	statusCmd := exec.Command("helm", "status", releaseName, "--namespace", namespace)
	statusOutput, err := statusCmd.CombinedOutput()
	if err != nil {
		t.Logf("Warning: Could not get status: %v", err)
	} else {
		t.Logf("Release status:\n%s", string(statusOutput))
	}

	// Step 6: Uninstall (cleanup will handle this)
	t.Log("Step 6: Test completed, cleaning up...")
}

func TestDeployer_GetPodStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipDeployer {
		t.Skip("Deployer tests are disabled")
	}
	RequireKind(t, cfg)

	// Get pods in kube-system namespace (should always exist)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "-n", "kube-system",
		"-o", "jsonpath={range .items[*]}{.metadata.name}{' '}{.status.phase}{'\\n'}{end}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get pods: %v\nOutput: %s", err, string(output))
	}

	// Verify some pods are running
	if !strings.Contains(string(output), "Running") {
		t.Errorf("Expected some pods to be Running, got: %s", string(output))
	}

	t.Logf("Pod status check successful")
}

func TestDeployer_ServiceExposure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipDeployer {
		t.Skip("Deployer tests are disabled")
	}
	RequireKind(t, cfg)

	// Get services in kube-system namespace
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "get", "svc", "-n", "kube-system",
		"-o", "jsonpath={range .items[*]}{.metadata.name}{' '}{.spec.type}{'\\n'}{end}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get services: %v\nOutput: %s", err, string(output))
	}

	t.Logf("Services in kube-system:\n%s", string(output))
}
