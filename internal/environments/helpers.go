package environments

import (
	"regexp"
	"strings"

	"github.com/goliatone/go-cms/internal/identity"
	"github.com/google/uuid"
)

var environmentKeyPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

const defaultEnvironmentKey = "default"
const defaultEnvironmentID = "00000000-0000-0000-0000-000000000001"

// DefaultKey is the canonical fallback environment key.
const DefaultKey = defaultEnvironmentKey

// DefaultID is the canonical UUID for the default environment key.
const DefaultID = defaultEnvironmentID

// NormalizeKey trims and lowercases environment keys.
func NormalizeKey(key string) string {
	return normalizeEnvironmentKey(key)
}

func normalizeEnvironmentKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func deriveEnvironmentName(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) == 1 {
		return strings.ToUpper(key)
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

// IDForKey derives deterministic UUIDs for environment keys.
func IDForKey(key string) uuid.UUID {
	if normalizeEnvironmentKey(key) == defaultEnvironmentKey {
		return uuid.MustParse(defaultEnvironmentID)
	}
	return identity.UUID("go-cms:environment:" + normalizeEnvironmentKey(key))
}

func cloneEnvironment(env *Environment) *Environment {
	if env == nil {
		return nil
	}
	cloned := *env
	cloned.Description = cloneString(env.Description)
	return &cloned
}

func cloneEnvironmentSlice(src []*Environment) []*Environment {
	if len(src) == 0 {
		return nil
	}
	out := make([]*Environment, len(src))
	for i, env := range src {
		out[i] = cloneEnvironment(env)
	}
	return out
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}
