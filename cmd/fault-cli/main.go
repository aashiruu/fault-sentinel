package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/aashiruu/fault-sentinel/pkg/chaos"
	"github.com/aashiruu/fault-sentinel/pkg/k8s"
	"github.com/aashiruu/fault-sentinel/pkg/telemetry"
	"github.com/spf13/cobra"
)

var (
	kubeconfigPath string
	namespace      string
	telemetryPort  int
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	exporter := telemetry.NewExporter()

	rootCmd := &cobra.Command{
		Use:   "fault-cli",
		Short: "fault-sentinel is an experimental Kubernetes fault injection CLI for chaos engineering.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			exporter.Start(telemetryPort)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = exporter.Stop(shutdownCtx)
		},
	}

	rootCmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig", "", "Path to kubeconfig file (defaults to in-cluster or ~/.kube/config)")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Target Kubernetes namespace")
	rootCmd.PersistentFlags().IntVar(&telemetryPort, "metrics-port", 8080, "HTTP port for exposing Prometheus telemetry metrics")

	rootCmd.AddCommand(newPodKillerCmd(exporter))
	rootCmd.AddCommand(newCPUStressCmd(exporter))
	rootCmd.AddCommand(newNetworkDelayCmd(exporter))

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		os.Exit(1)
	}
}

func newPodKillerCmd(exporter *telemetry.Exporter) *cobra.Command {
	var selector string
	var gracePeriod int64
	var force bool

	cmd := &cobra.Command{
		Use:   "kill-pod",
		Short: "Inject a pod termination fault against a target namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			client, err := k8s.NewClient(kubeconfigPath)
			if err != nil {
				exporter.ExperimentsTotal.WithLabelValues("pod_killer", "failure").Inc()
				return err
			}

			killer := chaos.NewPodKiller(client.Clientset)
			cfg := chaos.PodKillerConfig{
				Namespace:          namespace,
				LabelSelector:      selector,
				GracePeriodSeconds: gracePeriod,
				Force:              force,
			}

			podName, err := killer.Inject(cmd.Context(), cfg)
			elapsed := time.Since(start).Seconds()
			exporter.ExperimentDurationSeconds.WithLabelValues("pod_killer").Observe(elapsed)

			if err != nil {
				exporter.ExperimentsTotal.WithLabelValues("pod_killer", "failure").Inc()
				return err
			}

			exporter.ExperimentsTotal.WithLabelValues("pod_killer", "success").Inc()
			exporter.InjectedFaultsTotal.WithLabelValues(podName, "pod_deletion").Inc()

			fmt.Printf("Successfully deleted pod %q in namespace %q\n", podName, namespace)
			return nil
		},
	}

	cmd.Flags().StringVarP(&selector, "selector", "l", "", "Label selector identifying target pods (e.g. app=payment-api)")
	cmd.Flags().Int64Var(&gracePeriod, "grace-period", 30, "Grace period in seconds for pod deletion")
	cmd.Flags().BoolVar(&force, "force", false, "Force immediate pod deletion (sets grace-period to 0)")
	_ = cmd.MarkFlagRequired("selector")

	return cmd
}

func newCPUStressCmd(exporter *telemetry.Exporter) *cobra.Command {
	var duration time.Duration
	var cores int

	cmd := &cobra.Command{
		Use:   "stress-cpu",
		Short: "Inject CPU load using worker goroutines",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			stressor := chaos.NewStressor()

			activeCores := cores
			if activeCores <= 0 {
				activeCores = runtime.GOMAXPROCS(0)
			}

			fmt.Printf("Injecting CPU stress using %d cores for %s...\n", activeCores, duration)
			err := stressor.InjectCPU(cmd.Context(), duration, activeCores)

			elapsed := time.Since(start).Seconds()
			exporter.ExperimentDurationSeconds.WithLabelValues("cpu_stress").Observe(elapsed)

			if err != nil {
				exporter.ExperimentsTotal.WithLabelValues("cpu_stress", "failure").Inc()
				return err
			}

			exporter.ExperimentsTotal.WithLabelValues("cpu_stress", "success").Inc()
			exporter.InjectedFaultsTotal.WithLabelValues("local_node", "cpu_stress").Inc()

			fmt.Println("CPU stress injection completed.")
			return nil
		},
	}

	cmd.Flags().DurationVarP(&duration, "duration", "d", 30*time.Second, "Duration of the CPU stress test")
	cmd.Flags().IntVar(&cores, "cores", 0, "Number of CPU cores to stress (0 defaults to GOMAXPROCS)")

	return cmd
}

func newNetworkDelayCmd(exporter *telemetry.Exporter) *cobra.Command {
	var podName string
	var container string
	var delay time.Duration
	var duration time.Duration

	cmd := &cobra.Command{
		Use:   "network-delay",
		Short: "Inject network delay on a pod interface using tc netem via pod exec",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			client, err := k8s.NewClient(kubeconfigPath)
			if err != nil {
				exporter.ExperimentsTotal.WithLabelValues("network_delay", "failure").Inc()
				return err
			}

			inClusterConfig, err := k8s.GetRestConfig(kubeconfigPath)
			if err != nil {
				exporter.ExperimentsTotal.WithLabelValues("network_delay", "failure").Inc()
				return err
			}

			injector := chaos.NewNetworkInjector(client.Clientset, inClusterConfig)
			cfg := chaos.NetworkDelayConfig{
				Namespace:     namespace,
				PodName:       podName,
				ContainerName: container,
				Delay:         delay,
			}

			fmt.Printf("Injecting %s network delay into pod %q...\n", delay, podName)
			if err := injector.Inject(cmd.Context(), cfg); err != nil {
				exporter.ExperimentsTotal.WithLabelValues("network_delay", "failure").Inc()
				return err
			}

			exporter.InjectedFaultsTotal.WithLabelValues(podName, "network_delay").Inc()
			fmt.Printf("Delay active. Waiting %s before cleanup...\n", duration)

			select {
			case <-time.After(duration):
			case <-cmd.Context().Done():
				fmt.Println("Context canceled, removing network delay immediately...")
			}

			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if removeErr := injector.Remove(cleanupCtx, cfg); removeErr != nil {
				fmt.Printf("Warning: failed to remove network delay rule: %v\n", removeErr)
			} else {
				fmt.Println("Network delay removed successfully.")
			}

			elapsed := time.Since(start).Seconds()
			exporter.ExperimentDurationSeconds.WithLabelValues("network_delay").Observe(elapsed)
			exporter.ExperimentsTotal.WithLabelValues("network_delay", "success").Inc()

			return nil
		},
	}

	cmd.Flags().StringVar(&podName, "pod", "", "Target pod name")
	cmd.Flags().StringVar(&container, "container", "", "Target container name inside the pod")
	cmd.Flags().DurationVar(&delay, "delay", 200*time.Millisecond, "Latency to inject (e.g., 100ms, 1s)")
	cmd.Flags().DurationVar(&duration, "duration", 30*time.Second, "Duration to keep the latency active")

	_ = cmd.MarkFlagRequired("pod")
	_ = cmd.MarkFlagRequired("container")

	return cmd
}
