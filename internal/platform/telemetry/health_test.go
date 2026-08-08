package telemetry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler_HealthReadyAndVersion(t *testing.T) {
	t.Parallel()
	handler := NewHealthHandler("v0.1.0", checker{})
	for _, test := range []struct {
		handler func(http.ResponseWriter, *http.Request)
		want    int
	}{{handler.Health, 200}, {handler.Ready, 200}, {handler.Version, 200}} {
		response := httptest.NewRecorder()
		test.handler(response, httptest.NewRequest("GET", "/", nil))
		if response.Code != test.want {
			t.Fatalf("status=%d", response.Code)
		}
	}
}

func TestHealthHandler_ReadyDependencyFailure(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	NewHealthHandler("dev", checker{err: errors.New("down")}).Ready(response, httptest.NewRequest("GET", "/ready", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", response.Code)
	}
}

type checker struct{ err error }

func (c checker) Check(context.Context) error { return c.err }
