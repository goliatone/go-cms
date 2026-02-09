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
		"content.Service":                         reflect.TypeOf((*content.Service)(nil)).Elem(),
		"content.ContentTypeService":              reflect.TypeOf((*content.ContentTypeService)(nil)).Elem(),
		"content.CreateContentRequest":            reflect.TypeOf(content.CreateContentRequest{}),
		"content.ContentTranslationInput":         reflect.TypeOf(content.ContentTranslationInput{}),
		"content.UpdateContentRequest":            reflect.TypeOf(content.UpdateContentRequest{}),
		"content.DeleteContentRequest":            reflect.TypeOf(content.DeleteContentRequest{}),
		"content.UpdateContentTranslationRequest": reflect.TypeOf(content.UpdateContentTranslationRequest{}),
		"content.DeleteContentTranslationRequest": reflect.TypeOf(content.DeleteContentTranslationRequest{}),
		"content.CreateContentDraftRequest":       reflect.TypeOf(content.CreateContentDraftRequest{}),
		"content.PublishContentDraftRequest":      reflect.TypeOf(content.PublishContentDraftRequest{}),
		"content.PreviewContentDraftRequest":      reflect.TypeOf(content.PreviewContentDraftRequest{}),
		"content.RestoreContentVersionRequest":    reflect.TypeOf(content.RestoreContentVersionRequest{}),
		"content.ContentPreview":                  reflect.TypeOf(content.ContentPreview{}),
		"content.ScheduleContentRequest":          reflect.TypeOf(content.ScheduleContentRequest{}),
		"content.CreateContentTypeRequest":        reflect.TypeOf(content.CreateContentTypeRequest{}),
		"content.UpdateContentTypeRequest":        reflect.TypeOf(content.UpdateContentTypeRequest{}),
		"content.DeleteContentTypeRequest":        reflect.TypeOf(content.DeleteContentTypeRequest{}),

		"pages.Service":                      reflect.TypeOf((*pages.Service)(nil)).Elem(),
		"pages.CreatePageRequest":            reflect.TypeOf(pages.CreatePageRequest{}),
		"pages.PageTranslationInput":         reflect.TypeOf(pages.PageTranslationInput{}),
		"pages.UpdatePageRequest":            reflect.TypeOf(pages.UpdatePageRequest{}),
		"pages.DeletePageRequest":            reflect.TypeOf(pages.DeletePageRequest{}),
		"pages.UpdatePageTranslationRequest": reflect.TypeOf(pages.UpdatePageTranslationRequest{}),
		"pages.DeletePageTranslationRequest": reflect.TypeOf(pages.DeletePageTranslationRequest{}),
		"pages.MovePageRequest":              reflect.TypeOf(pages.MovePageRequest{}),
		"pages.DuplicatePageRequest":         reflect.TypeOf(pages.DuplicatePageRequest{}),
		"pages.CreatePageDraftRequest":       reflect.TypeOf(pages.CreatePageDraftRequest{}),
		"pages.PublishPageDraftRequest":      reflect.TypeOf(pages.PublishPageDraftRequest{}),
		"pages.PreviewPageDraftRequest":      reflect.TypeOf(pages.PreviewPageDraftRequest{}),
		"pages.RestorePageVersionRequest":    reflect.TypeOf(pages.RestorePageVersionRequest{}),
		"pages.PagePreview":                  reflect.TypeOf(pages.PagePreview{}),
		"pages.SchedulePageRequest":          reflect.TypeOf(pages.SchedulePageRequest{}),

		"blocks.Service":                       reflect.TypeOf((*blocks.Service)(nil)).Elem(),
		"blocks.RegisterDefinitionInput":       reflect.TypeOf(blocks.RegisterDefinitionInput{}),
		"blocks.UpdateDefinitionInput":         reflect.TypeOf(blocks.UpdateDefinitionInput{}),
		"blocks.CreateDefinitionVersionInput":  reflect.TypeOf(blocks.CreateDefinitionVersionInput{}),
		"blocks.DeleteDefinitionRequest":       reflect.TypeOf(blocks.DeleteDefinitionRequest{}),
		"blocks.CreateInstanceInput":           reflect.TypeOf(blocks.CreateInstanceInput{}),
		"blocks.UpdateInstanceInput":           reflect.TypeOf(blocks.UpdateInstanceInput{}),
		"blocks.DeleteInstanceRequest":         reflect.TypeOf(blocks.DeleteInstanceRequest{}),
		"blocks.AddTranslationInput":           reflect.TypeOf(blocks.AddTranslationInput{}),
		"blocks.UpdateTranslationInput":        reflect.TypeOf(blocks.UpdateTranslationInput{}),
		"blocks.DeleteTranslationRequest":      reflect.TypeOf(blocks.DeleteTranslationRequest{}),
		"blocks.CreateInstanceDraftRequest":    reflect.TypeOf(blocks.CreateInstanceDraftRequest{}),
		"blocks.PublishInstanceDraftRequest":   reflect.TypeOf(blocks.PublishInstanceDraftRequest{}),
		"blocks.RestoreInstanceVersionRequest": reflect.TypeOf(blocks.RestoreInstanceVersionRequest{}),

		"cms.MenuService":               reflect.TypeOf((*cms.MenuService)(nil)).Elem(),
		"cms.MenuInfo":                  reflect.TypeOf(cms.MenuInfo{}),
		"cms.NavigationNode":            reflect.TypeOf(cms.NavigationNode{}),
		"cms.MenuItemInfo":              reflect.TypeOf(cms.MenuItemInfo{}),
		"cms.MenuItemTranslationInput":  reflect.TypeOf(cms.MenuItemTranslationInput{}),
		"cms.ReconcileMenuResult":       reflect.TypeOf(cms.ReconcileMenuResult{}),
		"cms.UpsertMenuItemByPathInput": reflect.TypeOf(cms.UpsertMenuItemByPathInput{}),
		"cms.UpdateMenuItemByPathInput": reflect.TypeOf(cms.UpdateMenuItemByPathInput{}),

		"cms.AdminPageReadService":     reflect.TypeOf((*cms.AdminPageReadService)(nil)).Elem(),
		"cms.AdminPageRecord":          reflect.TypeOf(cms.AdminPageRecord{}),
		"cms.AdminPageListOptions":     reflect.TypeOf(cms.AdminPageListOptions{}),
		"cms.AdminPageGetOptions":      reflect.TypeOf(cms.AdminPageGetOptions{}),
		"cms.AdminPageIncludeOptions":  reflect.TypeOf(cms.AdminPageIncludeOptions{}),
		"cms.AdminPageIncludeDefaults": reflect.TypeOf(cms.AdminPageIncludeDefaults{}),

		"cms.LocaleService": reflect.TypeOf((*cms.LocaleService)(nil)).Elem(),
		"cms.LocaleInfo":    reflect.TypeOf(cms.LocaleInfo{}),
	}

	for name, typ := range types {
		assertNoInternalTypeRefs(t, name, typ, map[reflect.Type]bool{})
	}

	for _, methodName := range []string{"Content", "ContentTypes", "Pages", "Blocks", "Menus", "AdminPageRead", "Locales"} {
		method, ok := reflect.TypeOf((*cms.Module)(nil)).MethodByName(methodName)
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
