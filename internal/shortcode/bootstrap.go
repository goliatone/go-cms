package shortcode

import (
	"fmt"
	"strings"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// RegisterBuiltIns registers the built-in shortcode definitions on the provided registry.
// When names is empty, every built-in shortcode is registered.
func RegisterBuiltIns(registry interfaces.ShortcodeRegistry, names []string) error {
	if registry == nil {
		return fmt.Errorf("shortcode: registry is required")
	}

	available := make(map[string]interfaces.ShortcodeDefinition)
	for _, def := range BuiltInDefinitions() {
		available[strings.ToLower(strings.TrimSpace(def.Name))] = def
	}

	if len(names) == 0 {
		for _, def := range available {
			if err := registry.Register(def); err != nil {
				return err
			}
		}
		return nil
	}

	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		def, ok := available[key]
		if !ok {
			return fmt.Errorf("shortcode: built-in %q not found", name)
		}
		if err := registry.Register(def); err != nil {
			return err
		}
	}
	return nil
}
