package pages

import (
	"reflect"
	"strings"
	"testing"

	sharedi18n "github.com/goliatone/go-i18n"
)

func TestNormalizeAdminPageFilterValuesCanonicalizesLocaleFilters(t *testing.T) {
	values, ok := normalizeAdminPageFilterValues([]any{" ES_mx ", "fr_ca"}, func(value string) string {
		normalized := sharedi18n.NormalizeLocale(value)
		if normalized == "" {
			return ""
		}
		return strings.ToLower(normalized)
	})
	if !ok {
		t.Fatalf("expected locale filters to normalize successfully")
	}

	expected := []string{"es-mx", "fr-ca"}
	if !reflect.DeepEqual(values, expected) {
		t.Fatalf("expected canonical locale filters %#v, got %#v", expected, values)
	}
}
