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

// ProviderConfig represents one configured AI provider slot.
type ProviderConfig struct {
	Name    string
	BaseURL string
	Model   string
	APIKey  string
	Timeout time.Duration
}

// Config is what the composition root reads from the environment.
//
// Mirrors how `mailer` is configured — a transport name plus that transport's
// settings — so that choosing a model is the same kind of decision as choosing
// how mail is sent, and is made in the same place.
type Config struct {
	// Providers declares the ordered slot chain (slots 1 to 4).
	// Slot 1 is the primary; subsequent slots are fallbacks in order.
	Providers []ProviderConfig

	// Legacy provider configuration fields (kept for backward compatibility).
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

// resolveProviderConfigs flattens the configured slots into an ordered chain.
//
// The legacy single-provider fields are read only when no slot is filled, so a
// deployment part-way through the migration to numbered slots runs on the slots
// rather than on a silent mixture of both.
func resolveProviderConfigs(config Config) []ProviderConfig {
	var configs []ProviderConfig
	for _, p := range config.Providers {
		if strings.TrimSpace(p.Name) != "" {
			configs = append(configs, p)
		}
	}
	if len(configs) > 0 {
		return configs
	}

	if name := strings.TrimSpace(config.Provider); name != "" {
		configs = append(configs, ProviderConfig{
			Name: name, BaseURL: config.BaseURL, Model: config.Model,
			APIKey: config.APIKey, Timeout: config.Timeout,
		})
	}
	if name := strings.TrimSpace(config.FallbackProvider); name != "" {
		configs = append(configs, ProviderConfig{
			Name: name, BaseURL: config.FallbackBaseURL, Model: config.FallbackModel,
			APIKey: config.FallbackAPIKey, Timeout: config.FallbackTimeout,
		})
	}
	if len(configs) == 0 {
		configs = append(configs, ProviderConfig{Name: ProviderMock})
	}
	return configs
}

// buildProviders turns the resolved chain into providers, primary first.
//
// A repeated name is refused rather than accepted. ProviderRegistry keys its
// providers by Name(), so two slots sharing one name would overwrite each other
// in the map and the chain would silently be shorter than it was configured to
// be -- which is the exact defect the named-provider work exists to remove.
func buildProviders(configs []ProviderConfig, registry *Registry) ([]Provider, error) {
	seen := make(map[string]struct{}, len(configs))
	built := make([]Provider, 0, len(configs))

	for _, pc := range configs {
		name := strings.TrimSpace(strings.ToLower(pc.Name))
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("ai: provider %q is configured twice; each slot needs its own name", name)
		}
		seen[name] = struct{}{}

		if name == ProviderMock {
			built = append(built, NewMockProvider(registry))
			continue
		}

		provider, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig(pc), registry)
		if err != nil {
			return nil, fmt.Errorf("ai: configure provider %q: %w", pc.Name, err)
		}
		built = append(built, provider)
	}
	return built, nil
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

	builtProviders, err := buildProviders(resolveProviderConfigs(config), registry)
	if err != nil {
		return nil, err
	}

	primary := builtProviders[0]
	if primary.Name() == ProviderMock {
		slog.Warn("ai: using the mock provider; verification accepts every word it is given",
			"provider", ProviderMock)
	}
	fallbacks := builtProviders[1:]

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
