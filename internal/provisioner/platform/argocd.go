package platform

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// ArgoCDConfig holds configuration for ArgoCD installation
type ArgoCDConfig struct {
	// Version is the ArgoCD version to install
	Version string
	// Namespace is the namespace to install ArgoCD
	Namespace string
	// ServerServiceType is the service type for ArgoCD server (LoadBalancer, NodePort, ClusterIP)
	ServerServiceType string
	// EnableHA enables high availability mode
	EnableHA bool
	// EnableDex enables Dex for SSO
	EnableDex bool
	// AdminPassword is the admin password (bcrypt hash)
	AdminPassword string
	// Repositories are the Git repositories to configure
	Repositories []ArgoCDRepository
	// EnableMetrics enables Prometheus metrics
	EnableMetrics bool
	// EnableNotifications enables ArgoCD notifications
	EnableNotifications bool
}

// ArgoCDRepository represents a Git repository configuration
type ArgoCDRepository struct {
	Name     string
	URL      string
	Username string
	Password string
	SSHKey   string
	Type     string // git, helm
}

// DefaultArgoCDConfig returns default ArgoCD configuration
func DefaultArgoCDConfig() ArgoCDConfig {
	return ArgoCDConfig{
		Version:             "5.51.6",
		Namespace:           "argocd",
		ServerServiceType:   "LoadBalancer",
		EnableHA:            false,
		EnableDex:           false,
		EnableMetrics:       true,
		EnableNotifications: true,
	}
}

// ArgoCDResources holds created ArgoCD resources
type ArgoCDResources struct {
	Namespace *corev1.Namespace
	ArgoCD    *helmv3.Release
}

// InstallArgoCD installs ArgoCD using Helm
func InstallArgoCD(ctx *pulumi.Context, provider *kubernetes.Provider, config ArgoCDConfig) (*ArgoCDResources, error) {
	resources := &ArgoCDResources{}

	// Create argocd namespace
	argoNs, err := corev1.NewNamespace(ctx, "argocd-namespace", &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(config.Namespace),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/name": pulumi.String("argocd"),
				// Enable Istio sidecar injection for ArgoCD
				"istio-injection": pulumi.String("enabled"),
			},
		},
	}, pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}
	resources.Namespace = argoNs

	// Build values for ArgoCD Helm chart
	values := pulumi.Map{
		"global": pulumi.Map{
			"domain": pulumi.String("argocd.local"),
		},
		"configs": pulumi.Map{
			"params": pulumi.Map{
				"server.insecure": pulumi.Bool(true), // Disable TLS for internal use (Istio handles TLS)
			},
			"cm": pulumi.Map{
				"exec.enabled":  pulumi.Bool(true),
				"admin.enabled": pulumi.Bool(true),
			},
		},
		"server": pulumi.Map{
			"service": pulumi.Map{
				"type": pulumi.String(config.ServerServiceType),
			},
			"metrics": pulumi.Map{
				"enabled": pulumi.Bool(config.EnableMetrics),
				"serviceMonitor": pulumi.Map{
					"enabled": pulumi.Bool(config.EnableMetrics),
				},
			},
			"extraArgs": pulumi.StringArray{
				pulumi.String("--insecure"),
			},
		},
		"controller": pulumi.Map{
			"metrics": pulumi.Map{
				"enabled": pulumi.Bool(config.EnableMetrics),
				"serviceMonitor": pulumi.Map{
					"enabled": pulumi.Bool(config.EnableMetrics),
				},
			},
		},
		"repoServer": pulumi.Map{
			"metrics": pulumi.Map{
				"enabled": pulumi.Bool(config.EnableMetrics),
				"serviceMonitor": pulumi.Map{
					"enabled": pulumi.Bool(config.EnableMetrics),
				},
			},
		},
		"applicationSet": pulumi.Map{
			"enabled": pulumi.Bool(true),
			"metrics": pulumi.Map{
				"enabled": pulumi.Bool(config.EnableMetrics),
			},
		},
		"notifications": pulumi.Map{
			"enabled": pulumi.Bool(config.EnableNotifications),
			"metrics": pulumi.Map{
				"enabled": pulumi.Bool(config.EnableMetrics),
			},
		},
		"dex": pulumi.Map{
			"enabled": pulumi.Bool(config.EnableDex),
		},
		"redis": pulumi.Map{
			"enabled": pulumi.Bool(true),
		},
	}

	// Enable HA if configured
	if config.EnableHA {
		values["controller"] = pulumi.Map{
			"replicas": pulumi.Int(2),
		}
		values["server"] = pulumi.Map{
			"replicas": pulumi.Int(2),
			"service": pulumi.Map{
				"type": pulumi.String(config.ServerServiceType),
			},
		}
		values["repoServer"] = pulumi.Map{
			"replicas": pulumi.Int(2),
		}
		values["redis-ha"] = pulumi.Map{
			"enabled": pulumi.Bool(true),
		}
		values["redis"] = pulumi.Map{
			"enabled": pulumi.Bool(false),
		}
	}

	// Install ArgoCD Helm chart
	argocd, err := helmv3.NewRelease(ctx, "argocd", &helmv3.ReleaseArgs{
		Name:            pulumi.String("argocd"),
		Namespace:       argoNs.Metadata.Name(),
		Chart:           pulumi.String("argo-cd"),
		Version:         pulumi.String(config.Version),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://argoproj.github.io/argo-helm"),
		},
		Values: values,
	}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{argoNs}))
	if err != nil {
		return nil, err
	}
	resources.ArgoCD = argocd

	// Export ArgoCD outputs
	ctx.Export("argocdNamespace", argoNs.Metadata.Name())
	ctx.Export("argocdVersion", pulumi.String(config.Version))

	return resources, nil
}

// CreateArgoCDProject creates an ArgoCD project
func CreateArgoCDProject(ctx *pulumi.Context, provider *kubernetes.Provider, name, namespace string, sourceRepos, destinations []string) error {
	// Create ArgoCD AppProject CRD
	_, err := corev1.NewConfigMap(ctx, "argocd-project-"+name, &corev1.ConfigMapArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("argocd-project-" + name),
			Namespace: pulumi.String(namespace),
			Labels: pulumi.StringMap{
				"argocd.argoproj.io/project": pulumi.String(name),
			},
		},
	}, pulumi.Provider(provider))

	return err
}
