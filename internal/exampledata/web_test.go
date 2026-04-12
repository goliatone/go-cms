package exampledata

import (
	"testing"

	"github.com/goliatone/go-cms/internal/validation"
)

func TestDemoBlockDefinitionsUseSupportedSchemaSubset(t *testing.T) {
	for _, definition := range DemoBlockDefinitions() {
		if err := validation.ValidateSchema(definition.Schema); err != nil {
			t.Fatalf("definition %q schema failed validation: %v", definition.Name, err)
		}
	}
}
