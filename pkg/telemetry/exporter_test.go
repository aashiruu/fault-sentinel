package telemetry

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestExporter_StartAndStop(t *testing.T) {
	exporter := NewExporter()

	// Increment counters
	exporter.ExperimentsTotal.WithLabelValues("pod_killer", "success").Inc()
	exporter.InjectedFaultsTotal.WithLabelValues("pod-1", "pod_deletion").Inc()
	exporter.ExperimentDurationSeconds.WithLabelValues("pod_killer").Observe(1.5)

	// Start server on an ephemeral port for testing
	port := 8089
	exporter.Start(port)

	// Wait briefly for server startup
	time.Sleep(100 * time.Millisecond)

	// Query metrics endpoint
	resp, err := http.Get("http://localhost:8089/metrics")
	if err != nil {
		t.Fatalf("failed to query metrics endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
	}

	// shutdown test
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := exporter.Stop(ctx); err != nil {
		t.Errorf("failed to stop metrics exporter cleanly: %v", err)
	}
}
