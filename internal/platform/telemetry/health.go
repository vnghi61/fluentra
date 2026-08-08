package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
)

// Field names in the health endpoint response bodies. These are part of the
// operational HTTP contract, so they are deliberately independent of the log
// attribute keys in redact.go that happen to share a spelling.
const (
	healthFieldStatus  = "status"
	healthFieldVersion = "version"
)

// ReadinessChecker verifies a hard dependency is usable.
type ReadinessChecker interface{ Check(context.Context) error }

// HealthHandler exposes liveness, readiness, and build version responses.
type HealthHandler struct {
	checks  []ReadinessChecker
	version string
}

// NewHealthHandler creates health endpoints backed by hard dependency checks.
func NewHealthHandler(version string, checks ...ReadinessChecker) *HealthHandler {
	return &HealthHandler{checks: checks, version: version}
}

// Health reports that the process is running. It touches no dependency, so a
// liveness probe never restarts the process because a downstream is down.
func (h *HealthHandler) Health(writer http.ResponseWriter, _ *http.Request) {
	writeHealth(writer, http.StatusOK, map[string]string{healthFieldStatus: "ok"})
}

// Ready reports 503 as soon as any hard dependency check fails, so the process
// is pulled out of rotation instead of serving requests it cannot complete.
func (h *HealthHandler) Ready(writer http.ResponseWriter, request *http.Request) {
	for _, check := range h.checks {
		if err := check.Check(request.Context()); err != nil {
			writeHealth(writer, http.StatusServiceUnavailable,
				map[string]string{healthFieldStatus: "unavailable"})
			return
		}
	}
	writeHealth(writer, http.StatusOK, map[string]string{healthFieldStatus: "ready"})
}

// Version returns the build version this process was compiled from.
func (h *HealthHandler) Version(writer http.ResponseWriter, _ *http.Request) {
	writeHealth(writer, http.StatusOK, map[string]string{healthFieldVersion: h.version})
}

func writeHealth(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
