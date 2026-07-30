package chaos

import (
	"bytes"
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// NetworkDelayConfig defines parameters for network latency injection.
type NetworkDelayConfig struct {
	Namespace     string
	PodName       string
	ContainerName string
	Interface     string
	Delay         time.Duration
	Jitter        time.Duration
}

// NetworkInjector executes network-level fault injections on target pods.
type NetworkInjector struct {
	client     kubernetes.Interface
	restConfig *rest.Config
}

// NewNetworkInjector constructs a new NetworkInjector instance.
func NewNetworkInjector(client kubernetes.Interface, restConfig *rest.Config) *NetworkInjector {
	return &NetworkInjector{
		client:     client,
		restConfig: restConfig,
	}
}

// Inject applies network delay to the target pod's network interface using tc netem.
func (ni *NetworkInjector) Inject(ctx context.Context, cfg NetworkDelayConfig) error {
	if cfg.Interface == "" {
		cfg.Interface = "eth0"
	}
	if cfg.Delay <= 0 {
		return fmt.Errorf("delay duration must be greater than zero")
	}

	cmd := []string{
		"tc", "qdisc", "add", "dev", cfg.Interface, "root", "netem",
		"delay", fmt.Sprintf("%dms", cfg.Delay.Milliseconds()),
	}
	if cfg.Jitter > 0 {
		cmd = append(cmd, fmt.Sprintf("%dms", cfg.Jitter.Milliseconds()))
	}

	_, stderr, err := ni.execPodCommand(ctx, cfg.Namespace, cfg.PodName, cfg.ContainerName, cmd)
	if err != nil {
		return fmt.Errorf("failed to inject network delay on pod %s (requires NET_ADMIN capabilities and 'tc' binary): %w, stderr: %s", cfg.PodName, err, stderr)
	}

	return nil
}

// Remove clears any applied qdisc rules from the target pod's network interface.
func (ni *NetworkInjector) Remove(ctx context.Context, cfg NetworkDelayConfig) error {
	if cfg.Interface == "" {
		cfg.Interface = "eth0"
	}

	cmd := []string{"tc", "qdisc", "del", "dev", cfg.Interface, "root"}

	_, stderr, err := ni.execPodCommand(ctx, cfg.Namespace, cfg.PodName, cfg.ContainerName, cmd)
	if err != nil {
		return fmt.Errorf("failed to remove network delay rule from pod %s: %w, stderr: %s", cfg.PodName, err, stderr)
	}

	return nil
}

// execPodCommand leverages client-go remotecommand to execute CLI commands inside a running container.
func (ni *NetworkInjector) execPodCommand(ctx context.Context, namespace, podName, containerName string, command []string) (string, string, error) {
	req := ni.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	option := &corev1.PodExecOptions{
		Container: containerName,
		Command:   command,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}
	req.VersionedParams(option, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(ni.restConfig, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("failed to initialize SPDY executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return stdout.String(), stderr.String(), err
	}

	return stdout.String(), stderr.String(), nil
}
