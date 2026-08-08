//go:build integration

package telemetry_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/platform/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// The WP0 gate is "one request, three correlated signals", and its full form
// needs Tempo, Loki and Prometheus — that part is the 15-minute exercise in
// docs/development/getting-started.md §5, and it was run.
//
// What is automated here is the half that does not need the stack: the
// properties of the request path that make the correlation possible at all. If
// one of these breaks, the exercise fails and the reason would otherwise only
// be discovered by hand.

// newRecordingTracer installs a span recorder as the global tracer provider and
// returns it, restoring the previous provider afterwards.
func newRecordingTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})
	return recorder
}

// TestWP0_RequestSpanIsNamedByRouteTemplate is the property that keeps traces
// aggregatable. A span named `GET /api/v1/users/0193a7c1-…` makes every request
// its own operation, and no backend can group them.
func TestWP0_RequestSpanIsNamedByRouteTemplate(t *testing.T) {
	recorder := newRecordingTracer(t)

	handler := telemetry.Middleware(
		func(*http.Request) string { return "/api/v1/users/{id}" },
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)
	handler.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/api/v1/users/0193a7c1-1111-7abc-8000-000000000000", nil),
	)

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatal("the middleware recorded no span")
	}
	name := spans[0].Name()
	if !strings.Contains(name, "{id}") {
		t.Errorf("span name = %q, want the route template", name)
	}
	if strings.Contains(name, "0193a7c1") {
		t.Fatalf("span name %q contains a concrete identifier", name)
	}
}

// TestWP0_IncomingTraceContextIsJoined is what makes the browser span and the
// server span one trace. Ignore the inbound traceparent and the frontend's
// trace is orphaned — which is exactly the bug P0.R13 removed on the web side.
func TestWP0_IncomingTraceContextIsJoined(t *testing.T) {
	recorder := newRecordingTracer(t)

	const (
		parentTrace = "4bf92f3577b34da6a3ce929d0e0e4736"
		parentSpan  = "00f067aa0ba902b7"
	)

	handler := telemetry.Middleware(
		func(*http.Request) string { return "/api/v1/ping" },
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	request.Header.Set("traceparent", "00-"+parentTrace+"-"+parentSpan+"-01")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatal("the middleware recorded no span")
	}
	if got := spans[0].SpanContext().TraceID().String(); got != parentTrace {
		t.Errorf("trace id = %s, want the caller's %s — the trace was not joined", got, parentTrace)
	}
	if got := spans[0].Parent().SpanID().String(); got != parentSpan {
		t.Errorf("parent span = %s, want %s", got, parentSpan)
	}
}

// TestWP0_ChildWorkNestsUnderTheRequest is the shape §5 asks a reader to see in
// Tempo: the pgx and redis spans hanging under the HTTP span.
func TestWP0_ChildWorkNestsUnderTheRequest(t *testing.T) {
	recorder := newRecordingTracer(t)

	handler := telemetry.Middleware(
		func(*http.Request) string { return "/api/v1/ping" },
		http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			for _, name := range []string{"pgx.query", "redis.ping"} {
				_, span := otel.Tracer("test").Start(request.Context(), name)
				span.SetAttributes(attribute.String("db.system", name))
				span.End()
			}
		}),
	)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil))

	var root trace.SpanContext
	children := map[string]trace.SpanID{}
	for _, span := range recorder.Ended() {
		if span.Name() == "GET /api/v1/ping" {
			root = span.SpanContext()
			continue
		}
		children[span.Name()] = span.Parent().SpanID()
	}
	if !root.IsValid() {
		t.Fatal("no request span was recorded")
	}
	for _, name := range []string{"pgx.query", "redis.ping"} {
		parent, ok := children[name]
		if !ok {
			t.Errorf("no %s span was recorded", name)
			continue
		}
		if parent != root.SpanID() {
			t.Errorf("%s is not a child of the request span", name)
		}
	}
	for _, span := range recorder.Ended() {
		if span.SpanContext().TraceID() != root.TraceID() {
			t.Errorf("span %q is in a different trace from the request", span.Name())
		}
	}
}

// TestWP0_ReadinessFailsWhenADependencyIsDown is the /ready contract: 503 while
// a hard dependency is unreachable, so the instance leaves rotation instead of
// serving requests it cannot complete. Liveness must not follow it down.
func TestWP0_ReadinessFailsWhenADependencyIsDown(t *testing.T) {
	down := errors.New("dial tcp: connection refused")
	failing := false

	health := telemetry.NewHealthHandler("test", checkFunc(func(context.Context) error {
		if failing {
			return down
		}
		return nil
	}))

	probe := func(handler http.HandlerFunc) int {
		response := httptest.NewRecorder()
		handler(response, httptest.NewRequest(http.MethodGet, "/ready", nil))
		return response.Code
	}

	if got := probe(health.Ready); got != http.StatusOK {
		t.Fatalf("/ready with a healthy dependency = %d, want 200", got)
	}

	failing = true
	if got := probe(health.Ready); got != http.StatusServiceUnavailable {
		t.Errorf("/ready with a dead dependency = %d, want 503", got)
	}
	if got := probe(health.Health); got != http.StatusOK {
		t.Errorf("/health with a dead dependency = %d, want 200; liveness must not restart the "+
			"process because a downstream is down", got)
	}

	failing = false
	if got := probe(health.Ready); got != http.StatusOK {
		t.Errorf("/ready after recovery = %d, want 200", got)
	}
}

type checkFunc func(context.Context) error

func (c checkFunc) Check(ctx context.Context) error { return c(ctx) }

// TestWP0_PropagatorIsW3CTraceContext keeps the wire format the one every other
// component in the stack speaks.
func TestWP0_PropagatorIsW3CTraceContext(t *testing.T) {
	carrier := propagation.MapCarrier{}
	traceID := trace.TraceID{
		0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6,
		0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36,
	}
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     trace.SpanID{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7},
		TraceFlags: trace.FlagsSampled,
	}))
	telemetry.Propagator.Inject(ctx, carrier)

	if got := carrier.Get("traceparent"); !strings.HasPrefix(got, "00-4bf92f3577b34da6a3ce929d0e0e4736-") {
		t.Fatalf("traceparent = %q, want W3C trace context naming the active span", got)
	}
}
