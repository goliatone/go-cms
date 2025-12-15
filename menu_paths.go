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
	parts := strings.Split(trimmed, ".")
	if len(parts) < 2 {
		return MenuItemPath{}, ErrMenuItemPathInvalid
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return MenuItemPath{}, ErrMenuItemPathInvalid
		}
		if !isPathSegment(strings.TrimSpace(part)) {
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
		Path:       trimmed,
		MenuCode:   menuCode,
		ParentPath: parent,
		Key:        key,
	}, nil
}

// ParseMenuItemPathForMenu validates that the provided path belongs to the given menu code.
func ParseMenuItemPathForMenu(menuCode string, path string) (MenuItemPath, error) {
	if strings.TrimSpace(menuCode) == "" {
		return MenuItemPath{}, ErrMenuCodeRequired
	}
	parsed, err := ParseMenuItemPath(path)
	if err != nil {
		return MenuItemPath{}, err
	}
	if parsed.MenuCode != strings.TrimSpace(menuCode) {
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
		case r >= 'A' && r <= 'Z':
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
