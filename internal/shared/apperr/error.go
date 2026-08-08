// Package apperr defines application errors and RFC 9457 Problem Details rendering.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Kind classifies an application error into an HTTP status category.
type Kind string

const (
	Validation         Kind = "validation"
	BadRequest         Kind = "bad_request"
	Unauthenticated    Kind = "unauthenticated"
	Forbidden          Kind = "forbidden"
	NotFound           Kind = "not_found"
	Conflict           Kind = "conflict"
	PreconditionFailed Kind = "precondition_failed"
	TooLarge           Kind = "too_large"
	RateLimited        Kind = "rate_limited"
	Unavailable        Kind = "unavailable"
	Timeout            Kind = "timeout"
	Internal           Kind = "internal"
)

// FieldViolation describes one invalid input field.
type FieldViolation struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error is a safe, typed error that may be rendered at an HTTP boundary.
type Error struct {
	Kind       Kind             `json:"-"`
	Code       string           `json:"code"`
	Message    string           `json:"-"`
	Fields     []FieldViolation `json:"errors,omitempty"`
	Meta       map[string]any   `json:"meta,omitempty"`
	Retryable  bool             `json:"-"`
	RetryAfter int              `json:"-"`
	cause      error
	internal   string
}

// New creates a classified application error.
func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// Wrap classifies err unless it is already an application error.
func Wrap(err error, kind Kind, code, message string) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return New(kind, code, message).WithCause(err)
}

// Is reports whether err is an application error with kind.
func Is(err error, kind Kind) bool {
	var appErr *Error
	return errors.As(err, &appErr) && appErr.Kind == kind
}

// Error implements error without exposing an internal cause.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the cause to Go error inspection, never to JSON rendering.
func (e *Error) Unwrap() error { return e.cause }

// Cause returns the underlying error for boundary logging.
func (e *Error) Cause() error { return e.cause }

// InternalDetail returns developer-only context for boundary logging.
func (e *Error) InternalDetail() string { return e.internal }

// WithCause attaches an underlying error.
func (e *Error) WithCause(cause error) *Error { e.cause = cause; return e }

// WithInternal attaches developer-only context.
func (e *Error) WithInternal(detail string) *Error { e.internal = detail; return e }

// WithMeta adds safe structured metadata.
func (e *Error) WithMeta(key string, value any) *Error {
	if e.Meta == nil {
		e.Meta = make(map[string]any)
	}
	e.Meta[key] = value
	return e
}

// WithFields adds validation failures.
func (e *Error) WithFields(fields ...FieldViolation) *Error {
	e.Fields = append(e.Fields, fields...)
	return e
}

// WithRetryAfter marks an error retryable and sets a response hint in seconds.
func (e *Error) WithRetryAfter(seconds int) *Error {
	e.Retryable = true
	e.RetryAfter = seconds
	return e
}

// Status returns the HTTP status associated with the error kind.
func (e *Error) Status() int {
	if e == nil {
		return http.StatusInternalServerError
	}
	return statusFor(e.Kind)
}

// Problem is the RFC 9457 response body.
type Problem struct {
	Type      string           `json:"type"`
	Title     string           `json:"title"`
	Status    int              `json:"status"`
	Detail    string           `json:"detail"`
	Instance  string           `json:"instance,omitempty"`
	Code      string           `json:"code"`
	RequestID string           `json:"request_id,omitempty"`
	Errors    []FieldViolation `json:"errors,omitempty"`
	Meta      map[string]any   `json:"meta,omitempty"`
}

// ToProblem renders only information that is safe to disclose to a client.
func (e *Error) ToProblem(instance, requestID string) Problem {
	if e == nil {
		e = New(Internal, "INTERNAL_ERROR", "An unexpected error occurred.")
	}
	return Problem{
		Type:  "https://fluentra.dev/errors/" + e.Code,
		Title: titleFor(e.Kind), Status: e.Status(), Detail: e.Message,
		Instance: instance, Code: e.Code, RequestID: requestID, Errors: e.Fields, Meta: e.Meta,
	}
}

func statusFor(kind Kind) int {
	switch kind {
	case Validation:
		return http.StatusUnprocessableEntity
	case BadRequest:
		return http.StatusBadRequest
	case Unauthenticated:
		return http.StatusUnauthorized
	case Forbidden:
		return http.StatusForbidden
	case NotFound:
		return http.StatusNotFound
	case Conflict:
		return http.StatusConflict
	case PreconditionFailed:
		return http.StatusPreconditionFailed
	case TooLarge:
		return http.StatusRequestEntityTooLarge
	case RateLimited:
		return http.StatusTooManyRequests
	case Unavailable:
		return http.StatusServiceUnavailable
	case Timeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

func titleFor(kind Kind) string {
	switch kind {
	case Validation:
		return "Validation failed"
	case BadRequest:
		return "Malformed request"
	case Unauthenticated:
		return "Authentication required"
	case Forbidden:
		return "Permission denied"
	case NotFound:
		return "Resource not found"
	case Conflict:
		return "Conflict"
	case PreconditionFailed:
		return "Precondition failed"
	case TooLarge:
		return "Payload too large"
	case RateLimited:
		return "Rate limited"
	case Unavailable:
		return "Service unavailable"
	case Timeout:
		return "Request timed out"
	default:
		return "Internal server error"
	}
}
