package eventbus

import (
	"fmt"
	"sync"
)

// Registry manages topic-to-handler mappings in a thread-safe manner.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// NewRegistry creates a new event handler registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string][]Handler),
	}
}

// Register adds a handler for a specific topic.
func (r *Registry) Register(topic string, handler Handler) error {
	if topic == "" {
		return fmt.Errorf("topic name cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[topic] = append(r.handlers[topic], handler)
	return nil
}

// GetHandlers returns a copy of registered handlers for a topic.
func (r *Registry) GetHandlers(topic string) []Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := r.handlers[topic]
	if len(list) == 0 {
		return nil
	}

	result := make([]Handler, len(list))
	copy(result, list)
	return result
}
