package widgets

import (
	"context"
	"strings"
	"sync"
)

// DefinitionFactory returns the registration input for a widget definition.
type DefinitionFactory func() RegisterDefinitionInput

// InstanceFactory can generate configuration payloads for a widget instance.
type InstanceFactory func(ctx context.Context, definition *Definition, input CreateInstanceInput) (map[string]any, error)

// Registration bundles a definition factory with an optional instance factory.
type Registration struct {
	Definition      DefinitionFactory
	InstanceFactory InstanceFactory
}

// Registry stores built-in and host-defined widget registrations.
type Registry struct {
	mu            sync.RWMutex
	registrations map[string]Registration
}

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		registrations: make(map[string]Registration),
	}
}

// Register adds a static definition input to the registry
func (r *Registry) Register(input RegisterDefinitionInput) {
	r.RegisterFactory(input.Name, Registration{
		Definition: func() RegisterDefinitionInput { return input },
	})
}

// RegisterFactory adds a definition factory (and optional instance factory) to the registry
func (r *Registry) RegisterFactory(key string, registration Registration) {
	if registration.Definition == nil {
		return
	}
	name := canonicalKey(key)
	if name == "" {
		next := registration.Definition()
		name = canonicalKey(next.Name)
	}
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.registrations == nil {
		r.registrations = make(map[string]Registration)
	}
	r.registrations[name] = registration
}

// List returns all registered widget definition inputs.
func (r *Registry) List() []RegisterDefinitionInput {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]RegisterDefinitionInput, 0, len(r.registrations))
	for _, registration := range r.registrations {
		if registration.Definition == nil {
			continue
		}
		out = append(out, registration.Definition())
	}
	return out
}

// InstanceFactory resolves a registered instance factory by widget name.
func (r *Registry) InstanceFactory(name string) InstanceFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.registrations == nil {
		return nil
	}
	entry, ok := r.registrations[canonicalKey(name)]
	if !ok {
		return nil
	}
	return entry.InstanceFactory
}

func canonicalKey(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}
