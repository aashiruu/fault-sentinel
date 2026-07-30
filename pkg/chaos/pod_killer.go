package chaos

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PodKillerConfig holds parameters for executing pod termination experiments.
type PodKillerConfig struct {
	Namespace          string
	LabelSelector      string
	GracePeriodSeconds int64
	Force              bool
}

// PodKiller executes pod deletion faults against target namespaces.
type PodKiller struct {
	client kubernetes.Interface
}

// NewPodKiller constructs a new PodKiller instance.
func NewPodKiller(client kubernetes.Interface) *PodKiller {
	return &PodKiller{
		client: client,
	}
}

// Inject selects a target pod matching the configured label selector and deletes it.
func (pk *PodKiller) Inject(ctx context.Context, cfg PodKillerConfig) (string, error) {
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}
	if cfg.LabelSelector == "" {
		return "", fmt.Errorf("label selector cannot be empty")
	}

	podList, err := pk.client.CoreV1().Pods(cfg.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: cfg.LabelSelector,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods in namespace %s: %w", cfg.Namespace, err)
	}

	// Filter for eligible running pods
	var eligiblePods []corev1.Pod
	for _, pod := range podList.Items {
		if pod.DeletionTimestamp == nil && pod.Status.Phase == corev1.PodRunning {
			eligiblePods = append(eligiblePods, pod)
		}
	}

	if len(eligiblePods) == 0 {
		return "", fmt.Errorf("no running pods found matching selector %q in namespace %q", cfg.LabelSelector, cfg.Namespace)
	}

	// Pseudo-random selection
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	targetPod := eligiblePods[rng.Intn(len(eligiblePods))]

	deleteOptions := metav1.DeleteOptions{}
	if cfg.Force {
		zero := int64(0)
		deleteOptions.GracePeriodSeconds = &zero
	} else if cfg.GracePeriodSeconds > 0 {
		deleteOptions.GracePeriodSeconds = &cfg.GracePeriodSeconds
	}

	err = pk.client.CoreV1().Pods(cfg.Namespace).Delete(ctx, targetPod.Name, deleteOptions)
	if err != nil {
		return "", fmt.Errorf("failed to delete pod %s: %w", targetPod.Name, err)
	}

	return targetPod.Name, nil
}
