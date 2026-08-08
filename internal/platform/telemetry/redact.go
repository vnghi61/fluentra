package telemetry

import (
	"context"
	"log/slog"

	"github.com/fluentra/fluentra/internal/shared/httpx"
)

const redactedValue = "[redacted]"

// DefaultAllowedLogKeys is the reviewed, non-sensitive set shared by application logs.
var DefaultAllowedLogKeys = []string{
	"attempt", "bucket", "duration_ms", "error_code", "kind", "method", "module", "operation", "queue", "request_id", "result", "route", "status", "user_id",
}

// RedactingHandler permits only reviewed attribute keys to reach log output.
type RedactingHandler struct {
	next    slog.Handler
	allowed map[string]struct{}
	attrs   []slog.Attr
}

// NewRedactingHandler creates a fail-closed logging handler.
func NewRedactingHandler(next slog.Handler, allowed []string) *RedactingHandler {
	keys := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		keys[key] = struct{}{}
	}
	return &RedactingHandler{next: next, allowed: keys}
}

func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}
func (h *RedactingHandler) Handle(ctx context.Context, record slog.Record) error {
	copy := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	if requestID := httpx.RequestID(ctx); requestID != "" {
		copy.AddAttrs(slog.String("request_id", requestID))
	}
	for _, attr := range h.attrs {
		copy.AddAttrs(h.redact(attr))
	}
	record.Attrs(func(attr slog.Attr) bool { copy.AddAttrs(h.redact(attr)); return true })
	return h.next.Handle(ctx, copy)
}
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	clone := *h
	clone.next = h.next.WithGroup(name)
	return &clone
}
func (h *RedactingHandler) redact(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		attrs := attr.Value.Group()
		redacted := make([]slog.Attr, 0, len(attrs))
		for _, child := range attrs {
			redacted = append(redacted, h.redact(child))
		}
		return slog.Group(attr.Key, redacted...)
	}
	if _, ok := h.allowed[attr.Key]; ok {
		return attr
	}
	return slog.String(attr.Key, redactedValue)
}
