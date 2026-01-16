package deployer

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/alvesdmateus/app-deployer/internal/state"
)

// KubeClient wraps Kubernetes client operations
type KubeClient struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

// NewKubeClient creates a Kubernetes client from infrastructure details
func NewKubeClient(infra *state.Infrastructure) (*KubeClient, error) {
	log.Info().
		Str("clusterEndpoint", infra.ClusterEndpoint).
		Str("namespace", infra.Namespace).
		Msg("Creating Kubernetes client")

	// Decode CA certificate
	caCert, err := base64.StdEncoding.DecodeString(infra.ClusterCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to decode CA certificate: %w", err)
	}

	// Create kubeconfig
	config := createKubeConfig(infra.ClusterEndpoint, caCert, infra.ClusterName)

	// Create REST config
	restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create REST config: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Test connection
	if err := testConnection(clientset); err != nil {
		return nil, fmt.Errorf("failed to connect to cluster: %w", err)
	}

	log.Info().
		Str("clusterName", infra.ClusterName).
		Msg("Kubernetes client created successfully")

	return &KubeClient{
		clientset: clientset,
		config:    restConfig,
	}, nil
}

// createKubeConfig creates a kubeconfig from cluster details
func createKubeConfig(endpoint string, caCert []byte, clusterName string) *clientcmdapi.Config {
	// Add https:// prefix if not present
	if endpoint[:8] != "https://" && endpoint[:7] != "http://" {
		endpoint = "https://" + endpoint
	}

	return &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   endpoint,
				CertificateAuthorityData: caCert,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			clusterName: {
				Cluster:  clusterName,
				AuthInfo: "gcp",
			},
		},
		CurrentContext: clusterName,
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"gcp": {
				// Use gke-gcloud-auth-plugin for GKE authentication
				Exec: &clientcmdapi.ExecConfig{
					Command:            "gke-gcloud-auth-plugin",
					Args:               []string{},
					APIVersion:         "client.authentication.k8s.io/v1beta1",
					InteractiveMode:    clientcmdapi.NeverExecInteractiveMode,
					InstallHint:        "Install gke-gcloud-auth-plugin: gcloud components install gke-gcloud-auth-plugin",
					ProvideClusterInfo: true,
				},
			},
		},
	}
}

// testConnection tests the connection to the cluster
func testConnection(clientset *kubernetes.Clientset) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to list nodes as a connection test
	_, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}

	return nil
}

// CreateNamespace creates a Kubernetes namespace
func (k *KubeClient) CreateNamespace(ctx context.Context, name string, labels map[string]string) error {
	log.Info().Str("namespace", name).Msg("Creating namespace")

	// Check if namespace already exists
	_, err := k.clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		log.Info().Str("namespace", name).Msg("Namespace already exists")
		return nil
	}

	// Create namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}

	_, err = k.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	log.Info().Str("namespace", name).Msg("Namespace created successfully")
	return nil
}

// DeleteNamespace deletes a Kubernetes namespace
func (k *KubeClient) DeleteNamespace(ctx context.Context, name string) error {
	log.Info().Str("namespace", name).Msg("Deleting namespace")

	err := k.clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete namespace: %w", err)
	}

	log.Info().Str("namespace", name).Msg("Namespace deleted successfully")
	return nil
}

// GetLoadBalancerIP retrieves the external IP of a LoadBalancer service
func (k *KubeClient) GetLoadBalancerIP(ctx context.Context, namespace, serviceName string, timeout time.Duration) (string, error) {
	log.Info().
		Str("namespace", namespace).
		Str("service", serviceName).
		Msg("Waiting for LoadBalancer IP")

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		svc, err := k.clientset.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to get service: %w", err)
		}

		// Check for LoadBalancer ingress
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			ip := svc.Status.LoadBalancer.Ingress[0].IP
			if ip != "" {
				log.Info().
					Str("namespace", namespace).
					Str("service", serviceName).
					Str("ip", ip).
					Msg("LoadBalancer IP assigned")
				return ip, nil
			}

			// Some cloud providers use Hostname instead of IP
			hostname := svc.Status.LoadBalancer.Ingress[0].Hostname
			if hostname != "" {
				log.Info().
					Str("namespace", namespace).
					Str("service", serviceName).
					Str("hostname", hostname).
					Msg("LoadBalancer hostname assigned")
				return hostname, nil
			}
		}

		// Wait before retrying
		time.Sleep(5 * time.Second)
	}

	return "", fmt.Errorf("timeout waiting for LoadBalancer IP after %v", timeout)
}

// WaitForPodsReady waits for pods to be ready
func (k *KubeClient) WaitForPodsReady(ctx context.Context, namespace string, labelSelector string, timeout time.Duration) error {
	log.Info().
		Str("namespace", namespace).
		Str("labelSelector", labelSelector).
		Msg("Waiting for pods to be ready")

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		pods, err := k.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return fmt.Errorf("failed to list pods: %w", err)
		}

		if len(pods.Items) == 0 {
			log.Debug().Msg("No pods found yet, waiting...")
			time.Sleep(5 * time.Second)
			continue
		}

		// Check if all pods are ready
		allReady := true
		for _, pod := range pods.Items {
			podReady := false
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					podReady = true
					break
				}
			}

			if !podReady {
				allReady = false
				log.Debug().
					Str("pod", pod.Name).
					Str("phase", string(pod.Status.Phase)).
					Msg("Pod not ready yet")
				break
			}
		}

		if allReady {
			log.Info().
				Str("namespace", namespace).
				Int("podCount", len(pods.Items)).
				Msg("All pods are ready")
			return nil
		}

		// Wait before retrying
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for pods to be ready after %v", timeout)
}

// GetPodCount returns the number of ready and total pods
func (k *KubeClient) GetPodCount(ctx context.Context, namespace string, labelSelector string) (ready, total int, err error) {
	pods, err := k.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list pods: %w", err)
	}

	total = len(pods.Items)

	for _, pod := range pods.Items {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				ready++
				break
			}
		}
	}

	return ready, total, nil
}

// GetClientset returns the underlying Kubernetes clientset
func (k *KubeClient) GetClientset() *kubernetes.Clientset {
	return k.clientset
}

// GetRestConfig returns the REST config
func (k *KubeClient) GetRestConfig() *rest.Config {
	return k.config
}
