package cms

import (
	"fmt"
	"strings"
)

// CanonicalMenuCode normalizes user input into a go-cms menu code.
//
// Canonicalization rules are documented in MENU_CANONICALIZATION.md.
func CanonicalMenuCode(code string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(trimmed))

	lastDash := false
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '_' || r == '-':
			b.WriteRune(r)
			lastDash = false
		case r == '.':
			b.WriteRune('_')
			lastDash = false
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(b.String(), "-_")
}

// SanitizeMenuItemSegment converts an arbitrary string into a safe segment used in dot-paths.
//
// It returns an empty string when the input cannot be sanitized into a non-empty segment.
// Canonicalization rules are documented in MENU_CANONICALIZATION.md.
func SanitizeMenuItemSegment(seg string) string {
	raw := strings.TrimSpace(seg)
	if raw == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(raw))

	lastDash := false
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '_' || r == '-':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(b.String(), "-_")
}

// CanonicalMenuItemPath canonicalizes raw inputs (dot-path, slash-path, relative path) into a
// canonical dot-path for the provided menu code.
//
// Output is guaranteed to belong to menuCode and pass ParseMenuItemPathForMenu invariants, or
// an error is returned.
func CanonicalMenuItemPath(menuCode, raw string) (string, error) {
	code := CanonicalMenuCode(menuCode)
	if code == "" {
		return "", ErrMenuCodeRequired
	}

	pathRaw := strings.TrimSpace(raw)
	if pathRaw == "" {
		return "", ErrMenuItemPathRequired
	}

	path := sanitizeDotPath(pathRaw)
	if path == "" {
		return "", ErrMenuItemPathInvalid
	}

	// If the sanitized path does not already belong to code, treat it as relative.
	switch {
	case path == code, strings.HasPrefix(path, code+"."):
		// already prefixed
	default:
		path = code + "." + path
	}

	if _, err := ParseMenuItemPathForMenu(code, path); err != nil {
		return "", err
	}
	return path, nil
}

type DerivedMenuItemPath struct {
	Path       string
	ParentPath string
}

// DeriveMenuItemPaths canonicalizes and derives Path/ParentPath consistently from common user inputs.
//
// Canonicalization and derivation rules are documented in MENU_CANONICALIZATION.md.
func DeriveMenuItemPaths(menuCode string, id string, parent string, fallbackLabel string) (DerivedMenuItemPath, error) {
	code := CanonicalMenuCode(menuCode)
	if code == "" {
		return DerivedMenuItemPath{}, ErrMenuCodeRequired
	}

	parentPath, err := canonicalParentPath(code, parent)
	if err != nil {
		return DerivedMenuItemPath{}, err
	}

	idTrimmed := strings.TrimSpace(id)
	if idTrimmed == "" {
		seg := SanitizeMenuItemSegment(fallbackLabel)
		if seg == "" {
			seg = "item"
		}
		var path string
		if parentPath != "" {
			path = parentPath + "." + seg
		} else {
			path = code + "." + seg
		}
		if _, err := ParseMenuItemPathForMenu(code, path); err != nil {
			return DerivedMenuItemPath{}, err
		}
		return DerivedMenuItemPath{Path: path, ParentPath: parentPath}, nil
	}

	candidate, err := CanonicalMenuItemPath(code, idTrimmed)
	if err != nil {
		return DerivedMenuItemPath{}, err
	}

	// If the caller provided an explicit parent and the ID is a single segment, treat it as relative.
	if parentPath != "" && !strings.HasPrefix(candidate, parentPath+".") {
		if isSingleSegmentID(idTrimmed) {
			seg := SanitizeMenuItemSegment(idTrimmed)
			if seg == "" {
				return DerivedMenuItemPath{}, ErrMenuItemPathInvalid
			}
			candidate = parentPath + "." + seg
		}
	}

	if _, err := ParseMenuItemPathForMenu(code, candidate); err != nil {
		return DerivedMenuItemPath{}, err
	}

	return DerivedMenuItemPath{Path: candidate, ParentPath: parentPath}, nil
}

// SeedPositionPtrForType normalizes seed "optional position" semantics (nil vs 0).
//
// Canonicalization rules are documented in MENU_CANONICALIZATION.md.
func SeedPositionPtrForType(itemType string, pos int) *int {
	if pos < 0 {
		return nil
	}
	if pos > 0 {
		v := pos
		return &v
	}

	switch strings.TrimSpace(itemType) {
	case "group", "separator":
		v := 0
		return &v
	default:
		return nil
	}
}

// ShouldAutoCreateParentsSeed returns true when the provided seed spec suggests that intermediate
// parents are missing and should be auto-scaffolded.
func ShouldAutoCreateParentsSeed(items []SeedMenuItem) bool {
	if len(items) == 0 {
		return false
	}

	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		path := strings.TrimSpace(item.Path)
		if path == "" {
			continue
		}
		seen[path] = struct{}{}
	}

	for _, item := range items {
		parts := splitPathParts(item.Path)
		// menuCode + at least 2 segments suggests missing intermediates.
		if len(parts) < 3 {
			continue
		}

		for i := 2; i < len(parts); i++ {
			parentPath := strings.Join(parts[:i], ".")
			if _, ok := seen[parentPath]; !ok {
				return true
			}
		}
	}

	return false
}

// NormalizeTranslationFields implements common "fallback to key" behavior for label fields.
func NormalizeTranslationFields(label, labelKey, groupTitle, groupTitleKey string) (string, string, string, string) {
	label = strings.TrimSpace(label)
	labelKey = strings.TrimSpace(labelKey)
	groupTitle = strings.TrimSpace(groupTitle)
	groupTitleKey = strings.TrimSpace(groupTitleKey)

	if label == "" && labelKey != "" {
		label = labelKey
	}
	if groupTitle == "" && groupTitleKey != "" {
		groupTitle = groupTitleKey
	}
	return label, labelKey, groupTitle, groupTitleKey
}

func sanitizeDotPath(raw string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "/", ".")
	normalized = strings.Trim(normalized, ".")
	if normalized == "" {
		return ""
	}

	parts := strings.Split(normalized, ".")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		seg := SanitizeMenuItemSegment(p)
		if seg == "" {
			continue
		}
		out = append(out, seg)
	}
	return strings.Join(out, ".")
}

func canonicalParentPath(menuCode string, parent string) (string, error) {
	trimmed := strings.TrimSpace(parent)
	if trimmed == "" {
		return "", nil
	}

	path := sanitizeDotPath(trimmed)
	if path == "" {
		return "", fmt.Errorf("%w: invalid parent path", ErrMenuItemPathInvalid)
	}

	// Allow using the menu code as the root sentinel (matches existing public menu APIs).
	if path == menuCode {
		return menuCode, nil
	}

	switch {
	case strings.HasPrefix(path, menuCode+"."):
		// already prefixed
	default:
		path = menuCode + "." + path
	}

	if _, err := ParseMenuItemPathForMenu(menuCode, path); err != nil {
		return "", err
	}
	return path, nil
}

func isSingleSegmentID(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	return trimmed != "" && !strings.Contains(trimmed, ".") && !strings.Contains(trimmed, "/")
}

func splitPathParts(raw string) []string {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "/", ".")
	normalized = strings.Trim(normalized, ".")
	if normalized == "" {
		return nil
	}
	parts := strings.Split(normalized, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
