package telemetry

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/id"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
)

// Propagator is the one context propagator this service uses. Middleware reads
// it directly rather than through the OTel global, so extraction works even
// when the middleware runs before — or without — NewProvider. NewProvider
// installs this same value globally, so both paths agree.
var Propagator propagation.TextMapPropagator = propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	propagation.Baggage{},
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

		route := ""
		if routeName != nil {
			route = routeName(request)
		}
		route = normalizeRoute(route)
		if route == "" {
			route = "unknown"
		}
		ctx := Propagator.Extract(request.Context(), propagation.HeaderCarrier(request.Header))
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
				statusAttributes := metric.WithAttributes(
					attribute.String("route", route),
					attribute.String("method", request.Method),
					attribute.String("status_class", statusClass(response.status)),
				)
				instruments.HTTPActive.Add(ctx, -1, attributes)
				instruments.HTTPDuration.Record(ctx, duration.Seconds(), statusAttributes)
			}
			slog.InfoContext(ctx, "request completed",
				"method", request.Method,
				"route", route,
				"status", response.status,
				"duration_ms", duration.Milliseconds())
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

// normalizeRoute is a defensive backstop for a router that has not exposed its
// pattern yet. It keeps identifiers out of telemetry even in that situation.
func normalizeRoute(route string) string {
	segments := strings.Split(route, "/")
	for index, segment := range segments {
		if looksLikeIdentifier(segment) {
			segments[index] = "{id}"
		}
	}
	return strings.Join(segments, "/")
}

func looksLikeIdentifier(value string) bool {
	if value == "" {
		return false
	}
	if allDigits(value) {
		return true
	}
	if len(value) == 36 && value[8] == '-' && value[13] == '-' && value[18] == '-' && value[23] == '-' {
		return allHex(strings.ReplaceAll(value, "-", ""))
	}
	if len(value) == 26 {
		for _, character := range value {
			if (character < '0' || character > '9') && (character < 'A' || character > 'Z') {
				return false
			}
		}
		return true
	}
	return false
}

func allDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func allHex(value string) bool {
	for _, character := range value {
		isDigit := character >= '0' && character <= '9'
		isLower := character >= 'a' && character <= 'f'
		isUpper := character >= 'A' && character <= 'F'
		if !isDigit && !isLower && !isUpper {
			return false
		}
	}
	return true
}
