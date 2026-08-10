package httpx

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// TraceID returns the W3C trace id of the span the context is in, or "" when
// there is none.
//
// It lives beside WithRequestID and ActorFrom because it is the same kind of
// fact: something middleware established once, that a handler or a service
// reads without caring how it got there. Putting it here is also what keeps
// the OpenTelemetry SDK out of the business modules — `audit` records a trace
// id on every entry (BR-AUDIT-07) and may not import a vendor to do it.
//
// The value is the hex form the `traceparent` header carries, so an audit row
// can be pasted straight into Tempo.
func TraceID(ctx context.Context) string {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.HasTraceID() {
		return ""
	}
	return spanContext.TraceID().String()
}
