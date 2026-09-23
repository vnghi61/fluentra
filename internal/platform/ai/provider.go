package ai

import (
	"context"
	"fmt"
	"strings"
)

// Provider abstracts concrete LLM API providers.
type Provider interface {
	// Name returns the provider identifier (e.g. "mock", "openai_compatible", "anthropic", "gemini").
	Name() string
	// Model returns the model this provider is configured to answer with. It is
	// what lets a caller exclude a specific model from the chain (Request
	// .ExcludeModel) — two slots configured with the same model are the same
	// model, however many slot numbers they occupy.
	Model() string
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

// Chain returns the ordered provider chain, primary first, with every provider
// configured with the excluded model removed.
//
// Comparison is on the model, not the slot: two slots may name the same model,
// and a verifier that excluded "the other slot" while still being answered by
// the writer's model would not be independent at all. An empty exclude keeps
// the whole chain.
func (r *ProviderRegistry) Chain(exclude string) []Provider {
	var list []Provider
	if r.primary != "" {
		if p, ok := r.providers[r.primary]; ok && !sameModel(p, exclude) {
			list = append(list, p)
		}
	}
	for _, name := range r.fallbacks {
		if p, ok := r.providers[name]; ok && !sameModel(p, exclude) {
			list = append(list, p)
		}
	}
	return list
}

// sameModel reports whether a provider answers with the excluded model.
func sameModel(p Provider, exclude string) bool {
	exclude = strings.TrimSpace(exclude)
	return exclude != "" && strings.EqualFold(strings.TrimSpace(p.Model()), exclude)
}
