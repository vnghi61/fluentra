package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

// Logger creates a fail-closed structured logger that writes JSON to stdout and,
// when a collector is configured, to OTLP as well. Stdout is what a platform
// such as Render shows; the OTel bridge adds trace_id and span_id from the
// active span, and RedactingHandler adds request_id and redacts both outputs.
func (p *Provider) Logger(name string) *slog.Logger {
	handlers := []slog.Handler{slog.NewJSONHandler(os.Stdout, nil)}
	if p.exporting {
		handlers = append(handlers, otelslog.NewHandler(name, otelslog.WithLoggerProvider(p.loggerProvider)))
	}
	return slog.New(NewRedactingHandler(fanout(handlers), DefaultAllowedLogKeys))
}

// fanout sends each record to every handler that accepts its level.
type fanout []slog.Handler

func (f fanout) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f fanout) Handle(ctx context.Context, record slog.Record) error {
	var errs []error
	for _, h := range f {
		if h.Enabled(ctx, record.Level) {
			errs = append(errs, h.Handle(ctx, record.Clone()))
		}
	}
	return errors.Join(errs...)
}

func (f fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithAttrs(attrs)
	}
	return out
}

func (f fanout) WithGroup(name string) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithGroup(name)
	}
	return out
}
