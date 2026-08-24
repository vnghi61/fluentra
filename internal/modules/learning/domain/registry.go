package domain

import (
	"fmt"
	"sync"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
)

// GraderRegistry manages the association between activity kinds and their respective ExerciseGraders.
type GraderRegistry struct {
	mu      sync.RWMutex
	graders map[string]contract.ExerciseGrader
}

// NewGraderRegistry constructs an empty GraderRegistry.
func NewGraderRegistry() *GraderRegistry {
	return &GraderRegistry{
		graders: make(map[string]contract.ExerciseGrader),
	}
}

// Register adds or replaces an ExerciseGrader for a given activity kind.
func (r *GraderRegistry) Register(kind string, grader contract.ExerciseGrader) error {
	if kind == "" {
		return fmt.Errorf("cannot register grader for empty activity kind")
	}
	if grader == nil {
		return fmt.Errorf("cannot register nil grader for activity kind %q", kind)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.graders[kind] = grader
	return nil
}

// Get retrieves the ExerciseGrader registered for a given activity kind.
func (r *GraderRegistry) Get(kind string) (contract.ExerciseGrader, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	grader, ok := r.graders[kind]
	return grader, ok
}

// Validate ensures that every required activity kind has a non-nil registered grader (Trap 2).
// Fails startup with the specific kind named if missing.
func (r *GraderRegistry) Validate(requiredKinds []string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, kind := range requiredKinds {
		grader, ok := r.graders[kind]
		if !ok || grader == nil {
			return fmt.Errorf("grader registry validation failed: missing grader for required activity kind %q", kind)
		}
	}
	return nil
}
