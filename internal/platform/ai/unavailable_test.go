package ai_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/platform/ai"
)

func statusProvider(name string, status int) *namedProvider {
	return &namedProvider{name: name, err: &ai.ProviderStatusError{
		Endpoint: "https://" + name + ".example/v1/chat/completions", Status: status, Body: "{}",
	}}
}

// A chain in which every provider is out of quota, rate limited or unpaid will
// refuse the next request too. The pool top-ups stop on this error instead of
// sending a thousand more requests into the same refusal.
func TestRouter_EveryProviderRefusingIsUnavailable(t *testing.T) {
	registry, err := ai.NewRegistry()
	require.NoError(t, err)
	router := ai.NewRouter(ai.RouterOptions{
		Prompts: registry,
		Providers: ai.NewProviderRegistry(
			statusProvider(nameGroq, http.StatusTooManyRequests),
			statusProvider(nameCerebras, http.StatusPaymentRequired),
		),
	})

	_, err = router.Complete(context.Background(), verifyRequest())

	require.Error(t, err)
	assert.ErrorIs(t, err, ai.ErrProvidersUnavailable)
	assert.Contains(t, err.Error(), "returned 429", "the providers' own messages stay in the error")
}

// One provider failing for another reason — a bad request, an outage — is not a
// chain out of quota, and the next item may well succeed.
func TestRouter_AnOrdinaryFailureIsNotUnavailable(t *testing.T) {
	registry, err := ai.NewRegistry()
	require.NoError(t, err)
	router := ai.NewRouter(ai.RouterOptions{
		Prompts: registry,
		Providers: ai.NewProviderRegistry(
			statusProvider(nameGroq, http.StatusTooManyRequests),
			statusProvider(nameCerebras, http.StatusBadRequest),
		),
	})

	_, err = router.Complete(context.Background(), verifyRequest())

	require.Error(t, err)
	assert.False(t, errors.Is(err, ai.ErrProvidersUnavailable))
}
