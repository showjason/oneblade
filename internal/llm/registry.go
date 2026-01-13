package llm

import (
	"errors"
	"fmt"
	"sync"

	"github.com/go-kratos/blades"
)

// ModelRegistry manages the lifecycle and access to model providers
type ModelRegistry struct {
	mu     sync.RWMutex
	models map[string]blades.ModelProvider
}

// NewRegistry creates a new ModelRegistry
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models: make(map[string]blades.ModelProvider),
	}
}

// Register registers a model provider with a given name
func (r *ModelRegistry) Register(name string, model blades.ModelProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models[name] = model
}

// Get retrieves a model provider by name
func (r *ModelRegistry) Get(name string) (blades.ModelProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, ok := r.models[name]
	if !ok {
		return nil, fmt.Errorf("model provider for agent %s not found", name)
	}
	return model, nil
}

// Close closes all registered models that implement the Closer interface
func (r *ModelRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for name, m := range r.models {
		if closer, ok := m.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close model %s: %w", name, err))
			}
		}
	}

	return errors.Join(errs...)
}
