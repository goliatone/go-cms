package cms_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/blocks"
	"github.com/goliatone/go-cms/content"
	"github.com/goliatone/go-cms/pages"
)

var _ func(*cms.Module) content.Service = (*cms.Module).Content
var _ func(*cms.Module) content.ContentTypeService = (*cms.Module).ContentTypes
var _ func(*cms.Module) pages.Service = (*cms.Module).Pages
var _ func(*cms.Module) blocks.Service = (*cms.Module).Blocks
var _ func(*cms.Module) cms.MenuService = (*cms.Module).Menus
var _ func(*cms.Module) cms.AdminPageReadService = (*cms.Module).AdminPageRead
var _ func(*cms.Module) cms.LocaleService = (*cms.Module).Locales

var _ content.Service = (cms.ContentService)(nil)
var _ content.ContentTypeService = (cms.ContentTypeService)(nil)
var _ pages.Service = (cms.PageService)(nil)
var _ blocks.Service = (cms.BlockService)(nil)
var _ cms.MenuService = (cms.MenuService)(nil)
var _ cms.AdminPageReadService = (cms.AdminPageReadService)(nil)
var _ cms.LocaleService = (cms.LocaleService)(nil)

func TestPublicContractsDoNotReferenceInternalPackages(t *testing.T) {
	t.Parallel()

	types := map[string]reflect.Type{
		"content.Service":                         reflect.TypeFor[content.Service](),
		"content.ContentTypeService":              reflect.TypeFor[content.ContentTypeService](),
		"content.CreateContentRequest":            reflect.TypeFor[content.CreateContentRequest](),
		"content.ContentTranslationInput":         reflect.TypeFor[content.ContentTranslationInput](),
		"content.UpdateContentRequest":            reflect.TypeFor[content.UpdateContentRequest](),
		"content.DeleteContentRequest":            reflect.TypeFor[content.DeleteContentRequest](),
		"content.UpdateContentTranslationRequest": reflect.TypeFor[content.UpdateContentTranslationRequest](),
		"content.DeleteContentTranslationRequest": reflect.TypeFor[content.DeleteContentTranslationRequest](),
		"content.CreateContentDraftRequest":       reflect.TypeFor[content.CreateContentDraftRequest](),
		"content.PublishContentDraftRequest":      reflect.TypeFor[content.PublishContentDraftRequest](),
		"content.PreviewContentDraftRequest":      reflect.TypeFor[content.PreviewContentDraftRequest](),
		"content.RestoreContentVersionRequest":    reflect.TypeFor[content.RestoreContentVersionRequest](),
		"content.ContentPreview":                  reflect.TypeFor[content.ContentPreview](),
		"content.ScheduleContentRequest":          reflect.TypeFor[content.ScheduleContentRequest](),
		"content.CreateContentTypeRequest":        reflect.TypeFor[content.CreateContentTypeRequest](),
		"content.UpdateContentTypeRequest":        reflect.TypeFor[content.UpdateContentTypeRequest](),
		"content.DeleteContentTypeRequest":        reflect.TypeFor[content.DeleteContentTypeRequest](),

		"pages.Service":                      reflect.TypeFor[pages.Service](),
		"pages.CreatePageRequest":            reflect.TypeFor[pages.CreatePageRequest](),
		"pages.PageTranslationInput":         reflect.TypeFor[pages.PageTranslationInput](),
		"pages.UpdatePageRequest":            reflect.TypeFor[pages.UpdatePageRequest](),
		"pages.DeletePageRequest":            reflect.TypeFor[pages.DeletePageRequest](),
		"pages.UpdatePageTranslationRequest": reflect.TypeFor[pages.UpdatePageTranslationRequest](),
		"pages.DeletePageTranslationRequest": reflect.TypeFor[pages.DeletePageTranslationRequest](),
		"pages.MovePageRequest":              reflect.TypeFor[pages.MovePageRequest](),
		"pages.DuplicatePageRequest":         reflect.TypeFor[pages.DuplicatePageRequest](),
		"pages.CreatePageDraftRequest":       reflect.TypeFor[pages.CreatePageDraftRequest](),
		"pages.PublishPageDraftRequest":      reflect.TypeFor[pages.PublishPageDraftRequest](),
		"pages.PreviewPageDraftRequest":      reflect.TypeFor[pages.PreviewPageDraftRequest](),
		"pages.RestorePageVersionRequest":    reflect.TypeFor[pages.RestorePageVersionRequest](),
		"pages.PagePreview":                  reflect.TypeFor[pages.PagePreview](),
		"pages.SchedulePageRequest":          reflect.TypeFor[pages.SchedulePageRequest](),

		"blocks.Service":                       reflect.TypeFor[blocks.Service](),
		"blocks.RegisterDefinitionInput":       reflect.TypeFor[blocks.RegisterDefinitionInput](),
		"blocks.UpdateDefinitionInput":         reflect.TypeFor[blocks.UpdateDefinitionInput](),
		"blocks.CreateDefinitionVersionInput":  reflect.TypeFor[blocks.CreateDefinitionVersionInput](),
		"blocks.DeleteDefinitionRequest":       reflect.TypeFor[blocks.DeleteDefinitionRequest](),
		"blocks.CreateInstanceInput":           reflect.TypeFor[blocks.CreateInstanceInput](),
		"blocks.UpdateInstanceInput":           reflect.TypeFor[blocks.UpdateInstanceInput](),
		"blocks.DeleteInstanceRequest":         reflect.TypeFor[blocks.DeleteInstanceRequest](),
		"blocks.AddTranslationInput":           reflect.TypeFor[blocks.AddTranslationInput](),
		"blocks.UpdateTranslationInput":        reflect.TypeFor[blocks.UpdateTranslationInput](),
		"blocks.DeleteTranslationRequest":      reflect.TypeFor[blocks.DeleteTranslationRequest](),
		"blocks.CreateInstanceDraftRequest":    reflect.TypeFor[blocks.CreateInstanceDraftRequest](),
		"blocks.PublishInstanceDraftRequest":   reflect.TypeFor[blocks.PublishInstanceDraftRequest](),
		"blocks.RestoreInstanceVersionRequest": reflect.TypeFor[blocks.RestoreInstanceVersionRequest](),

		"cms.MenuService":               reflect.TypeFor[cms.MenuService](),
		"cms.MenuInfo":                  reflect.TypeFor[cms.MenuInfo](),
		"cms.NavigationNode":            reflect.TypeFor[cms.NavigationNode](),
		"cms.MenuItemInfo":              reflect.TypeFor[cms.MenuItemInfo](),
		"cms.MenuItemTranslationInput":  reflect.TypeFor[cms.MenuItemTranslationInput](),
		"cms.ReconcileMenuResult":       reflect.TypeFor[cms.ReconcileMenuResult](),
		"cms.UpsertMenuItemByPathInput": reflect.TypeFor[cms.UpsertMenuItemByPathInput](),
		"cms.UpdateMenuItemByPathInput": reflect.TypeFor[cms.UpdateMenuItemByPathInput](),

		"cms.AdminPageReadService":     reflect.TypeFor[cms.AdminPageReadService](),
		"cms.AdminPageRecord":          reflect.TypeFor[cms.AdminPageRecord](),
		"cms.AdminPageListOptions":     reflect.TypeFor[cms.AdminPageListOptions](),
		"cms.AdminPageGetOptions":      reflect.TypeFor[cms.AdminPageGetOptions](),
		"cms.AdminPageIncludeOptions":  reflect.TypeFor[cms.AdminPageIncludeOptions](),
		"cms.AdminPageIncludeDefaults": reflect.TypeFor[cms.AdminPageIncludeDefaults](),

		"cms.LocaleService": reflect.TypeFor[cms.LocaleService](),
		"cms.LocaleInfo":    reflect.TypeFor[cms.LocaleInfo](),
	}

	for name, typ := range types {
		assertNoInternalTypeRefs(t, name, typ, map[reflect.Type]bool{})
	}

	for _, methodName := range []string{"Content", "ContentTypes", "Pages", "Blocks", "Menus", "AdminPageRead", "Locales"} {
		method, ok := reflect.TypeFor[*cms.Module]().MethodByName(methodName)
		if !ok {
			t.Fatalf("expected cms.Module.%s method", methodName)
		}
		if method.Type.NumOut() != 1 {
			t.Fatalf("expected cms.Module.%s to return one value, got %d", methodName, method.Type.NumOut())
		}
		assertNoInternalTypeRefs(t, "cms.Module."+methodName, method.Type.Out(0), map[reflect.Type]bool{})
	}
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

	if pkgPath := typ.PkgPath(); strings.Contains(pkgPath, "/internal/") && !isAllowedInternalAliasType(typ) {
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

func isAllowedInternalAliasType(typ reflect.Type) bool {
	switch typ.PkgPath() {
	case "github.com/goliatone/go-cms/internal/domain":
		return typ.Name() == "Status"
	case "github.com/goliatone/go-cms/internal/media":
		return true
	default:
		return false
	}
}
