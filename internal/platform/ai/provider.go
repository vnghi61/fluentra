package ai

import (
	"context"
	"fmt"
)

// Provider abstracts concrete LLM API providers.
type Provider interface {
	// Name returns the provider identifier (e.g. "mock", "openai_compatible", "anthropic", "gemini").
	Name() string
	// Complete executes an AI request and returns the model response.
	Complete(ctx context.Context, req Request) (Response, error)
}

// ProviderRegistry manages configured providers and allows task-based lookup.
type ProviderRegistry struct {
	providers map[string]Provider
	primary   string
	fallbacks []string
}

// NewProviderRegistry creates a new registry with registered providers.
func NewProviderRegistry(primary Provider, fallbacks ...Provider) *ProviderRegistry {
	reg := &ProviderRegistry{
		providers: make(map[string]Provider),
	}
	if primary != nil {
		reg.primary = primary.Name()
		reg.providers[primary.Name()] = primary
	}
	for _, fb := range fallbacks {
		if fb != nil {
			reg.fallbacks = append(reg.fallbacks, fb.Name())
			reg.providers[fb.Name()] = fb
		}
	}
	return reg
}

// Get returns the named provider or error if not found.
func (r *ProviderRegistry) Get(name string) (Provider, error) {
	if p, ok := r.providers[name]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("ai: provider %q not registered", name)
}

// Primary returns the default provider.
func (r *ProviderRegistry) Primary() (Provider, error) {
	if r.primary == "" {
		return nil, ErrDisabled
	}
	return r.Get(r.primary)
}

// Fallback returns the first fallback provider if available.
func (r *ProviderRegistry) Fallback() (Provider, bool) {
	if len(r.fallbacks) == 0 {
		return nil, false
	}
	p, ok := r.providers[r.fallbacks[0]]
	return p, ok
}

// Fallbacks returns all registered fallback providers in configured order.
func (r *ProviderRegistry) Fallbacks() []Provider {
	var list []Provider
	for _, name := range r.fallbacks {
		if p, ok := r.providers[name]; ok {
			list = append(list, p)
		}
	}
	return list
}
