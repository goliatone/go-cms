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
		"widgets.Service":                     reflect.TypeOf((*widgets.Service)(nil)).Elem(),
		"widgets.RegisterDefinitionInput":     reflect.TypeOf(widgets.RegisterDefinitionInput{}),
		"widgets.DeleteDefinitionRequest":     reflect.TypeOf(widgets.DeleteDefinitionRequest{}),
		"widgets.CreateInstanceInput":         reflect.TypeOf(widgets.CreateInstanceInput{}),
		"widgets.UpdateInstanceInput":         reflect.TypeOf(widgets.UpdateInstanceInput{}),
		"widgets.DeleteInstanceRequest":       reflect.TypeOf(widgets.DeleteInstanceRequest{}),
		"widgets.AddTranslationInput":         reflect.TypeOf(widgets.AddTranslationInput{}),
		"widgets.UpdateTranslationInput":      reflect.TypeOf(widgets.UpdateTranslationInput{}),
		"widgets.DeleteTranslationRequest":    reflect.TypeOf(widgets.DeleteTranslationRequest{}),
		"widgets.RegisterAreaDefinitionInput": reflect.TypeOf(widgets.RegisterAreaDefinitionInput{}),
		"widgets.AssignWidgetToAreaInput":     reflect.TypeOf(widgets.AssignWidgetToAreaInput{}),
		"widgets.RemoveWidgetFromAreaInput":   reflect.TypeOf(widgets.RemoveWidgetFromAreaInput{}),
		"widgets.ReorderAreaWidgetsInput":     reflect.TypeOf(widgets.ReorderAreaWidgetsInput{}),
		"widgets.ResolveAreaInput":            reflect.TypeOf(widgets.ResolveAreaInput{}),
		"widgets.VisibilityContext":           reflect.TypeOf(widgets.VisibilityContext{}),
		"widgets.Definition":                  reflect.TypeOf(widgets.Definition{}),
		"widgets.Instance":                    reflect.TypeOf(widgets.Instance{}),
		"widgets.Translation":                 reflect.TypeOf(widgets.Translation{}),
		"widgets.AreaDefinition":              reflect.TypeOf(widgets.AreaDefinition{}),
		"widgets.AreaPlacement":               reflect.TypeOf(widgets.AreaPlacement{}),
		"widgets.ResolvedWidget":              reflect.TypeOf(widgets.ResolvedWidget{}),
	}

	for name, typ := range types {
		assertNoInternalTypeRefs(t, name, typ, map[reflect.Type]bool{})
	}

	method, ok := reflect.TypeOf((*cms.Module)(nil)).MethodByName("Widgets")
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
		for i := 0; i < typ.NumField(); i++ {
			assertNoInternalTypeRefs(t, name+"."+typ.Field(i).Name, typ.Field(i).Type, seen)
		}
	case reflect.Interface:
		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			assertNoInternalTypeRefs(t, name+"."+method.Name, method.Type, seen)
		}
	case reflect.Func:
		for i := 0; i < typ.NumIn(); i++ {
			assertNoInternalTypeRefs(t, name, typ.In(i), seen)
		}
		for i := 0; i < typ.NumOut(); i++ {
			assertNoInternalTypeRefs(t, name, typ.Out(i), seen)
		}
	}
}
