package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/rs/zerolog/log"
)

// PlatformConfig holds configuration for the entire platform stack
type PlatformConfig struct {
	// StackName is the name of the Pulumi stack
	StackName string
	// BackendURL is the Pulumi backend URL
	BackendURL string
	// Kubeconfig is the path to kubeconfig file or the content itself
	Kubeconfig string
	// KubeContext is the kubeconfig context to use
	KubeContext string
	// ClusterEndpoint is the Kubernetes API server endpoint
	ClusterEndpoint string
	// ClusterCACert is the cluster CA certificate
	ClusterCACert string
	// ClusterToken is the authentication token
	ClusterToken string
	// Istio configuration
	Istio IstioConfig
	// ArgoCD configuration
	ArgoCD ArgoCDConfig
	// AppDeployer configuration
	AppDeployer AppDeployerConfig
	// EnableIstio enables Istio installation
	EnableIstio bool
	// EnableArgoCD enables ArgoCD installation
	EnableArgoCD bool
	// ProvisionTimeout is the timeout for provisioning
	ProvisionTimeout time.Duration
}

// AppDeployerConfig holds configuration for app-deployer deployment
type AppDeployerConfig struct {
	// Namespace is the namespace for app-deployer
	Namespace string
	// Replicas is the number of API replicas
	Replicas int
	// Image is the container image
	Image string
	// ImageTag is the image tag
	ImageTag string
	// EnableIstioSidecar enables Istio sidecar injection
	EnableIstioSidecar bool
	// DatabaseURL is the PostgreSQL connection string
	DatabaseURL string
	// RedisURL is the Redis connection string
	RedisURL string
}

// DefaultPlatformConfig returns default platform configuration
func DefaultPlatformConfig() PlatformConfig {
	return PlatformConfig{
		StackName:        "app-deployer-platform",
		EnableIstio:      true,
		EnableArgoCD:     true,
		Istio:            DefaultIstioConfig(),
		ArgoCD:           DefaultArgoCDConfig(),
		ProvisionTimeout: 30 * time.Minute,
		AppDeployer: AppDeployerConfig{
			Namespace:          "app-deployer",
			Replicas:           2,
			Image:              "app-deployer",
			ImageTag:           "latest",
			EnableIstioSidecar: true,
		},
	}
}

// PlatformProvisioner provisions the complete platform stack
type PlatformProvisioner struct {
	config PlatformConfig
}

// NewPlatformProvisioner creates a new platform provisioner
func NewPlatformProvisioner(config PlatformConfig) *PlatformProvisioner {
	return &PlatformProvisioner{config: config}
}

// PlatformResult holds the result of platform provisioning
type PlatformResult struct {
	IstioInstalled   bool
	ArgoCDInstalled  bool
	IstioNamespace   string
	ArgoCDNamespace  string
	ArgoCDServerURL  string
	KialiURL         string
	JaegerURL        string
	GrafanaURL       string
	PrometheusURL    string
	Duration         time.Duration
}

// Provision provisions the complete platform
func (p *PlatformProvisioner) Provision(ctx context.Context) (*PlatformResult, error) {
	startTime := time.Now()

	log.Info().
		Str("stackName", p.config.StackName).
		Bool("istio", p.config.EnableIstio).
		Bool("argocd", p.config.EnableArgoCD).
		Msg("Starting platform provisioning")

	// Create Pulumi program
	program := p.createPulumiProgram()

	// Create or select stack
	stack, err := p.createOrSelectStack(ctx, program)
	if err != nil {
		return nil, fmt.Errorf("failed to create stack: %w", err)
	}

	// Set stack configuration
	if err := p.setStackConfig(ctx, stack); err != nil {
		return nil, fmt.Errorf("failed to set stack config: %w", err)
	}

	// Run pulumi up
	log.Info().Str("stackName", p.config.StackName).Msg("Running pulumi up")

	upResult, err := stack.Up(ctx, optup.ProgressStreams(log.Logger))
	if err != nil {
		return nil, fmt.Errorf("pulumi up failed: %w", err)
	}

	// Extract results
	result := &PlatformResult{
		IstioInstalled:  p.config.EnableIstio,
		ArgoCDInstalled: p.config.EnableArgoCD,
		Duration:        time.Since(startTime),
	}

	if p.config.EnableIstio {
		if ns, ok := upResult.Outputs["istioNamespace"].Value.(string); ok {
			result.IstioNamespace = ns
		}
	}

	if p.config.EnableArgoCD {
		if ns, ok := upResult.Outputs["argocdNamespace"].Value.(string); ok {
			result.ArgoCDNamespace = ns
		}
	}

	log.Info().
		Dur("duration", result.Duration).
		Bool("istio", result.IstioInstalled).
		Bool("argocd", result.ArgoCDInstalled).
		Msg("Platform provisioning completed")

	return result, nil
}

// createPulumiProgram creates the Pulumi program for platform provisioning
func (p *PlatformProvisioner) createPulumiProgram() pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		// Create Kubernetes provider
		provider, err := kubernetes.NewProvider(ctx, "k8s-provider", &kubernetes.ProviderArgs{
			Kubeconfig: pulumi.String(p.config.Kubeconfig),
			Context:    pulumi.String(p.config.KubeContext),
		})
		if err != nil {
			return fmt.Errorf("failed to create k8s provider: %w", err)
		}

		// Install Istio if enabled
		if p.config.EnableIstio {
			log.Info().Msg("Installing Istio service mesh")
			_, err := InstallIstio(ctx, provider, p.config.Istio)
			if err != nil {
				return fmt.Errorf("failed to install Istio: %w", err)
			}
		}

		// Install ArgoCD if enabled
		if p.config.EnableArgoCD {
			log.Info().Msg("Installing ArgoCD")
			_, err := InstallArgoCD(ctx, provider, p.config.ArgoCD)
			if err != nil {
				return fmt.Errorf("failed to install ArgoCD: %w", err)
			}
		}

		return nil
	}
}

// createOrSelectStack creates or selects a Pulumi stack
func (p *PlatformProvisioner) createOrSelectStack(ctx context.Context, program pulumi.RunFunc) (auto.Stack, error) {
	projectName := "app-deployer-platform"

	stack, err := auto.UpsertStackInlineSource(ctx, p.config.StackName, projectName, program,
		auto.Project(workspace.Project{
			Name:    tokens.PackageName(projectName),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			Backend: &workspace.ProjectBackend{
				URL: p.config.BackendURL,
			},
		}),
	)
	if err != nil {
		return auto.Stack{}, fmt.Errorf("failed to upsert stack: %w", err)
	}

	return stack, nil
}

// setStackConfig sets the Pulumi stack configuration
func (p *PlatformProvisioner) setStackConfig(ctx context.Context, stack auto.Stack) error {
	// Set Kubernetes configuration
	if p.config.Kubeconfig != "" {
		if err := stack.SetConfig(ctx, "kubernetes:kubeconfig", auto.ConfigValue{Value: p.config.Kubeconfig}); err != nil {
			return err
		}
	}

	if p.config.KubeContext != "" {
		if err := stack.SetConfig(ctx, "kubernetes:context", auto.ConfigValue{Value: p.config.KubeContext}); err != nil {
			return err
		}
	}

	return nil
}

// Destroy destroys the platform stack
func (p *PlatformProvisioner) Destroy(ctx context.Context) error {
	log.Info().Str("stackName", p.config.StackName).Msg("Destroying platform stack")

	program := pulumi.RunFunc(func(ctx *pulumi.Context) error {
		return nil
	})

	stack, err := auto.SelectStackInlineSource(ctx, p.config.StackName, "app-deployer-platform", program,
		auto.Project(workspace.Project{
			Name:    tokens.PackageName("app-deployer-platform"),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			Backend: &workspace.ProjectBackend{
				URL: p.config.BackendURL,
			},
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to select stack: %w", err)
	}

	_, err = stack.Destroy(ctx)
	if err != nil {
		return fmt.Errorf("failed to destroy stack: %w", err)
	}

	// Remove the stack
	if err := stack.Workspace().RemoveStack(ctx, p.config.StackName); err != nil {
		log.Warn().Err(err).Msg("Failed to remove stack")
	}

	return nil
}
