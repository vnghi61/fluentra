package telemetry

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/id"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
)

// Middleware creates a route-safe HTTP span, metrics, access log, and correlates each response with a request ID.
func Middleware(routeName func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := request.Header.Get("X-Request-Id")
		if requestID == "" {
			generated, err := id.NewULID(request.Context())
			if err == nil {
				requestID = generated.String()
			}
		}
		if requestID == "" {
			requestID = "unavailable"
		}
		writer.Header().Set("X-Request-Id", requestID)

		route := routeName(request)
		if route == "" {
			route = "unknown"
		}
		ctx := otel.GetTextMapPropagator().Extract(request.Context(), propagation.HeaderCarrier(request.Header))
		ctx = httpx.WithRequestID(ctx, requestID)
		ctx, span := otel.Tracer("fluentra.http").Start(ctx, request.Method+" "+route)
		started := time.Now()
		response := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
		attributes := metric.WithAttributes(attribute.String("route", route), attribute.String("method", request.Method))
		instruments, metricsEnabled := currentInstruments()
		if metricsEnabled {
			instruments.HTTPActive.Add(ctx, 1, attributes)
		}

		defer func() {
			if recovered := recover(); recovered != nil {
				span.SetStatus(codes.Error, "panic")
				span.RecordError(errors.New("panic recovered"))
				http.Error(response, "internal server error", http.StatusInternalServerError)
			}
			duration := time.Since(started)
			span.SetAttributes(
				attribute.String("http.request.method", request.Method),
				attribute.Int("http.response.status_code", response.status),
				attribute.String("request_id", requestID),
			)
			if response.status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(response.status))
			}
			if metricsEnabled {
				statusAttributes := metric.WithAttributes(attribute.String("route", route), attribute.String("method", request.Method), attribute.String("status_class", statusClass(response.status)))
				instruments.HTTPActive.Add(ctx, -1, attributes)
				instruments.HTTPDuration.Record(ctx, duration.Seconds(), statusAttributes)
			}
			slog.InfoContext(ctx, "request completed", "method", request.Method, "route", route, "status", response.status, "duration_ms", duration.Milliseconds())
			span.End()
		}()
		next.ServeHTTP(response, request.WithContext(httpx.WithRequestID(ctx, requestID)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(value []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(value)
}

func statusClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	case status >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}
