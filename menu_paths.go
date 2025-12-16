package cms

import (
	"errors"
	"strings"
)

var (
	ErrMenuCodeRequired     = errors.New("cms: menu code is required")
	ErrMenuItemPathRequired = errors.New("cms: menu item path is required")
	ErrMenuItemPathInvalid  = errors.New("cms: menu item path is invalid")
	ErrMenuItemPathMismatch = errors.New("cms: menu item path does not match menu code")
)

// MenuItemPath captures parsed information about a dot-path menu item identifier.
//
// Canonicalization contract
//   - Menu codes and path segments are treated as lowercase.
//   - Callers may supply dot-paths, slash-paths, or relative paths; go-cms will canonicalize inputs
//     into a stable dot-path (`<menuCode>.<seg>...`) and validate against `isPathSegment`.
//   - Canonicalization rules are documented in `MENU_CANONICALIZATION.md`.
//
// Example:
// - Path:      "admin.content.pages"
// - MenuCode:  "admin"
// - ParentPath:"admin.content"
// - Key:       "pages"
type MenuItemPath struct {
	Path       string
	MenuCode   string
	ParentPath string
	Key        string
}

// ParseMenuItemPath parses a dot-separated menu item path and derives menu code and parent path.
//
// Invariants:
// - Path must include the menu code prefix and at least one item segment (min 2 segments).
// - No leading/trailing dots and no empty segments.
func ParseMenuItemPath(path string) (MenuItemPath, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return MenuItemPath{}, ErrMenuItemPathRequired
	}

	canonical := sanitizeDotPath(trimmed)
	if canonical == "" {
		return MenuItemPath{}, ErrMenuItemPathInvalid
	}

	parts := strings.Split(canonical, ".")
	if len(parts) < 2 {
		return MenuItemPath{}, ErrMenuItemPathInvalid
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return MenuItemPath{}, ErrMenuItemPathInvalid
		}
		if !isPathSegment(part) {
			return MenuItemPath{}, ErrMenuItemPathInvalid
		}
	}

	menuCode := strings.TrimSpace(parts[0])
	key := strings.TrimSpace(parts[len(parts)-1])

	parent := ""
	if len(parts) == 2 {
		parent = menuCode
	} else {
		parent = strings.Join(parts[:len(parts)-1], ".")
	}

	return MenuItemPath{
		Path:       canonical,
		MenuCode:   menuCode,
		ParentPath: parent,
		Key:        key,
	}, nil
}

// ParseMenuItemPathForMenu validates that the provided path belongs to the given menu code.
func ParseMenuItemPathForMenu(menuCode string, path string) (MenuItemPath, error) {
	code := CanonicalMenuCode(menuCode)
	if code == "" {
		return MenuItemPath{}, ErrMenuCodeRequired
	}
	parsed, err := ParseMenuItemPath(path)
	if err != nil {
		return MenuItemPath{}, err
	}
	if parsed.MenuCode != code {
		return MenuItemPath{}, ErrMenuItemPathMismatch
	}
	return parsed, nil
}

func isPathSegment(seg string) bool {
	if seg == "" {
		return false
	}
	for _, r := range seg {
		switch {
		case r >= 'a' && r <= 'z':
			continue
		case r >= '0' && r <= '9':
			continue
		case r == '_' || r == '-':
			continue
		default:
			return false
		}
	}
	return true
}
