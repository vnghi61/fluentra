// Package http is the user module's HTTP surface: decode, validate the shape,
// call the service, render. It holds no business rules and touches no SQL.
package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// maxBodyBytes is generous for these two endpoints and still small enough that
// a hostile body cannot cost anything. shared/httpx uses one megabyte for the
// general case; a preference set is a few hundred bytes.
const maxBodyBytes = 16 << 10

// body is a decoded JSON object with each member still unparsed, which is what
// makes "the client did not send this field" and "the client sent null"
// distinguishable. A struct with pointer fields collapses those two, and for a
// PATCH they mean opposite things.
type body map[string]json.RawMessage

// decodeBody reads exactly one JSON object and rejects any member the
// operation does not define.
//
// An unknown field is a 422 and not a 400, and not silently ignored, because a
// client that misspells `display_name` needs to be told. Ignoring it returns
// 200 for a request that changed nothing, which is indistinguishable from
// success until somebody notices their name never updates.
func decodeBody(request *http.Request, allowed []string) (body, error) {
	limited := http.MaxBytesReader(nil, request.Body, maxBodyBytes)
	decoder := json.NewDecoder(limited)

	var decoded body
	if err := decoder.Decode(&decoded); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return nil, apperr.New(apperr.TooLarge, "PAYLOAD_TOO_LARGE", "Request body exceeds the maximum size.")
		}
		return nil, apperr.Wrap(err, apperr.BadRequest, "MALFORMED_REQUEST", "Request body must be a JSON object.")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, apperr.New(apperr.BadRequest, "MALFORMED_REQUEST", "Request body must contain one JSON value.")
	}
	if decoded == nil {
		return nil, apperr.New(apperr.BadRequest, "MALFORMED_REQUEST", "Request body must be a JSON object.")
	}

	violations := make([]apperr.FieldViolation, 0)
	for field := range decoded {
		if !slices.Contains(allowed, field) {
			violations = append(violations, apperr.FieldViolation{
				Field:   field,
				Code:    "UNKNOWN_FIELD",
				Message: fmt.Sprintf("%s is not a field of this resource.", field),
			})
		}
	}
	if len(violations) > 0 {
		// Sorted so that a body with two unknown fields produces the same
		// response every time: map iteration order is not stable, and a
		// response that reorders itself is a nuisance to assert on.
		slices.SortFunc(violations, func(a, b apperr.FieldViolation) int {
			return strings.Compare(a.Field, b.Field)
		})
		return nil, validationFailed().WithFields(violations...)
	}
	return decoded, nil
}

// present reports whether the client sent the field with a value. An explicit
// null is not a value: these fields are not nullable in the schema, so null is
// rejected rather than read as "clear it".
func (b body) present(field string) bool {
	raw, ok := b[field]
	return ok && string(raw) != "null"
}

// isNull reports whether the client sent the field as an explicit null.
func (b body) isNull(field string) bool {
	raw, ok := b[field]
	return ok && string(raw) == "null"
}

// readString unmarshals a string member.
func readString(source body, field string, target *string) error {
	return readInto(source, field, target)
}

// readInto unmarshals one member into target, turning a type mismatch into a
// field-level 422 rather than the decoder's message, which names Go types the
// client has never heard of.
func readInto(source body, field string, target any) error {
	raw, ok := source[field]
	if !ok {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return validationFailed().WithFields(apperr.FieldViolation{
			Field:   field,
			Code:    "TYPE",
			Message: fmt.Sprintf("%s has the wrong type.", field),
		})
	}
	return nil
}

// rejectNulls turns every explicitly null member into a field violation.
func rejectNulls(source body, fields []string) error {
	violations := make([]apperr.FieldViolation, 0)
	for _, field := range fields {
		if source.isNull(field) {
			violations = append(violations, apperr.FieldViolation{
				Field:   field,
				Code:    "NOT_NULLABLE",
				Message: fmt.Sprintf("%s cannot be null. Omit it to leave it unchanged.", field),
			})
		}
	}
	if len(violations) > 0 {
		return validationFailed().WithFields(violations...)
	}
	return nil
}

// requireFields reports the members a full replacement must carry.
func requireFields(source body, fields []string) error {
	violations := make([]apperr.FieldViolation, 0)
	for _, field := range fields {
		if !source.present(field) {
			violations = append(violations, apperr.FieldViolation{
				Field:   field,
				Code:    "REQUIRED",
				Message: fmt.Sprintf("%s is required.", field),
			})
		}
	}
	if len(violations) > 0 {
		return validationFailed().WithFields(violations...)
	}
	return nil
}

func validationFailed() *apperr.Error {
	return apperr.New(apperr.Validation, "VALIDATION_FAILED", "One or more request fields are invalid.")
}
