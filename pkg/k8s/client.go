package k8s

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client encapsulates the typed Kubernetes clientset.
type Client struct {
	Clientset kubernetes.Interface
}

// GetRestConfig resolves the REST configuration using an in-cluster configuration first,
// falling back to out-of-cluster kubeconfig resolution if run locally.
func GetRestConfig(kubeconfigPath string) (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fallback to out-of-cluster resolution
	if kubeconfigPath == "" {
		if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" {
			kubeconfigPath = envKubeconfig
		} else if home := homedir.HomeDir(); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		} else {
			return nil, fmt.Errorf("out-of-cluster configuration failed: no kubeconfig path provided and home directory unavailable")
		}
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig from path %s: %w", kubeconfigPath, err)
	}

	return config, nil
}

// NewClient initializes a Kubernetes client using GetRestConfig.
func NewClient(kubeconfigPath string) (*Client, error) {
	config, err := GetRestConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset from rest config: %w", err)
	}

	return &Client{Clientset: clientset}, nil
}
