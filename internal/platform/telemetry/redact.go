package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

const (
	redactedValue = "[redacted]"
	errorKey      = "error"
	requestIDKey  = "request_id"
)

// DefaultAllowedLogKeys is the reviewed, non-sensitive set shared by application
// logs. It covers every attribute the situation table in LOGGING_GUIDELINE.md §4
// requires. Adding a key here is a reviewed change (LG6): anything absent is
// redacted, so a new field fails closed.
var DefaultAllowedLogKeys = []string{
	"actor_id", "attempt", "bucket", "config_digest", "content_id", "count", "duration_ms",
	"email_hash", errorKey, "error_code", "event", "event_id", "family_id", "from_provider",
	"ip_country", "job_id", "kind", "limiter", "max_attempts", "method", "module", "op",
	"operation", "permission", "prompt_version", "provider", "queue", "reason", requestIDKey,
	"result", "route", "source", "status", "subject_hash", "submission_id", "task", "topic",
	"to_provider", "truncated", "user_id", "version",
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

// Enabled reports whether the wrapped handler accepts records at this level.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle rebuilds the record with every attribute passed through the allowlist,
// then forwards it. The request ID from ctx is attached here so callers never
// have to remember to log it.
func (h *RedactingHandler) Handle(ctx context.Context, record slog.Record) error {
	safe := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	if requestID := httpx.RequestID(ctx); requestID != "" {
		safe.AddAttrs(slog.String(requestIDKey, requestID))
	}
	for _, attr := range h.attrs {
		safe.AddAttrs(h.redact(attr))
	}
	record.Attrs(func(attr slog.Attr) bool { safe.AddAttrs(h.redact(attr)); return true })
	return h.next.Handle(ctx, safe)
}

// WithAttrs returns a copy that redacts attrs on every subsequent record.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}

// WithGroup returns a copy whose wrapped handler nests records under name.
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
		return slog.Attr{Key: attr.Key, Value: slog.GroupValue(redacted...)}
	}
	if _, ok := h.allowed[attr.Key]; !ok {
		return slog.String(attr.Key, redactedValue)
	}
	if attr.Key == errorKey {
		return slog.Attr{Key: attr.Key, Value: sanitizeError(attr.Value)}
	}
	return attr
}

// sanitizeError keeps the `error` key useful without letting a raw error string
// reach the log. A raw string routinely carries a DSN with its password, an
// email address, or a fragment of user input; an application error carries only
// its code and the message already disclosed over HTTP, so that is safe to
// keep. Anything else is reduced to its type.
func sanitizeError(value slog.Value) slog.Value {
	err, ok := value.Any().(error)
	if !ok {
		return slog.StringValue(redactedValue)
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return slog.StringValue(appErr.Error())
	}
	switch {
	case errors.Is(err, context.Canceled):
		return slog.StringValue("context canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return slog.StringValue("context deadline exceeded")
	default:
		return slog.StringValue(fmt.Sprintf("%T", err))
	}
}
