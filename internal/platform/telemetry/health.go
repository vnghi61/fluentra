package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
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

func (h *HealthHandler) Health(writer http.ResponseWriter, _ *http.Request) {
	writeHealth(writer, http.StatusOK, map[string]string{"status": "ok"})
}
func (h *HealthHandler) Ready(writer http.ResponseWriter, request *http.Request) {
	for _, check := range h.checks {
		if err := check.Check(request.Context()); err != nil {
			writeHealth(writer, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
	}
	writeHealth(writer, http.StatusOK, map[string]string{"status": "ready"})
}
func (h *HealthHandler) Version(writer http.ResponseWriter, _ *http.Request) {
	writeHealth(writer, http.StatusOK, map[string]string{"version": h.version})
}
func writeHealth(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
