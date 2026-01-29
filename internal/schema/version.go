package schema

import (
	"fmt"
	"strconv"
	"strings"
)

// Version identifies a schema revision for a content type or block.
type Version struct {
	Slug   string
	SemVer string
}

// ParseVersion parses a "<slug>@vMAJOR.MINOR.PATCH" string.
func ParseVersion(value string) (Version, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return Version{}, fmt.Errorf("%w: empty", ErrInvalidSchemaVersion)
	}
	parts := strings.Split(trimmed, "@")
	if len(parts) != 2 {
		return Version{}, fmt.Errorf("%w: %s", ErrInvalidSchemaVersion, value)
	}
	slug := strings.TrimSpace(parts[0])
	version := strings.TrimSpace(parts[1])
	if slug == "" || version == "" {
		return Version{}, fmt.Errorf("%w: %s", ErrInvalidSchemaVersion, value)
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	if !isSemVer(version) {
		return Version{}, fmt.Errorf("%w: %s", ErrInvalidSchemaVersion, value)
	}
	return Version{Slug: slug, SemVer: version}, nil
}

// DefaultVersion builds the initial schema version for a slug.
func DefaultVersion(slug string) Version {
	return Version{Slug: strings.TrimSpace(slug), SemVer: "v1.0.0"}
}

// String returns the canonical string format.
func (v Version) String() string {
	if strings.TrimSpace(v.Slug) == "" {
		return strings.TrimSpace(v.SemVer)
	}
	return strings.TrimSpace(v.Slug) + "@" + strings.TrimSpace(v.SemVer)
}

func isSemVer(value string) bool {
	if !strings.HasPrefix(value, "v") {
		return false
	}
	parts := strings.Split(value[1:], ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}

// BumpVersion increments a semantic version by the provided change level.
func BumpVersion(base Version, level ChangeLevel) (Version, error) {
	if strings.TrimSpace(base.SemVer) == "" {
		return Version{}, ErrInvalidSchemaVersion
	}
	major, minor, patch, err := parseSemVer(base.SemVer)
	if err != nil {
		return Version{}, err
	}
	switch level {
	case ChangeMajor:
		major++
		minor = 0
		patch = 0
	case ChangeMinor:
		minor++
		patch = 0
	case ChangePatch:
		patch++
	case ChangeNone:
		return base, nil
	default:
		return base, nil
	}
	return Version{Slug: base.Slug, SemVer: fmt.Sprintf("v%d.%d.%d", major, minor, patch)}, nil
}

// EnsureSchemaVersion ensures the schema metadata contains a valid schema_version.
func EnsureSchemaVersion(schema map[string]any, slug string) (map[string]any, Version, error) {
	if schema == nil {
		return nil, Version{}, ErrInvalidSchemaVersion
	}
	meta := ExtractMetadata(schema)
	normalizedSlug := strings.TrimSpace(slug)
	if meta.Slug == "" && normalizedSlug != "" {
		meta.Slug = normalizedSlug
	}
	if meta.SchemaVersion != "" {
		version, err := ParseVersion(meta.SchemaVersion)
		if err != nil {
			return nil, Version{}, err
		}
		if normalizedSlug != "" && version.Slug != normalizedSlug {
			return nil, Version{}, fmt.Errorf("%w: slug mismatch", ErrInvalidSchemaVersion)
		}
		if normalizedSlug == "" {
			normalizedSlug = version.Slug
		}
		meta.SchemaVersion = version.String()
		return ApplyMetadata(schema, meta), version, nil
	}
	if normalizedSlug == "" {
		return nil, Version{}, fmt.Errorf("%w: slug required", ErrInvalidSchemaVersion)
	}
	version := DefaultVersion(normalizedSlug)
	meta.SchemaVersion = version.String()
	return ApplyMetadata(schema, meta), version, nil
}

func parseSemVer(value string) (int, int, int, error) {
	if !strings.HasPrefix(value, "v") {
		return 0, 0, 0, ErrInvalidSchemaVersion
	}
	parts := strings.Split(value[1:], ".")
	if len(parts) != 3 {
		return 0, 0, 0, ErrInvalidSchemaVersion
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, ErrInvalidSchemaVersion
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, ErrInvalidSchemaVersion
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, ErrInvalidSchemaVersion
	}
	return major, minor, patch, nil
}
