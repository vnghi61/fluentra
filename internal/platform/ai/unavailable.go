package ai

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrProvidersUnavailable means every provider in the chain refused for a reason
// an immediate retry will not fix: out of quota, rate limited, or not paid for.
//
// A batch job that sees it should stop its run. The pool top-ups kept going and
// once sent a thousand requests in three hours into refusals, which also kept a
// rate-limited fallback from ever recovering.
var ErrProvidersUnavailable = errors.New("ai: every provider is out of quota or unavailable")

// ProviderStatusError is a provider's non-2xx reply.
type ProviderStatusError struct {
	Endpoint string
	Status   int
	Body     string
}

// Error keeps the message the logs and ai_requests have always shown.
func (e *ProviderStatusError) Error() string {
	return fmt.Sprintf("ai: %s returned %d: %.300s", e.Endpoint, e.Status, e.Body)
}

// isRequestRejected reports a refusal of the request itself, which every other
// provider would refuse too, so the chain stops instead of resending it.
func isRequestRejected(err error) bool {
	var status *ProviderStatusError
	return errors.As(err, &status) && status.Status == http.StatusRequestEntityTooLarge
}

// isUnavailable reports whether err is a refusal that retrying soon will not fix.
func isUnavailable(err error) bool {
	if errors.Is(err, ErrQuotaExhausted) || errors.Is(err, ErrProvidersUnavailable) {
		return true
	}
	var status *ProviderStatusError
	if !errors.As(err, &status) {
		return false
	}
	switch status.Status {
	case http.StatusTooManyRequests, http.StatusPaymentRequired, http.StatusUnauthorized, http.StatusForbidden:
		return true
	default:
		return false
	}
}
