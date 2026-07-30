# Architectural Trade-Offs & Decision Records (ADRs)

## ADR 001: Typed client-go over HTTP / Dynamic Client
- **Decision:** Use `k8s.io/client-go` typed interface.
- **Trade-off:** Increases binary size by ~20MB, but provides compile-time safety and standard `kubeconfig` fallback handling.

## ADR 002: In-Process Goroutine Stress vs. OS Stress Tools
- **Decision:** Generate CPU load via native Go goroutines.
- **Trade-off:** Load is capped at `GOMAXPROCS` runtime limits and cannot exceed outer container limits, avoiding hard host locks while removing system binary dependencies.
