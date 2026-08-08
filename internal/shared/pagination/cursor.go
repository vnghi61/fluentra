package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// SortValue is a supported stable cursor sort value.
type SortValue interface {
	string | int | int64 | time.Time
}

// Cursor is an opaque keyset position with a stable ID tie-breaker.
type Cursor[T SortValue] struct {
	SortValue T
	ID        string
}

type encodedCursor struct {
	Type      string `json:"t"`
	SortValue string `json:"s"`
	ID        string `json:"i"`
}

// Encode returns an opaque base64 cursor.
func Encode[T SortValue](cursor Cursor[T]) (string, error) {
	data, _ := json.Marshal(encode(cursor))
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// Decode converts an opaque cursor into the caller's requested sort-value type.
func Decode[T SortValue](value string) (Cursor[T], error) {
	var empty Cursor[T]
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return empty, fmt.Errorf("decode cursor: %w", err)
	}
	var encoded encodedCursor
	if err := json.Unmarshal(data, &encoded); err != nil {
		return empty, fmt.Errorf("unmarshal cursor: %w", err)
	}
	sortValue, err := decodeValue[T](encoded)
	if err != nil {
		return empty, err
	}
	return Cursor[T]{SortValue: sortValue, ID: encoded.ID}, nil
}

func encode[T SortValue](cursor Cursor[T]) encodedCursor {
	value := fmt.Sprint(cursor.SortValue)
	kind := "string"
	if _, ok := any(cursor.SortValue).(int); ok {
		kind = "int"
	}
	if _, ok := any(cursor.SortValue).((int64)); ok {
		kind = "int64"
	}
	if timestamp, ok := any(cursor.SortValue).(time.Time); ok {
		kind, value = "time", timestamp.UTC().Format(time.RFC3339Nano)
	}
	return encodedCursor{Type: kind, SortValue: value, ID: cursor.ID}
}

func decodeValue[T SortValue](encoded encodedCursor) (T, error) {
	var zero T
	if _, ok := any(zero).(string); ok {
		if encoded.Type != "string" {
			return zero, fmt.Errorf("cursor type %q is not string", encoded.Type)
		}
		return any(encoded.SortValue).(T), nil
	}
	if _, ok := any(zero).(int); ok {
		if encoded.Type != "int" {
			return zero, fmt.Errorf("cursor type %q is not int", encoded.Type)
		}
		value, err := strconv.Atoi(encoded.SortValue)
		if err != nil {
			return zero, fmt.Errorf("parse cursor int: %w", err)
		}
		return any(value).(T), nil
	}
	if _, ok := any(zero).(int64); ok {
		if encoded.Type != "int64" {
			return zero, fmt.Errorf("cursor type %q is not int64", encoded.Type)
		}
		value, err := strconv.ParseInt(encoded.SortValue, 10, 64)
		if err != nil {
			return zero, fmt.Errorf("parse cursor int64: %w", err)
		}
		return any(value).(T), nil
	}
	if encoded.Type != "time" {
		return zero, fmt.Errorf("cursor type %q is not time", encoded.Type)
	}
	value, err := time.Parse(time.RFC3339Nano, encoded.SortValue)
	if err != nil {
		return zero, fmt.Errorf("parse cursor time: %w", err)
	}
	return any(value).(T), nil
}
