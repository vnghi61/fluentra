package outbox_test

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// The outbox is a process boundary: the transaction that writes an event and
// the worker that delivers it are different traces unless something carries the
// context across. These tests are that something, from both ends.

// spanContext returns a context inside a real recording span, and the trace id
// it belongs to. A no-op provider produces a zero trace id, which would make
// every assertion below pass for the wrong reason.
func spanContext(t *testing.T) (context.Context, string, func()) {
	t.Helper()

	provider := sdktrace.NewTracerProvider()
	ctx, span := provider.Tracer("outbox-test").Start(context.Background(), "producer")
	return ctx, span.SpanContext().TraceID().String(), func() {
		span.End()
		_ = provider.Shutdown(context.Background())
	}
}

func TestWriteRecordsTheCallersTraceParent(t *testing.T) {
	t.Parallel()

	ctx, traceID, done := spanContext(t)
	defer done()

	tx := &recordingTx{}
	if _, err := outbox.NewWriter().Write(ctx, tx, aggregateUser, "user."+eventCreated, map[string]string{}); err != nil {
		t.Fatalf("write: %v", err)
	}

	arguments := tx.calls[0]
	stored, ok := arguments[4].(*string)
	if !ok || stored == nil {
		t.Fatalf("traceparent argument = %#v, want the caller's span", arguments[4])
	}
	if !containsTraceID(*stored, traceID) {
		t.Errorf("traceparent %q does not carry the caller's trace %q", *stored, traceID)
	}
}

// TestWriteOutsideASpanRecordsNoTraceParent. An event written by a seed script
// or a migration has no trace, and inventing one would be worse than recording
// none: it would link the row to a trace that does not exist.
func TestWriteOutsideASpanRecordsNoTraceParent(t *testing.T) {
	t.Parallel()

	tx := &recordingTx{}
	if _, err := outbox.NewWriter().Write(
		context.Background(), tx, aggregateUser, "user."+eventCreated, map[string]string{},
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	stored, ok := tx.calls[0][4].(*string)
	if !ok {
		t.Fatalf("traceparent argument = %#v, want a *string", tx.calls[0][4])
	}
	if stored != nil {
		t.Errorf("traceparent = %q, want SQL NULL", *stored)
	}
}

// tracingDispatcher records the trace each delivery arrived in.
type tracingDispatcher struct {
	traces []string
}

func (d *tracingDispatcher) Dispatch(ctx context.Context, _ outbox.Event) error {
	d.traces = append(d.traces, trace.SpanContextFromContext(ctx).TraceID().String())
	return nil
}

// TestDispatchContinuesTheProducersTrace is the other half, and the one that
// makes BR-AUDIT-07 true: a consumer that asks its context for the trace gets
// the trace of the transaction that produced the event, not of the polling loop
// that delivered it.
//
// Nothing downstream had to change to get this. `audit` reads the trace from
// the context it is handed, and the publisher is what puts the right one there.
func TestDispatchContinuesTheProducersTrace(t *testing.T) {
	t.Parallel()

	ctx, producerTrace, done := spanContext(t)
	defer done()

	// The traceparent a producing transaction would have stored.
	tx := &recordingTx{}
	if _, err := outbox.NewWriter().Write(ctx, tx, aggregateUser, "user."+eventCreated, map[string]string{}); err != nil {
		t.Fatalf("write: %v", err)
	}
	stored, _ := tx.calls[0][4].(*string)

	// The publisher's own context is a different trace, as it is in the worker.
	consumerCtx, consumerTrace, doneConsumer := spanContext(t)
	defer doneConsumer()
	if consumerTrace == producerTrace {
		t.Fatal("the two spans landed in the same trace; the test cannot tell them apart")
	}

	dispatcher := &tracingDispatcher{}
	event := outbox.Event{Aggregate: aggregateUser, Name: eventCreated, TraceParent: *stored}
	if err := dispatcher.Dispatch(event.DispatchContext(consumerCtx), event); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if len(dispatcher.traces) != 1 {
		t.Fatalf("dispatched %d times, want 1", len(dispatcher.traces))
	}
	if dispatcher.traces[0] != producerTrace {
		t.Errorf("delivered in trace %q, want the producer's %q", dispatcher.traces[0], producerTrace)
	}
}

// TestDispatchWithoutATraceParentKeepsTheCallersContext. An event that carries
// no trace must not lose the delivery's own — a worker span is better than no
// span.
func TestDispatchWithoutATraceParentKeepsTheCallersContext(t *testing.T) {
	t.Parallel()

	consumerCtx, consumerTrace, done := spanContext(t)
	defer done()

	dispatcher := &tracingDispatcher{}
	plain := outbox.Event{Aggregate: aggregateUser, Name: eventCreated}
	if err := dispatcher.Dispatch(plain.DispatchContext(consumerCtx), plain); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if dispatcher.traces[0] != consumerTrace {
		t.Errorf("delivered in trace %q, want the caller's %q", dispatcher.traces[0], consumerTrace)
	}
}

// TestDispatchWithAMalformedTraceParentDoesNotFail. The column is checked, but
// a row written before that constraint existed, or by another writer, must not
// take the delivery down with it.
func TestDispatchWithAMalformedTraceParentDoesNotFail(t *testing.T) {
	t.Parallel()

	consumerCtx, consumerTrace, done := spanContext(t)
	defer done()

	dispatcher := &tracingDispatcher{}
	event := outbox.Event{Aggregate: aggregateUser, Name: eventCreated, TraceParent: "not-a-traceparent"}
	if err := dispatcher.Dispatch(event.DispatchContext(consumerCtx), event); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if dispatcher.traces[0] != consumerTrace {
		t.Errorf("delivered in trace %q, want the caller's %q as a fallback",
			dispatcher.traces[0], consumerTrace)
	}
}

func containsTraceID(traceparent, traceID string) bool {
	return len(traceparent) >= 3+len(traceID) && traceparent[3:3+len(traceID)] == traceID
}
