// Package secret prevents sensitive values from being printed or logged accidentally.
package secret

import "encoding/json"

// Redacted wraps a sensitive value and always renders as [redacted].
type Redacted[T any] struct {
	value T
}

// New wraps value so it cannot be formatted accidentally.
func New[T any](value T) Redacted[T] { return Redacted[T]{value: value} }

// Reveal returns the protected value to code that explicitly needs it.
func (r Redacted[T]) Reveal() T { return r.value }

// String implements fmt.Stringer without exposing the value.
func (Redacted[T]) String() string { return "[redacted]" }

// MarshalJSON prevents a protected value being emitted in structured logs.
func (Redacted[T]) MarshalJSON() ([]byte, error) { return json.Marshal("[redacted]") }
