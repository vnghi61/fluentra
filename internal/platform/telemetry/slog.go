package telemetry

import (
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

// Logger creates an OTLP-backed, fail-closed structured logger. The OTel bridge
// adds trace_id and span_id from the active span; RedactingHandler adds request_id.
func (p *Provider) Logger(name string) *slog.Logger {
	bridge := otelslog.NewHandler(name, otelslog.WithLoggerProvider(p.loggerProvider))
	return slog.New(NewRedactingHandler(bridge, DefaultAllowedLogKeys))
}
