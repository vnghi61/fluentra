package telemetry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// routeUserByID is the normalized (ID-free) route these tests assert on. Span and
// metric labels must carry the template, never the concrete ID.
const routeUserByID = "/api/v1/users/{id}"

func TestMiddleware_PropagatesRequestID(t *testing.T) {
	t.Parallel()
	handler := Middleware(func(*http.Request) string { return routeUserByID },
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			httpx.WriteJSON(writer, request, http.StatusOK,
				map[string]string{requestIDKey: httpx.RequestID(request.Context())})
		}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	request.Header.Set("X-Request-Id", "req-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("X-Request-Id") != "req-1" {
		t.Fatalf("status=%d request_id=%q", response.Code, response.Header().Get("X-Request-Id"))
	}
}

func TestMiddleware_RecoversPanic(t *testing.T) {
	t.Parallel()
	handler := Middleware(func(*http.Request) string { return routeUserByID },
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("unexpected")
		}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", response.Code)
	}
	if response.Header().Get("X-Request-Id") == "" {
		t.Fatal("missing request ID")
	}
}

func TestMiddleware_ExtractsTraceContext(t *testing.T) {
	t.Parallel()
	var valid bool
	handler := Middleware(func(*http.Request) string { return "/health" },
		http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			valid = trace.SpanContextFromContext(request.Context()).IsValid()
		}))
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if !valid {
		t.Fatal("middleware did not extract trace context")
	}
}

func TestStatusClass(t *testing.T) {
	t.Parallel()
	for status, want := range map[int]string{101: "1xx", 204: "2xx", 302: "3xx", 404: "4xx", 503: "5xx"} {
		if got := statusClass(status); got != want {
			t.Errorf("statusClass(%d)=%q, want %q", status, got, want)
		}
	}
}

func TestNormalizeRouteRemovesConcreteIdentifiers(t *testing.T) {
	t.Parallel()
	for route, want := range map[string]string{
		"/api/v1/users/123": routeUserByID,
		"/api/v1/users/0193a7c1-1111-7abc-8000-000000000000": routeUserByID,
		"/api/v1/users/01J8XQ7Z0ABCDE123456789012":           routeUserByID,
		"/api/v1/ping": "/api/v1/ping",
	} {
		if got := normalizeRoute(route); got != want {
			t.Errorf("normalizeRoute(%q)=%q, want %q", route, got, want)
		}
	}
}
