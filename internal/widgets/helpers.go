package widgets

import "github.com/google/uuid"

// translationKey formats a composite cache key for widget instance translations.
func translationKey(instanceID uuid.UUID, localeID uuid.UUID) string {
	return instanceID.String() + ":" + localeID.String()
}

// areaLocaleKey generates a stable key for area + locale combinations.
func areaLocaleKey(areaCode string, localeID *uuid.UUID) string {
	if localeID == nil {
		return areaCode + ":default"
	}
	return areaCode + ":" + localeID.String()
}
