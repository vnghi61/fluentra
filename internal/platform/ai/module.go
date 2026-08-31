package ai

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// The provider names configuration may select.
const (
	ProviderMock             = "mock"
	ProviderOpenAICompatible = "openai_compatible"
)

// Config is what the composition root reads from the environment.
//
// Mirrors how `mailer` is configured — a transport name plus that transport's
// settings — so that choosing a model is the same kind of decision as choosing
// how mail is sent, and is made in the same place.
type Config struct {
	// Provider is `mock` or `openai_compatible`. Empty means mock.
	Provider string
	BaseURL  string
	Model    string
	APIKey   string
	Timeout  time.Duration
}

// New builds the client the configuration asks for.
//
// An unusable `openai_compatible` configuration — no base URL, no model — is an
// error rather than a silent fall back to the mock. Falling back would mean a
// deployment that believed it was verifying words against a real model while
// accepting every word it was given, which is worse than not starting.
func New(config Config) (Client, error) {
	registry, err := NewRegistry()
	if err != nil {
		return nil, err
	}

	switch strings.TrimSpace(strings.ToLower(config.Provider)) {
	case "", ProviderMock:
		slog.Warn("ai: using the mock provider; verification accepts every word it is given",
			"provider", ProviderMock)
		return NewMockProvider(registry), nil

	case ProviderOpenAICompatible:
		return NewOpenAICompatibleProvider(OpenAICompatibleConfig{
			BaseURL: config.BaseURL,
			Model:   config.Model,
			APIKey:  config.APIKey,
			Timeout: config.Timeout,
		}, registry)

	default:
		return nil, fmt.Errorf("ai: unknown provider %q; use %q or %q",
			config.Provider, ProviderMock, ProviderOpenAICompatible)
	}
}
