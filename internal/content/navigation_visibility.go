package content

import (
	"fmt"
	"sort"
	"strings"

	cmscontent "github.com/goliatone/go-cms/content"
)

const (
	NavigationStateInherit = "inherit"
	NavigationStateShow    = "show"
	NavigationStateHide    = "hide"

	NavigationOriginDefault  = "default"
	NavigationOriginOverride = "override"

	NavigationMergeAppend  = "append"
	NavigationMergePrepend = "prepend"
	NavigationMergeReplace = "replace"

	NavigationDuplicateByURL    = "by_url"
	NavigationDuplicateByTarget = "by_target"
	NavigationDuplicateNone     = "none"
)

// NavigationConfig captures normalized content-type navigation capability values.
type NavigationConfig struct {
	Enabled               bool
	EligibleLocations     []string
	DefaultLocations      []string
	DefaultVisible        bool
	AllowInstanceOverride bool
	LabelField            string
	URLField              string
	MergeMode             string
	DuplicatePolicy       string
}

// NavigationVisibilityResult reports effective per-location visibility and origin.
type NavigationVisibilityResult struct {
	Config                 NavigationConfig
	Overrides              map[string]string
	EffectiveVisibility    map[string]bool
	Origins                map[string]string
	EffectiveState         map[string]string
	EffectiveMenuLocations []string
}

// ReadNavigationConfig resolves canonical navigation capability settings.
func ReadNavigationConfig(contentType *ContentType) NavigationConfig {
	cfg := NavigationConfig{
		DefaultVisible:        true,
		AllowInstanceOverride: true,
		LabelField:            "title",
		URLField:              "path",
		MergeMode:             NavigationMergeAppend,
		DuplicatePolicy:       NavigationDuplicateByURL,
	}
	if contentType == nil {
		return cfg
	}

	contracts := cmscontent.ParseContentTypeCapabilityContracts(contentType.Capabilities)
	navigation := contracts.Navigation
	if len(navigation) == 0 {
		return cfg
	}

	cfg.Enabled = boolValue(navigation["enabled"], false)
	cfg.EligibleLocations = normalizeLocationList(navigation["eligible_locations"])
	cfg.DefaultLocations = normalizeLocationList(navigation["default_locations"])
	if len(cfg.DefaultLocations) == 0 && len(cfg.EligibleLocations) > 0 {
		cfg.DefaultLocations = []string{cfg.EligibleLocations[0]}
	}
	cfg.DefaultVisible = boolValue(navigation["default_visible"], true)
	cfg.AllowInstanceOverride = boolValue(navigation["allow_instance_override"], true)

	if field := strings.TrimSpace(toString(navigation["label_field"])); field != "" {
		cfg.LabelField = field
	}
	if field := strings.TrimSpace(toString(navigation["url_field"])); field != "" {
		cfg.URLField = field
	}

	cfg.MergeMode = normalizeNavigationMergeMode(navigation["merge_mode"])
	cfg.DuplicatePolicy = normalizeNavigationDuplicatePolicy(
		firstNonEmptyString(navigation["duplicate_policy"], navigation["dedupe_policy"]),
	)
	return cfg
}

// NormalizeNavigationOverrides normalizes record-level _navigation tri-state overrides.
func NormalizeNavigationOverrides(raw any) (map[string]string, error) {
	if raw == nil {
		return nil, nil
	}

	var source map[string]any
	switch typed := raw.(type) {
	case map[string]any:
		source = typed
	case map[string]string:
		source = make(map[string]any, len(typed))
		for key, value := range typed {
			source[key] = value
		}
	default:
		return nil, fmt.Errorf("%s must be an object", entryFieldNavigation)
	}

	normalized := make(map[string]string, len(source))
	for location, value := range source {
		locationKey := strings.TrimSpace(location)
		if locationKey == "" {
			continue
		}
		state := strings.ToLower(strings.TrimSpace(toString(value)))
		if state == "" {
			continue
		}
		switch state {
		case NavigationStateInherit, NavigationStateShow, NavigationStateHide:
			normalized[locationKey] = state
		default:
			return nil, fmt.Errorf("%s.%s must be inherit|show|hide", entryFieldNavigation, locationKey)
		}
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return normalized, nil
}

// ResolveNavigationVisibility computes effective visibility for every eligible location.
func ResolveNavigationVisibility(contentType *ContentType, metadata map[string]any) NavigationVisibilityResult {
	cfg := ReadNavigationConfig(contentType)
	result := NavigationVisibilityResult{
		Config:              cfg,
		EffectiveVisibility: map[string]bool{},
		Origins:             map[string]string{},
		EffectiveState:      map[string]string{},
	}

	var rawOverrides any
	if metadata != nil {
		rawOverrides = metadata[entryFieldNavigation]
	}
	overrides, err := NormalizeNavigationOverrides(rawOverrides)
	if err == nil && len(overrides) > 0 {
		result.Overrides = overrides
	}

	locations := append([]string{}, cfg.EligibleLocations...)
	for location := range result.Overrides {
		if !containsString(locations, location) {
			locations = append(locations, location)
		}
	}
	sort.Strings(locations[len(cfg.EligibleLocations):])

	eligible := make(map[string]struct{}, len(cfg.EligibleLocations))
	for _, location := range cfg.EligibleLocations {
		eligible[location] = struct{}{}
	}
	defaults := make(map[string]struct{}, len(cfg.DefaultLocations))
	for _, location := range cfg.DefaultLocations {
		defaults[location] = struct{}{}
	}

	for _, location := range locations {
		_, isEligible := eligible[location]
		visible := false
		origin := NavigationOriginDefault
		state := NavigationStateInherit

		if override, ok := result.Overrides[location]; ok {
			state = override
		}
		if !cfg.Enabled {
			isEligible = false
			state = NavigationStateInherit
		}
		if !cfg.AllowInstanceOverride && state != NavigationStateInherit {
			state = NavigationStateInherit
		}

		if isEligible {
			switch state {
			case NavigationStateShow:
				visible = true
				origin = NavigationOriginOverride
			case NavigationStateHide:
				visible = false
				origin = NavigationOriginOverride
			default:
				_, isDefault := defaults[location]
				visible = cfg.DefaultVisible && isDefault
			}
		}

		result.EffectiveVisibility[location] = visible
		result.Origins[location] = origin
		result.EffectiveState[location] = state
		if visible {
			result.EffectiveMenuLocations = append(result.EffectiveMenuLocations, location)
		}
	}

	if len(result.EffectiveVisibility) == 0 {
		result.EffectiveVisibility = nil
		result.Origins = nil
		result.EffectiveState = nil
	}
	if len(result.EffectiveMenuLocations) == 0 {
		result.EffectiveMenuLocations = nil
	}
	return result
}

// ApplyNavigationVisibilityMetadata injects effective visibility metadata on read payloads.
func ApplyNavigationVisibilityMetadata(record *Content) {
	if record == nil || record.Type == nil {
		return
	}
	result := ResolveNavigationVisibility(record.Type, record.Metadata)
	metadata := cloneMap(record.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}

	if len(result.Overrides) > 0 {
		metadata[entryFieldNavigation] = cloneMapString(result.Overrides)
	} else {
		delete(metadata, entryFieldNavigation)
	}
	if len(result.EffectiveMenuLocations) > 0 {
		metadata[entryFieldEffectiveMenuLocations] = append([]string{}, result.EffectiveMenuLocations...)
	} else {
		delete(metadata, entryFieldEffectiveMenuLocations)
	}
	if len(result.EffectiveVisibility) > 0 {
		visibility := make(map[string]bool, len(result.EffectiveVisibility))
		for location, visible := range result.EffectiveVisibility {
			visibility[location] = visible
		}
		metadata[entryFieldEffectiveNavigationVisibility] = visibility
	} else {
		delete(metadata, entryFieldEffectiveNavigationVisibility)
	}

	if len(metadata) == 0 {
		record.Metadata = nil
		return
	}
	record.Metadata = metadata
}

// ApplyNavigationVisibilityToMetadata computes navigation metadata for writes.
func ApplyNavigationVisibilityToMetadata(contentType *ContentType, metadata map[string]any) map[string]any {
	record := &Content{
		Type:     contentType,
		Metadata: cloneMap(metadata),
	}
	ApplyNavigationVisibilityMetadata(record)
	return cloneMap(record.Metadata)
}

func normalizeNavigationMergeMode(raw any) string {
	value := strings.ToLower(strings.TrimSpace(toString(raw)))
	switch value {
	case NavigationMergePrepend:
		return NavigationMergePrepend
	case NavigationMergeReplace:
		return NavigationMergeReplace
	case NavigationMergeAppend:
		return NavigationMergeAppend
	default:
		return NavigationMergeAppend
	}
}

func normalizeNavigationDuplicatePolicy(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case NavigationDuplicateByTarget:
		return NavigationDuplicateByTarget
	case NavigationDuplicateNone:
		return NavigationDuplicateNone
	case NavigationDuplicateByURL:
		return NavigationDuplicateByURL
	default:
		return NavigationDuplicateByURL
	}
}

func normalizeLocationList(raw any) []string {
	switch typed := raw.(type) {
	case []string:
		return dedupeStringList(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, value := range typed {
			location := strings.TrimSpace(toString(value))
			if location != "" {
				out = append(out, location)
			}
		}
		return dedupeStringList(out)
	default:
		location := strings.TrimSpace(toString(raw))
		if location == "" {
			return nil
		}
		return []string{location}
	}
}

func firstNonEmptyString(values ...any) string {
	for _, value := range values {
		text := strings.TrimSpace(toString(value))
		if text != "" {
			return text
		}
	}
	return ""
}

func boolValue(raw any, fallback bool) bool {
	switch typed := raw.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "y", "on":
			return true
		case "false", "0", "no", "n", "off":
			return false
		}
	case int:
		return typed != 0
	case int8:
		return typed != 0
	case int16:
		return typed != 0
	case int32:
		return typed != 0
	case int64:
		return typed != 0
	case uint:
		return typed != 0
	case uint8:
		return typed != 0
	case uint16:
		return typed != 0
	case uint32:
		return typed != 0
	case uint64:
		return typed != 0
	case float32:
		return typed != 0
	case float64:
		return typed != 0
	}
	return fallback
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cloneMapString(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}
