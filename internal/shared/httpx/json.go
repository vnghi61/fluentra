package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

const (
	requestIDHeader  = "X-Request-Id"
	defaultJSONLimit = 1 << 20
)

type requestIDKey struct{}

// WithRequestID adds a request ID to a context used by response helpers.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// RequestID returns the request ID recorded by middleware.
func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}

// DecodeJSON strictly decodes one JSON object with a one-megabyte size limit.
func DecodeJSON(request *http.Request, target any) error {
	return DecodeJSONLimit(request, target, defaultJSONLimit)
}

// DecodeJSONLimit strictly decodes one JSON object within maxBytes.
func DecodeJSONLimit(request *http.Request, target any, maxBytes int64) error {
	request.Body = http.MaxBytesReader(nil, request.Body, maxBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return apperr.New(apperr.TooLarge, "PAYLOAD_TOO_LARGE", "Request body exceeds the maximum size.")
		}
		return apperr.Wrap(err, apperr.BadRequest, "MALFORMED_REQUEST", "Request body must be valid JSON.")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.New(apperr.BadRequest, "MALFORMED_REQUEST", "Request body must contain one JSON value.")
	}
	return nil
}

// WriteJSON writes a JSON response and the request ID header.
func WriteJSON(writer http.ResponseWriter, request *http.Request, status int, value any) {
	writer.Header().Set(requestIDHeader, RequestID(request.Context()))
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

// WriteProblem writes an RFC 9457 response without exposing internal error details.
func WriteProblem(writer http.ResponseWriter, request *http.Request, err error) {
	appErr := apperr.Wrap(err, apperr.Internal, "INTERNAL_ERROR", "An unexpected error occurred.")
	if appErr.RetryAfter > 0 {
		writer.Header().Set("Retry-After", strconv.Itoa(appErr.RetryAfter))
	}
	writer.Header().Set(requestIDHeader, RequestID(request.Context()))
	writer.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	writer.WriteHeader(appErr.Status())
	_ = json.NewEncoder(writer).Encode(appErr.ToProblem(request.URL.Path, RequestID(request.Context())))
}
