package identity

import (
	"strings"

	hashid "github.com/goliatone/hashid/pkg/hashid"
	"github.com/google/uuid"
)

// UUID derives a deterministic UUID from a stable key using go-hashid.
//
// Callers must ensure key construction prevents cross-entity collisions (prefix by domain/type).
func UUID(key string) uuid.UUID {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return uuid.Nil
	}
	uid, err := hashid.NewUUID(trimmed, hashid.WithHashAlgorithm(hashid.SHA256), hashid.WithNormalization(true))
	if err != nil || uid == uuid.Nil {
		return uuid.NewSHA1(uuid.NameSpaceOID, []byte(trimmed))
	}
	return uid
}

func MenuUUID(menuCode string) uuid.UUID {
	return UUID("go-cms:menu:" + strings.TrimSpace(menuCode))
}

func MenuItemUUID(menuID uuid.UUID, canonicalKey string) uuid.UUID {
	return UUID("go-cms:menu_item:" + menuID.String() + ":" + strings.TrimSpace(canonicalKey))
}

func LocaleUUID(localeCode string) uuid.UUID {
	return UUID("go-cms:locale:" + strings.ToLower(strings.TrimSpace(localeCode)))
}

func ThemeUUID(themePath string) uuid.UUID {
	return UUID("go-cms:theme:" + strings.TrimSpace(themePath))
}

func TemplateUUID(themeID uuid.UUID, slug string) uuid.UUID {
	return UUID("go-cms:template:" + themeID.String() + ":" + strings.ToLower(strings.TrimSpace(slug)))
}

func WidgetDefinitionUUID(name string) uuid.UUID {
	return UUID("go-cms:widget_definition:" + strings.ToLower(strings.TrimSpace(name)))
}

func WidgetAreaDefinitionUUID(code string) uuid.UUID {
	return UUID("go-cms:widget_area_definition:" + strings.ToLower(strings.TrimSpace(code)))
}

func BlockDefinitionUUID(name string) uuid.UUID {
	return UUID("go-cms:block_definition:" + strings.ToLower(strings.TrimSpace(name)))
}
