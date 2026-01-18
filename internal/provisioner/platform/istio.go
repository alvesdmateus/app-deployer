package platform

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// IstioConfig holds configuration for Istio installation
type IstioConfig struct {
	// Version is the Istio version to install
	Version string
	// Profile is the Istio profile (default, demo, minimal, remote, empty)
	Profile string
	// EnableTracing enables distributed tracing
	EnableTracing bool
	// TracingSamplingRate is the sampling rate for tracing (0-100)
	TracingSamplingRate float64
	// EnableKiali enables Kiali dashboard
	EnableKiali bool
	// EnableGrafana enables Grafana
	EnableGrafana bool
	// EnablePrometheus enables Prometheus
	EnablePrometheus bool
	// EnableJaeger enables Jaeger tracing
	EnableJaeger bool
	// IngressGatewayType is the service type for ingress gateway (LoadBalancer, NodePort, ClusterIP)
	IngressGatewayType string
}

// DefaultIstioConfig returns default Istio configuration
func DefaultIstioConfig() IstioConfig {
	return IstioConfig{
		Version:             "1.20.2",
		Profile:             "default",
		EnableTracing:       true,
		TracingSamplingRate: 100.0,
		EnableKiali:         true,
		EnableGrafana:       true,
		EnablePrometheus:    true,
		EnableJaeger:        true,
		IngressGatewayType:  "LoadBalancer",
	}
}

// IstioResources holds created Istio resources
type IstioResources struct {
	Namespace      *corev1.Namespace
	IstioBase      *helmv3.Release
	Istiod         *helmv3.Release
	IngressGateway *helmv3.Release
	Kiali          *helmv3.Release
	Jaeger         *helmv3.Release
	Prometheus     *helmv3.Release
	Grafana        *helmv3.Release
}

// InstallIstio installs Istio service mesh using Helm
func InstallIstio(ctx *pulumi.Context, provider *kubernetes.Provider, config IstioConfig) (*IstioResources, error) {
	resources := &IstioResources{}

	// Create istio-system namespace
	istioNs, err := corev1.NewNamespace(ctx, "istio-system", &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("istio-system"),
			Labels: pulumi.StringMap{
				"istio-injection": pulumi.String("disabled"),
			},
		},
	}, pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}
	resources.Namespace = istioNs

	// Install Istio base (CRDs)
	istioBase, err := helmv3.NewRelease(ctx, "istio-base", &helmv3.ReleaseArgs{
		Name:            pulumi.String("istio-base"),
		Namespace:       istioNs.Metadata.Name(),
		Chart:           pulumi.String("base"),
		Version:         pulumi.String(config.Version),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://istio-release.storage.googleapis.com/charts"),
		},
		Values: pulumi.Map{
			"defaultRevision": pulumi.String("default"),
		},
	}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{istioNs}))
	if err != nil {
		return nil, err
	}
	resources.IstioBase = istioBase

	// Install Istiod (control plane)
	istiod, err := helmv3.NewRelease(ctx, "istiod", &helmv3.ReleaseArgs{
		Name:            pulumi.String("istiod"),
		Namespace:       istioNs.Metadata.Name(),
		Chart:           pulumi.String("istiod"),
		Version:         pulumi.String(config.Version),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://istio-release.storage.googleapis.com/charts"),
		},
		Values: pulumi.Map{
			"meshConfig": pulumi.Map{
				"enableTracing": pulumi.Bool(config.EnableTracing),
				"defaultConfig": pulumi.Map{
					"tracing": pulumi.Map{
						"sampling": pulumi.Float64(config.TracingSamplingRate),
					},
				},
				"accessLogFile":     pulumi.String("/dev/stdout"),
				"accessLogEncoding": pulumi.String("JSON"),
			},
			"pilot": pulumi.Map{
				"traceSampling": pulumi.Float64(config.TracingSamplingRate),
			},
			"global": pulumi.Map{
				"tracer": pulumi.Map{
					"zipkin": pulumi.Map{
						"address": pulumi.String("jaeger-collector.istio-system.svc.cluster.local:9411"),
					},
				},
			},
		},
	}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{istioBase}))
	if err != nil {
		return nil, err
	}
	resources.Istiod = istiod

	// Install Istio Ingress Gateway
	ingressGateway, err := helmv3.NewRelease(ctx, "istio-ingress", &helmv3.ReleaseArgs{
		Name:            pulumi.String("istio-ingressgateway"),
		Namespace:       istioNs.Metadata.Name(),
		Chart:           pulumi.String("gateway"),
		Version:         pulumi.String(config.Version),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://istio-release.storage.googleapis.com/charts"),
		},
		Values: pulumi.Map{
			"service": pulumi.Map{
				"type": pulumi.String(config.IngressGatewayType),
			},
		},
	}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{istiod}))
	if err != nil {
		return nil, err
	}
	resources.IngressGateway = ingressGateway

	// Install Jaeger for tracing
	if config.EnableJaeger {
		jaeger, err := helmv3.NewRelease(ctx, "jaeger", &helmv3.ReleaseArgs{
			Name:            pulumi.String("jaeger"),
			Namespace:       istioNs.Metadata.Name(),
			Chart:           pulumi.String("jaeger"),
			Version:         pulumi.String("0.71.14"),
			CreateNamespace: pulumi.Bool(false),
			RepositoryOpts: &helmv3.RepositoryOptsArgs{
				Repo: pulumi.String("https://jaegertracing.github.io/helm-charts"),
			},
			Values: pulumi.Map{
				"provisionDataStore": pulumi.Map{
					"cassandra": pulumi.Bool(false),
				},
				"allInOne": pulumi.Map{
					"enabled": pulumi.Bool(true),
				},
				"storage": pulumi.Map{
					"type": pulumi.String("memory"),
				},
				"agent": pulumi.Map{
					"enabled": pulumi.Bool(false),
				},
				"collector": pulumi.Map{
					"enabled": pulumi.Bool(false),
				},
				"query": pulumi.Map{
					"enabled": pulumi.Bool(false),
				},
			},
		}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{istiod}))
		if err != nil {
			return nil, err
		}
		resources.Jaeger = jaeger
	}

	// Install Kiali dashboard
	if config.EnableKiali {
		kiali, err := helmv3.NewRelease(ctx, "kiali", &helmv3.ReleaseArgs{
			Name:            pulumi.String("kiali"),
			Namespace:       istioNs.Metadata.Name(),
			Chart:           pulumi.String("kiali-server"),
			Version:         pulumi.String("1.79.0"),
			CreateNamespace: pulumi.Bool(false),
			RepositoryOpts: &helmv3.RepositoryOptsArgs{
				Repo: pulumi.String("https://kiali.org/helm-charts"),
			},
			Values: pulumi.Map{
				"auth": pulumi.Map{
					"strategy": pulumi.String("anonymous"),
				},
				"external_services": pulumi.Map{
					"prometheus": pulumi.Map{
						"url": pulumi.String("http://prometheus.istio-system.svc.cluster.local:9090"),
					},
					"grafana": pulumi.Map{
						"enabled":        pulumi.Bool(config.EnableGrafana),
						"in_cluster_url": pulumi.String("http://grafana.istio-system.svc.cluster.local:3000"),
					},
					"tracing": pulumi.Map{
						"enabled":        pulumi.Bool(config.EnableJaeger),
						"in_cluster_url": pulumi.String("http://jaeger-query.istio-system.svc.cluster.local:16686"),
					},
				},
			},
		}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{istiod}))
		if err != nil {
			return nil, err
		}
		resources.Kiali = kiali
	}

	// Install Prometheus for metrics
	if config.EnablePrometheus {
		prometheus, err := helmv3.NewRelease(ctx, "prometheus", &helmv3.ReleaseArgs{
			Name:            pulumi.String("prometheus"),
			Namespace:       istioNs.Metadata.Name(),
			Chart:           pulumi.String("prometheus"),
			Version:         pulumi.String("25.8.0"),
			CreateNamespace: pulumi.Bool(false),
			RepositoryOpts: &helmv3.RepositoryOptsArgs{
				Repo: pulumi.String("https://prometheus-community.github.io/helm-charts"),
			},
			Values: pulumi.Map{
				"server": pulumi.Map{
					"global": pulumi.Map{
						"scrape_interval": pulumi.String("15s"),
					},
					"persistentVolume": pulumi.Map{
						"enabled": pulumi.Bool(false),
					},
				},
				"alertmanager": pulumi.Map{
					"enabled": pulumi.Bool(false),
				},
				"kube-state-metrics": pulumi.Map{
					"enabled": pulumi.Bool(true),
				},
				"prometheus-node-exporter": pulumi.Map{
					"enabled": pulumi.Bool(true),
				},
				"prometheus-pushgateway": pulumi.Map{
					"enabled": pulumi.Bool(false),
				},
			},
		}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{istiod}))
		if err != nil {
			return nil, err
		}
		resources.Prometheus = prometheus
	}

	// Install Grafana
	if config.EnableGrafana {
		grafana, err := helmv3.NewRelease(ctx, "grafana", &helmv3.ReleaseArgs{
			Name:            pulumi.String("grafana"),
			Namespace:       istioNs.Metadata.Name(),
			Chart:           pulumi.String("grafana"),
			Version:         pulumi.String("7.0.19"),
			CreateNamespace: pulumi.Bool(false),
			RepositoryOpts: &helmv3.RepositoryOptsArgs{
				Repo: pulumi.String("https://grafana.github.io/helm-charts"),
			},
			Values: pulumi.Map{
				"adminPassword": pulumi.String("admin"),
				"persistence": pulumi.Map{
					"enabled": pulumi.Bool(false),
				},
				"datasources": pulumi.Map{
					"datasources.yaml": pulumi.Map{
						"apiVersion": pulumi.Int(1),
						"datasources": pulumi.Array{
							pulumi.Map{
								"name":      pulumi.String("Prometheus"),
								"type":      pulumi.String("prometheus"),
								"url":       pulumi.String("http://prometheus-server.istio-system.svc.cluster.local"),
								"access":    pulumi.String("proxy"),
								"isDefault": pulumi.Bool(true),
							},
							pulumi.Map{
								"name":   pulumi.String("Jaeger"),
								"type":   pulumi.String("jaeger"),
								"url":    pulumi.String("http://jaeger-query.istio-system.svc.cluster.local:16686"),
								"access": pulumi.String("proxy"),
							},
						},
					},
				},
			},
		}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{istiod}))
		if err != nil {
			return nil, err
		}
		resources.Grafana = grafana
	}

	// Export Istio outputs
	ctx.Export("istioNamespace", istioNs.Metadata.Name())
	ctx.Export("istioVersion", pulumi.String(config.Version))

	return resources, nil
}
