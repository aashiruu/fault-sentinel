package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Exporter encapsulates the Prometheus metric registry and HTTP server.
type Exporter struct {
	Registry                 *prometheus.Registry
	ExperimentsTotal         *prometheus.CounterVec
	InjectedFaultsTotal      *prometheus.CounterVec
	ExperimentDurationSeconds *prometheus.HistogramVec
	server                   *http.Server
}

// NewExporter initializes an Exporter with custom Prometheus metrics and an isolated registry.
func NewExporter() *Exporter {
	reg := prometheus.NewRegistry()

	experimentsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chaos_experiments_total",
			Help: "Total number of executed chaos experiments.",
		},
		[]string{"experiment_type", "status"},
	)

	injectedFaultsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chaos_injected_faults_total",
			Help: "Total number of injected faults per target pod.",
		},
		[]string{"target_pod", "fault_type"},
	)

	experimentDurationSeconds := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chaos_experiment_duration_seconds",
			Help:    "Execution duration of chaos experiments in seconds.",
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 8), // 0.5s, 1s, 2s, 4s, 8s, 16s, 32s, 64s
		},
		[]string{"experiment_type"},
	)

	reg.MustRegister(experimentsTotal, injectedFaultsTotal, experimentDurationSeconds)

	return &Exporter{
		Registry:                  reg,
		ExperimentsTotal:          experimentsTotal,
		InjectedFaultsTotal:       injectedFaultsTotal,
		ExperimentDurationSeconds: experimentDurationSeconds,
	}
}

// Start serves the /metrics endpoint on the specified HTTP port asynchronously.
func (e *Exporter) Start(port int) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(e.Registry, promhttp.HandlerOpts{}))

	e.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := e.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Telemetry metrics server error: %v\n", err)
		}
	}()
}

// Stop gracefully shuts down the telemetry HTTP server.
func (e *Exporter) Stop(ctx context.Context) error {
	if e.server != nil {
		return e.server.Shutdown(ctx)
	}
	return nil
}
