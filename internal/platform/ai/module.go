package ai

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

	// Optional database pool for persistent response caching and usage recording.
	Pool *pgxpool.Pool

	// Optional overrides for cache, usage, budget or fallback providers.
	Cache            ResponseCache
	Usage            UsageRecorder
	Budget           BudgetChecker
	FallbackProvider string
	FallbackBaseURL  string
	FallbackModel    string
	FallbackAPIKey   string
	FallbackTimeout  time.Duration
}

// New builds the client the configuration asks for, wrapped in a Router with caching
// and usage recording.
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

	var primary Provider
	switch strings.TrimSpace(strings.ToLower(config.Provider)) {
	case "", ProviderMock:
		slog.Warn("ai: using the mock provider; verification accepts every word it is given",
			"provider", ProviderMock)
		primary = NewMockProvider(registry)

	case ProviderOpenAICompatible:
		p, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
			BaseURL: config.BaseURL,
			Model:   config.Model,
			APIKey:  config.APIKey,
			Timeout: config.Timeout,
		}, registry)
		if err != nil {
			return nil, err
		}
		primary = p

	default:
		return nil, fmt.Errorf("ai: unknown provider %q; use %q or %q",
			config.Provider, ProviderMock, ProviderOpenAICompatible)
	}

	var fallbacks []Provider
	if fb := strings.TrimSpace(strings.ToLower(config.FallbackProvider)); fb != "" {
		switch fb {
		case ProviderMock:
			fallbacks = append(fallbacks, NewMockProvider(registry))
		case ProviderOpenAICompatible:
			p, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
				BaseURL: config.FallbackBaseURL,
				Model:   config.FallbackModel,
				APIKey:  config.FallbackAPIKey,
				Timeout: config.FallbackTimeout,
			}, registry)
			if err != nil {
				return nil, fmt.Errorf("ai: fallback provider configuration invalid: %w", err)
			}
			fallbacks = append(fallbacks, p)
		default:
			return nil, fmt.Errorf("ai: unknown fallback provider %q; use %q or %q",
				config.FallbackProvider, ProviderMock, ProviderOpenAICompatible)
		}
	}

	providerReg := NewProviderRegistry(primary, fallbacks...)

	// Cache selection
	cache := config.Cache
	if cache == nil {
		if config.Pool != nil {
			cache = NewDBCache(config.Pool)
		} else {
			cache = NewMemoryCache()
		}
	}

	// Usage recorder selection
	usage := config.Usage
	if usage == nil {
		if config.Pool != nil {
			usage = NewDBUsageRecorder(config.Pool)
		} else {
			usage = NoopUsageRecorder{}
		}
	}

	// Budget checker selection
	budget := config.Budget
	if budget == nil {
		if config.Pool != nil {
			budget = NewDBBudgetChecker(config.Pool)
		} else {
			budget = NoopBudgetChecker{}
		}
	}

	return NewRouter(RouterOptions{
		Prompts:   registry,
		Providers: providerReg,
		Cache:     cache,
		Usage:     usage,
		Budget:    budget,
	}), nil
}
