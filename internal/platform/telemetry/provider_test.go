package telemetry

import (
	"context"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/sdk/trace"
)

func TestProviderShutdownFlushesTraceExporter(t *testing.T) {
	t.Parallel()
	exporter := &recordingSpanExporter{}
	provider := &Provider{tracerProvider: trace.NewTracerProvider(trace.WithBatcher(exporter))}
	_, span := provider.tracerProvider.Tracer("test").Start(context.Background(), "operation")
	span.End()
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if !exporter.shutdownCalled || exporter.exportedCount() != 1 {
		t.Fatalf("shutdown=%t exported=%d", exporter.shutdownCalled, exporter.exportedCount())
	}
}

type recordingSpanExporter struct {
	mu             sync.Mutex
	exported       int
	shutdownCalled bool
}

func (e *recordingSpanExporter) ExportSpans(_ context.Context, spans []trace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.exported += len(spans)
	return nil
}

func (e *recordingSpanExporter) Shutdown(context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shutdownCalled = true
	return nil
}

func (e *recordingSpanExporter) exportedCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exported
}
