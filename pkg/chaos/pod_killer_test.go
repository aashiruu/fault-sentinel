package chaos

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPodKiller_Inject_Success(t *testing.T) {
	// 1. Seed the fake clientset with dummy pods
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "payment-api-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "payment-api"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "payment-api-2",
			Namespace: "default",
			Labels:    map[string]string{"app": "payment-api"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	clientset := fake.NewSimpleClientset(pod1, pod2)
	killer := NewPodKiller(clientset)

	cfg := PodKillerConfig{
		Namespace:     "default",
		LabelSelector: "app=payment-api",
		Force:         true,
	}

	// 2. Execute Pod Killer injection
	deletedPod, err := killer.Inject(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if deletedPod != "payment-api-1" && deletedPod != "payment-api-2" {
		t.Errorf("unexpected pod deleted: %s", deletedPod)
	}

	// 3. Verify pod was removed from the API server state
	remainingPods, err := clientset.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list pods: %v", err)
	}

	if len(remainingPods.Items) != 1 {
		t.Errorf("expected 1 pod remaining, found %d", len(remainingPods.Items))
	}
}

func TestPodKiller_Inject_NoMatchingPods(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	killer := NewPodKiller(clientset)

	cfg := PodKillerConfig{
		Namespace:     "default",
		LabelSelector: "app=nonexistent",
	}

	_, err := killer.Inject(context.Background(), cfg)
	if err == nil {
		t.Error("expected error when no matching pods exist, got nil")
	}
}
