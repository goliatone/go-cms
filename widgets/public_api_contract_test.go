package widgets_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/widgets"
)

var _ func(*cms.Module) widgets.Service = (*cms.Module).Widgets
var _ widgets.Service = (cms.WidgetService)(nil)

func TestPublicWidgetSignaturesDoNotReferenceInternalPackages(t *testing.T) {
	t.Parallel()

	types := map[string]reflect.Type{
		"widgets.Service":                     reflect.TypeFor[widgets.Service](),
		"widgets.RegisterDefinitionInput":     reflect.TypeFor[widgets.RegisterDefinitionInput](),
		"widgets.DeleteDefinitionRequest":     reflect.TypeFor[widgets.DeleteDefinitionRequest](),
		"widgets.CreateInstanceInput":         reflect.TypeFor[widgets.CreateInstanceInput](),
		"widgets.UpdateInstanceInput":         reflect.TypeFor[widgets.UpdateInstanceInput](),
		"widgets.DeleteInstanceRequest":       reflect.TypeFor[widgets.DeleteInstanceRequest](),
		"widgets.AddTranslationInput":         reflect.TypeFor[widgets.AddTranslationInput](),
		"widgets.UpdateTranslationInput":      reflect.TypeFor[widgets.UpdateTranslationInput](),
		"widgets.DeleteTranslationRequest":    reflect.TypeFor[widgets.DeleteTranslationRequest](),
		"widgets.RegisterAreaDefinitionInput": reflect.TypeFor[widgets.RegisterAreaDefinitionInput](),
		"widgets.AssignWidgetToAreaInput":     reflect.TypeFor[widgets.AssignWidgetToAreaInput](),
		"widgets.RemoveWidgetFromAreaInput":   reflect.TypeFor[widgets.RemoveWidgetFromAreaInput](),
		"widgets.ReorderAreaWidgetsInput":     reflect.TypeFor[widgets.ReorderAreaWidgetsInput](),
		"widgets.ResolveAreaInput":            reflect.TypeFor[widgets.ResolveAreaInput](),
		"widgets.VisibilityContext":           reflect.TypeFor[widgets.VisibilityContext](),
		"widgets.Definition":                  reflect.TypeFor[widgets.Definition](),
		"widgets.Instance":                    reflect.TypeFor[widgets.Instance](),
		"widgets.Translation":                 reflect.TypeFor[widgets.Translation](),
		"widgets.AreaDefinition":              reflect.TypeFor[widgets.AreaDefinition](),
		"widgets.AreaPlacement":               reflect.TypeFor[widgets.AreaPlacement](),
		"widgets.ResolvedWidget":              reflect.TypeFor[widgets.ResolvedWidget](),
	}

	for name, typ := range types {
		assertNoInternalTypeRefs(t, name, typ, map[reflect.Type]bool{})
	}

	method, ok := reflect.TypeFor[*cms.Module]().MethodByName("Widgets")
	if !ok {
		t.Fatalf("expected cms.Module.Widgets method")
	}
	if method.Type.NumOut() != 1 {
		t.Fatalf("expected cms.Module.Widgets to return one value, got %d", method.Type.NumOut())
	}
	assertNoInternalTypeRefs(t, "cms.Module.Widgets", method.Type.Out(0), map[reflect.Type]bool{})
}

func assertNoInternalTypeRefs(t *testing.T, name string, typ reflect.Type, seen map[reflect.Type]bool) {
	t.Helper()

	if typ == nil {
		return
	}
	if seen[typ] {
		return
	}
	seen[typ] = true

	if pkgPath := typ.PkgPath(); strings.Contains(pkgPath, "/internal/") {
		t.Fatalf("%s references internal package type %s (%s)", name, typ.String(), pkgPath)
	}

	switch typ.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Chan:
		assertNoInternalTypeRefs(t, name, typ.Elem(), seen)
	case reflect.Map:
		assertNoInternalTypeRefs(t, name, typ.Key(), seen)
		assertNoInternalTypeRefs(t, name, typ.Elem(), seen)
	case reflect.Struct:
		for field := range typ.Fields() {
			assertNoInternalTypeRefs(t, name+"."+field.Name, field.Type, seen)
		}
	case reflect.Interface:
		for method := range typ.Methods() {
			method := method
			assertNoInternalTypeRefs(t, name+"."+method.Name, method.Type, seen)
		}
	case reflect.Func:
		for in := range typ.Ins() {
			assertNoInternalTypeRefs(t, name, in, seen)
		}
		for out := range typ.Outs() {
			assertNoInternalTypeRefs(t, name, out, seen)
		}
	}
}
