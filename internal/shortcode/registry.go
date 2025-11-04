package shortcode

import (
	"sort"
	"strings"
	"sync"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Registry is the thread-safe in-memory implementation of interfaces.ShortcodeRegistry.
type Registry struct {
	mu          sync.RWMutex
	definitions map[string]interfaces.ShortcodeDefinition
	validator   DefinitionValidator
}

// DefinitionValidator abstracts definition validation so callers can customise behaviour in tests.
type DefinitionValidator interface {
	ValidateDefinition(def interfaces.ShortcodeDefinition) error
}

// NewRegistry constructs a registry using the supplied validator.
func NewRegistry(validator DefinitionValidator) *Registry {
	return &Registry{
		definitions: make(map[string]interfaces.ShortcodeDefinition),
		validator:   validator,
	}
}

// Register stores a definition if it passes validation and the name is not taken.
func (r *Registry) Register(def interfaces.ShortcodeDefinition) error {
	name := strings.TrimSpace(strings.ToLower(def.Name))
	if name == "" {
		return ErrInvalidDefinition
	}

	if r.validator != nil {
		if err := r.validator.ValidateDefinition(def); err != nil {
			return err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.definitions[name]; exists {
		return ErrDuplicateDefinition
	}

	r.definitions[name] = def
	return nil
}

// Get returns the stored definition.
func (r *Registry) Get(name string) (interfaces.ShortcodeDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	def, ok := r.definitions[strings.ToLower(name)]
	return def, ok
}

// List returns all registered definitions in name order.
func (r *Registry) List() []interfaces.ShortcodeDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]interfaces.ShortcodeDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		result = append(result, def)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Remove deletes the definition if it exists.
func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.definitions, strings.ToLower(name))
}

// Ensure Registry implements interfaces.ShortcodeRegistry.
var _ interfaces.ShortcodeRegistry = (*Registry)(nil)
