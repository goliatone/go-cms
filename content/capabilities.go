package content

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

const (
	defaultNavigationLocation  = "site.main"
	defaultNavigationMergeMode = "append"
)

// ContentTypeCapabilityContracts captures normalized capability payloads and
// per-field validation metadata for callers that need canonical contracts.
type ContentTypeCapabilityContracts struct {
	Normalized           map[string]any
	Delivery             map[string]any
	Navigation           map[string]any
	Search               map[string]any
	Validation           map[string]string
	MigratedDeliveryMenu bool
}

// ReadContentTypeCapabilityContracts resolves normalized capability contracts
// from a content type payload.
func ReadContentTypeCapabilityContracts(contentType ContentType) ContentTypeCapabilityContracts {
	return ParseContentTypeCapabilityContracts(contentType.Capabilities)
}

// ParseContentTypeCapabilityContracts resolves normalized capability contracts
// from a raw capabilities payload.
func ParseContentTypeCapabilityContracts(capabilities map[string]any) ContentTypeCapabilityContracts {
	normalized, validation, migrated := normalizeContentTypeCapabilitiesInternal(capabilities)
	return ContentTypeCapabilityContracts{
		Normalized:           normalized,
		Delivery:             normalizeCapabilityMap(normalized["delivery"]),
		Navigation:           normalizeCapabilityMap(normalized["navigation"]),
		Search:               normalizeCapabilityMap(normalized["search"]),
		Validation:           validation,
		MigratedDeliveryMenu: migrated,
	}
}

// NormalizeContentTypeCapabilities returns canonical capability objects and
// validation metadata without mutating the input payload.
func NormalizeContentTypeCapabilities(capabilities map[string]any) (map[string]any, map[string]string) {
	normalized, validation, _ := normalizeContentTypeCapabilitiesInternal(capabilities)
	return normalized, validation
}

// ValidateAndNormalizeContentTypeCapabilities validates and canonicalizes
// capability payloads.
func ValidateAndNormalizeContentTypeCapabilities(capabilities map[string]any) (map[string]any, error) {
	normalized, validation := NormalizeContentTypeCapabilities(capabilities)
	if len(validation) == 0 {
		return normalized, nil
	}
	return nil, &ContentTypeCapabilityValidationError{Fields: validation}
}

// BackfillContentTypeNavigationDefaults normalizes persisted content type
// capabilities and writes canonical navigation defaults when needed.
func BackfillContentTypeNavigationDefaults(ctx context.Context, service ContentTypeService, env ...string) (int, error) {
	if service == nil {
		return 0, errors.New("content type service unavailable")
	}
	records, err := service.List(ctx, env...)
	if err != nil {
		return 0, err
	}
	updated := 0
	environmentKey := ""
	if len(env) > 0 {
		environmentKey = strings.TrimSpace(env[0])
	}
	for _, record := range records {
		if record == nil {
			continue
		}
		normalized, validation := NormalizeContentTypeCapabilities(record.Capabilities)
		if len(validation) > 0 {
			return updated, &ContentTypeCapabilityValidationError{Fields: validation}
		}
		if reflect.DeepEqual(normalized, record.Capabilities) {
			continue
		}
		_, err := service.Update(ctx, UpdateContentTypeRequest{
			ID:             record.ID,
			Capabilities:   normalized,
			EnvironmentKey: environmentKey,
		})
		if err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

func normalizeContentTypeCapabilitiesInternal(capabilities map[string]any) (map[string]any, map[string]string, bool) {
	sourceWasNil := capabilities == nil
	normalized := cloneCapabilityMap(capabilities)
	if normalized == nil {
		normalized = map[string]any{}
	}

	validation := map[string]string{}

	delivery := normalizeCapabilityMap(normalized["delivery"])
	delivery = mergeFlatDeliveryAliases(normalized, delivery)

	navigation := normalizeCapabilityMap(normalized["navigation"])
	navigation = mergeFlatNavigationAliases(normalized, navigation)

	search := normalizeCapabilityMap(normalized["search"])
	search = mergeFlatSearchAliases(normalized, search, validation)

	legacyDeliveryMenu := false
	delivery, navigation, legacyDeliveryMenu = migrateLegacyDeliveryMenu(delivery, navigation)

	delivery = normalizeDeliveryContract(delivery, validation)
	navigation = normalizeNavigationContract(navigation, validation)
	search = normalizeSearchContract(search, validation)

	if len(delivery) > 0 {
		normalized["delivery"] = delivery
	} else {
		delete(normalized, "delivery")
	}
	if len(navigation) > 0 {
		normalized["navigation"] = navigation
	} else {
		delete(normalized, "navigation")
	}
	if len(search) > 0 {
		normalized["search"] = search
	} else {
		delete(normalized, "search")
	}

	if len(normalized) == 0 && sourceWasNil {
		normalized = nil
	}
	if len(validation) == 0 {
		validation = nil
	}
	return normalized, validation, legacyDeliveryMenu
}

func cloneCapabilityMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func normalizeCapabilityMap(raw any) map[string]any {
	switch typed := raw.(type) {
	case map[string]any:
		return cloneCapabilityMap(typed)
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out
	default:
		return map[string]any{}
	}
}

func mergeFlatDeliveryAliases(capabilities map[string]any, delivery map[string]any) map[string]any {
	if len(capabilities) == 0 {
		return delivery
	}
	out := cloneCapabilityMap(delivery)
	if out == nil {
		out = map[string]any{}
	}

	if raw, ok := capabilities["delivery_enabled"]; ok {
		out["enabled"] = raw
	}
	if raw, ok := capabilities["deliveryEnabled"]; ok {
		out["enabled"] = raw
	}
	if raw, ok := capabilities["delivery_kind"]; ok {
		out["kind"] = raw
	}
	if raw, ok := capabilities["deliveryKind"]; ok {
		out["kind"] = raw
	}

	routes := normalizeCapabilityMap(out["routes"])
	if raw, ok := capabilities["delivery_list_route"]; ok {
		routes["list"] = raw
	}
	if raw, ok := capabilities["deliveryListRoute"]; ok {
		routes["list"] = raw
	}
	if raw, ok := capabilities["delivery_detail_route"]; ok {
		routes["detail"] = raw
	}
	if raw, ok := capabilities["deliveryDetailRoute"]; ok {
		routes["detail"] = raw
	}
	if len(routes) > 0 {
		out["routes"] = routes
	}

	templates := normalizeCapabilityMap(out["templates"])
	if raw, ok := capabilities["delivery_list_template"]; ok {
		templates["list"] = raw
	}
	if raw, ok := capabilities["deliveryListTemplate"]; ok {
		templates["list"] = raw
	}
	if raw, ok := capabilities["delivery_detail_template"]; ok {
		templates["detail"] = raw
	}
	if raw, ok := capabilities["deliveryDetailTemplate"]; ok {
		templates["detail"] = raw
	}
	if len(templates) > 0 {
		out["templates"] = templates
	}

	menu := normalizeCapabilityMap(out["menu"])
	if raw, ok := capabilities["delivery_menu_location"]; ok {
		menu["location"] = raw
	}
	if raw, ok := capabilities["deliveryMenuLocation"]; ok {
		menu["location"] = raw
	}
	if raw, ok := capabilities["delivery_menu_label_key"]; ok {
		menu["label_key"] = raw
	}
	if raw, ok := capabilities["deliveryMenuLabelKey"]; ok {
		menu["label_key"] = raw
	}
	if len(menu) > 0 {
		out["menu"] = menu
	}

	return out
}

func mergeFlatNavigationAliases(capabilities map[string]any, navigation map[string]any) map[string]any {
	if len(capabilities) == 0 {
		return navigation
	}
	out := cloneCapabilityMap(navigation)
	if out == nil {
		out = map[string]any{}
	}

	if raw, ok := capabilities["navigation_enabled"]; ok {
		out["enabled"] = raw
	}
	if raw, ok := capabilities["navigationEnabled"]; ok {
		out["enabled"] = raw
	}
	if raw, ok := capabilities["navigation_eligible_locations"]; ok {
		out["eligible_locations"] = raw
	}
	if raw, ok := capabilities["navigationEligibleLocations"]; ok {
		out["eligible_locations"] = raw
	}
	if raw, ok := capabilities["navigation_default_locations"]; ok {
		out["default_locations"] = raw
	}
	if raw, ok := capabilities["navigationDefaultLocations"]; ok {
		out["default_locations"] = raw
	}
	if raw, ok := capabilities["navigation_default_visible"]; ok {
		out["default_visible"] = raw
	}
	if raw, ok := capabilities["navigationDefaultVisible"]; ok {
		out["default_visible"] = raw
	}
	if raw, ok := capabilities["allow_instance_override"]; ok {
		out["allow_instance_override"] = raw
	}
	if raw, ok := capabilities["allowInstanceOverride"]; ok {
		out["allow_instance_override"] = raw
	}
	if raw, ok := capabilities["navigation_merge_mode"]; ok {
		out["merge_mode"] = raw
	}
	if raw, ok := capabilities["navigationMergeMode"]; ok {
		out["merge_mode"] = raw
	}
	return out
}

func mergeFlatSearchAliases(capabilities map[string]any, search map[string]any, validation map[string]string) map[string]any {
	if len(capabilities) == 0 {
		return search
	}
	out := cloneCapabilityMap(search)
	if out == nil {
		out = map[string]any{}
	}
	if raw, ok := capabilities["search_enabled"]; ok {
		out["enabled"] = raw
	}
	if raw, ok := capabilities["searchEnabled"]; ok {
		out["enabled"] = raw
	}
	if raw, ok := capabilities["search_collection"]; ok {
		out["collection"] = raw
	}
	if raw, ok := capabilities["searchCollection"]; ok {
		out["collection"] = raw
	}
	if raw, ok := capabilities["search_facets"]; ok {
		out["facets"] = raw
	}
	if raw, ok := capabilities["searchFacets"]; ok {
		out["facets"] = raw
	}
	if raw, ok := capabilities["search_filters"]; ok {
		out["filters"] = raw
	}
	if raw, ok := capabilities["searchFilters"]; ok {
		out["filters"] = raw
	}
	if raw, ok := capabilities["search_published_only"]; ok {
		out["published_only"] = raw
	}
	if raw, ok := capabilities["searchPublishedOnly"]; ok {
		out["published_only"] = raw
	}
	if raw, ok := capabilities["search_fields"]; ok {
		out["fields"] = raw
	}
	if raw, ok := capabilities["searchFields"]; ok {
		out["fields"] = raw
	}

	if raw, exists := capabilities["search"]; exists && len(out) == 0 {
		switch typed := raw.(type) {
		case bool:
			out["enabled"] = typed
		case map[string]any:
			out = cloneCapabilityMap(typed)
		case map[string]string:
			out = normalizeCapabilityMap(typed)
		default:
			validation["capabilities.search"] = "must be an object or boolean"
		}
	}
	return out
}

func migrateLegacyDeliveryMenu(delivery map[string]any, navigation map[string]any) (map[string]any, map[string]any, bool) {
	if len(delivery) == 0 {
		return delivery, navigation, false
	}
	menu := normalizeCapabilityMap(delivery["menu"])
	location := strings.TrimSpace(toString(menu["location"]))
	if location == "" {
		return delivery, navigation, false
	}

	nextDelivery := cloneCapabilityMap(delivery)
	delete(nextDelivery, "menu")

	out := cloneCapabilityMap(navigation)
	if out == nil {
		out = map[string]any{}
	}
	eligible := normalizeStringListAny(out["eligible_locations"])
	defaults := normalizeStringListAny(out["default_locations"])

	if len(eligible) == 0 {
		eligible = []string{location}
	}
	if len(defaults) == 0 {
		defaults = []string{location}
	}
	out["eligible_locations"] = dedupeAndSortStrings(eligible)
	out["default_locations"] = dedupeAndSortStrings(defaults)
	if _, exists := out["enabled"]; !exists {
		out["enabled"] = true
	}
	if _, exists := out["default_visible"]; !exists {
		out["default_visible"] = true
	}
	if _, exists := out["allow_instance_override"]; !exists {
		out["allow_instance_override"] = true
	}
	if strings.TrimSpace(toString(out["merge_mode"])) == "" {
		out["merge_mode"] = defaultNavigationMergeMode
	}
	return nextDelivery, out, true
}

func normalizeDeliveryContract(delivery map[string]any, validation map[string]string) map[string]any {
	if len(delivery) == 0 {
		return delivery
	}
	out := cloneCapabilityMap(delivery)

	if raw, exists := out["enabled"]; exists {
		if value, ok := strictBool(raw); ok {
			out["enabled"] = value
		} else {
			validation["capabilities.delivery.enabled"] = "must be a boolean"
		}
	}

	kind := strings.ToLower(strings.TrimSpace(toString(out["kind"])))
	if kind != "" {
		switch kind {
		case "page", "collection", "detail", "hybrid":
			out["kind"] = kind
		default:
			validation["capabilities.delivery.kind"] = "must be one of page|collection|detail|hybrid"
		}
	} else {
		delete(out, "kind")
	}

	routes := normalizeCapabilityMap(out["routes"])
	if list := strings.TrimSpace(toString(routes["list"])); list != "" {
		routes["list"] = list
	} else {
		delete(routes, "list")
	}
	if detail := strings.TrimSpace(toString(routes["detail"])); detail != "" {
		routes["detail"] = detail
	} else {
		delete(routes, "detail")
	}
	if len(routes) > 0 {
		out["routes"] = routes
	} else {
		delete(out, "routes")
	}

	templates := normalizeCapabilityMap(out["templates"])
	if list := strings.TrimSpace(toString(templates["list"])); list != "" {
		templates["list"] = list
	} else {
		delete(templates, "list")
	}
	if detail := strings.TrimSpace(toString(templates["detail"])); detail != "" {
		templates["detail"] = detail
	} else {
		delete(templates, "detail")
	}
	if len(templates) > 0 {
		out["templates"] = templates
	} else {
		delete(out, "templates")
	}

	return out
}

func normalizeNavigationContract(navigation map[string]any, validation map[string]string) map[string]any {
	if len(navigation) == 0 {
		return navigation
	}
	out := cloneCapabilityMap(navigation)

	if raw, exists := out["allow_instance_override"]; exists {
		if value, ok := strictBool(raw); ok {
			out["allow_instance_override"] = value
		} else {
			validation["capabilities.navigation.allow_instance_override"] = "must be a boolean"
		}
	}
	if raw, exists := out["default_visible"]; exists {
		if value, ok := strictBool(raw); ok {
			out["default_visible"] = value
		} else {
			validation["capabilities.navigation.default_visible"] = "must be a boolean"
		}
	}
	if raw, exists := out["enabled"]; exists {
		if value, ok := strictBool(raw); ok {
			out["enabled"] = value
		} else {
			validation["capabilities.navigation.enabled"] = "must be a boolean"
		}
	}

	eligible := dedupeAndSortStrings(normalizeStringListAny(out["eligible_locations"]))
	defaults := dedupeAndSortStrings(normalizeStringListAny(out["default_locations"]))
	if len(eligible) == 0 && len(defaults) > 0 {
		eligible = append([]string{}, defaults...)
	}
	if len(eligible) == 0 && toBool(out["enabled"]) {
		eligible = []string{defaultNavigationLocation}
	}
	if len(defaults) == 0 && len(eligible) > 0 {
		defaults = []string{eligible[0]}
	}

	if len(eligible) > 0 {
		out["eligible_locations"] = eligible
	} else {
		delete(out, "eligible_locations")
	}
	if len(defaults) > 0 {
		out["default_locations"] = defaults
	} else {
		delete(out, "default_locations")
	}

	eligibleSet := make(map[string]struct{}, len(eligible))
	for _, location := range eligible {
		eligibleSet[location] = struct{}{}
	}
	for _, location := range defaults {
		if _, ok := eligibleSet[location]; !ok {
			validation["capabilities.navigation.default_locations"] = "must be a subset of eligible_locations"
			break
		}
	}

	mergeMode := strings.ToLower(strings.TrimSpace(toString(out["merge_mode"])))
	if mergeMode == "" {
		mergeMode = defaultNavigationMergeMode
	}
	switch mergeMode {
	case "append", "prepend", "replace":
		out["merge_mode"] = mergeMode
	default:
		validation["capabilities.navigation.merge_mode"] = "must be one of append|prepend|replace"
	}

	if _, exists := out["allow_instance_override"]; !exists {
		out["allow_instance_override"] = true
	}
	if _, exists := out["default_visible"]; !exists {
		out["default_visible"] = true
	}
	return out
}

func normalizeSearchContract(search map[string]any, validation map[string]string) map[string]any {
	if len(search) == 0 {
		return search
	}
	out := cloneCapabilityMap(search)

	if raw, exists := out["enabled"]; exists {
		if value, ok := strictBool(raw); ok {
			out["enabled"] = value
		} else {
			validation["capabilities.search.enabled"] = "must be a boolean"
		}
	}
	if raw, exists := out["published_only"]; exists {
		if value, ok := strictBool(raw); ok {
			out["published_only"] = value
		} else {
			validation["capabilities.search.published_only"] = "must be a boolean"
		}
	}
	if collection := strings.TrimSpace(toString(out["collection"])); collection != "" {
		out["collection"] = collection
	} else {
		delete(out, "collection")
	}
	if facets := dedupeAndSortStrings(normalizeStringListAny(out["facets"])); len(facets) > 0 {
		out["facets"] = facets
	} else {
		delete(out, "facets")
	}
	if filters := dedupeAndSortStrings(normalizeStringListAny(out["filters"])); len(filters) > 0 {
		out["filters"] = filters
	} else {
		delete(out, "filters")
	}
	if raw, exists := out["fields"]; exists {
		switch typed := raw.(type) {
		case nil:
			delete(out, "fields")
		case map[string]any:
			out["fields"] = cloneCapabilityMap(typed)
		case map[string]string:
			out["fields"] = normalizeCapabilityMap(typed)
		default:
			validation["capabilities.search.fields"] = "must be an object"
		}
	}

	return out
}

func strictBool(raw any) (bool, bool) {
	switch typed := raw.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "y", "on":
			return true, true
		case "false", "0", "no", "n", "off":
			return false, true
		default:
			return false, false
		}
	case int:
		switch typed {
		case 0:
			return false, true
		case 1:
			return true, true
		default:
			return false, false
		}
	case int64:
		switch typed {
		case 0:
			return false, true
		case 1:
			return true, true
		default:
			return false, false
		}
	case float64:
		switch typed {
		case 0:
			return false, true
		case 1:
			return true, true
		default:
			return false, false
		}
	default:
		return false, false
	}
}

func toBool(raw any) bool {
	if value, ok := strictBool(raw); ok {
		return value
	}
	return false
}

func toString(raw any) string {
	if raw == nil {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case fmt.Stringer:
		return typed.String()
	default:
		text := strings.TrimSpace(fmt.Sprint(raw))
		if text == "<nil>" {
			return ""
		}
		return text
	}
}

func normalizeStringListAny(raw any) []string {
	switch typed := raw.(type) {
	case []string:
		return dedupeAndSortStrings(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			value := strings.TrimSpace(toString(item))
			if value != "" {
				out = append(out, value)
			}
		}
		return dedupeAndSortStrings(out)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		parts := strings.Split(trimmed, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			value := strings.TrimSpace(part)
			if value != "" {
				out = append(out, value)
			}
		}
		return dedupeAndSortStrings(out)
	default:
		return nil
	}
}

func dedupeAndSortStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}
