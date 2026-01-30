package blocks

import (
	"sort"
	"strconv"
	"strings"
	"sync"

	cmsschema "github.com/goliatone/go-cms/internal/schema"
	"github.com/goliatone/go-slug"
)

// Registry stores versioned block schema definitions and migrations.
type Registry struct {
	mu       sync.RWMutex
	entries  map[string]RegisterDefinitionInput
	versions map[string]map[string]RegisterDefinitionInput
	migrator *Migrator
}

// NewRegistry constructs an empty block schema registry.
func NewRegistry() *Registry {
	return &Registry{
		entries:  make(map[string]RegisterDefinitionInput),
		versions: make(map[string]map[string]RegisterDefinitionInput),
		migrator: NewMigrator(),
	}
}

// Register records a versioned schema definition. The latest version is exposed via List().
func (r *Registry) Register(input RegisterDefinitionInput) {
	if r == nil {
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return
	}
	input.Name = name
	slugValue := registrySlug(input)
	if slugValue != "" {
		input.Slug = slugValue
	}

	normalized := input.Schema
	version := ""
	if input.Schema != nil {
		key := slugValue
		if key == "" {
			key = name
		}
		if updated, parsed, err := cmsschema.EnsureSchemaVersion(input.Schema, key); err == nil {
			normalized = updated
			version = parsed.String()
		}
	}
	input.Schema = normalized

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.versions == nil {
		r.versions = make(map[string]map[string]RegisterDefinitionInput)
	}
	if r.entries == nil {
		r.entries = make(map[string]RegisterDefinitionInput)
	}

	if r.versions[name] == nil {
		r.versions[name] = make(map[string]RegisterDefinitionInput)
	}
	key := version
	r.versions[name][key] = input

	latest, ok := r.entries[name]
	if !ok || version == "" {
		r.entries[name] = input
		return
	}

	latestKey := registrySlug(latest)
	if latestKey == "" {
		latestKey = name
	}
	latestVersion := extractVersion(latest.Schema, latestKey)
	if latestVersion == "" || compareSchemaVersions(version, latestVersion) > 0 {
		r.entries[name] = input
	}
}

// RegisterMigration adds a migration step for a block definition.
func (r *Registry) RegisterMigration(name, from, to string, fn MigrationFunc) error {
	if r == nil {
		return cmsschema.ErrInvalidSchemaVersion
	}
	if r.migrator == nil {
		r.migrator = NewMigrator()
	}
	return r.migrator.Register(strings.TrimSpace(name), from, to, fn)
}

// Migrate applies registered migration steps.
func (r *Registry) Migrate(name, from, to string, payload map[string]any) (map[string]any, error) {
	if r == nil || r.migrator == nil {
		return nil, cmsschema.ErrInvalidSchemaVersion
	}
	return r.migrator.Migrate(strings.TrimSpace(name), from, to, payload)
}

// Migrator exposes the underlying migration runner.
func (r *Registry) Migrator() *Migrator {
	if r == nil {
		return nil
	}
	return r.migrator
}

// List returns the latest version of each registered definition.
func (r *Registry) List() []RegisterDefinitionInput {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]RegisterDefinitionInput, 0, len(r.entries))
	for _, entry := range r.entries {
		out = append(out, entry)
	}
	return out
}

// Latest returns the latest registered schema definition for a name.
func (r *Registry) Latest(name string) (RegisterDefinitionInput, bool) {
	if r == nil {
		return RegisterDefinitionInput{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[strings.TrimSpace(name)]
	return entry, ok
}

// ListVersions returns all versions registered for a definition name.
func (r *Registry) ListVersions(name string) []RegisterDefinitionInput {
	if r == nil {
		return nil
	}
	key := strings.TrimSpace(name)
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs, ok := r.versions[key]
	if !ok {
		return nil
	}
	out := make([]RegisterDefinitionInput, 0, len(defs))
	for _, entry := range defs {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		aiKey := registrySlug(out[i])
		if aiKey == "" {
			aiKey = key
		}
		ajKey := registrySlug(out[j])
		if ajKey == "" {
			ajKey = key
		}
		ai := extractVersion(out[i].Schema, aiKey)
		aj := extractVersion(out[j].Schema, ajKey)
		return compareSchemaVersions(ai, aj) < 0
	})
	return out
}

// ListAllVersions returns every registered definition version.
func (r *Registry) ListAllVersions() []RegisterDefinitionInput {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]RegisterDefinitionInput, 0)
	for name := range r.versions {
		for _, entry := range r.versions[name] {
			out = append(out, entry)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ni := strings.TrimSpace(out[i].Name)
		nj := strings.TrimSpace(out[j].Name)
		if ni == nj {
			viKey := registrySlug(out[i])
			if viKey == "" {
				viKey = ni
			}
			vjKey := registrySlug(out[j])
			if vjKey == "" {
				vjKey = nj
			}
			vi := extractVersion(out[i].Schema, viKey)
			vj := extractVersion(out[j].Schema, vjKey)
			return compareSchemaVersions(vi, vj) < 0
		}
		return ni < nj
	})
	return out
}

func registrySlug(input RegisterDefinitionInput) string {
	candidate := strings.TrimSpace(input.Slug)
	if candidate == "" {
		candidate = strings.TrimSpace(input.Name)
	}
	if candidate == "" {
		return ""
	}
	normalizer := slug.Default()
	normalized, err := normalizer.Normalize(candidate)
	if err != nil || normalized == "" {
		return candidate
	}
	return normalized
}

func extractVersion(schema map[string]any, slug string) string {
	if schema == nil {
		return ""
	}
	_, version, err := cmsschema.EnsureSchemaVersion(schema, slug)
	if err != nil {
		return ""
	}
	return version.String()
}

func compareSchemaVersions(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	av, errA := cmsschema.ParseVersion(a)
	bv, errB := cmsschema.ParseVersion(b)
	if errA != nil || errB != nil {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
	return compareSemVer(av.SemVer, bv.SemVer)
}

func compareSemVer(a, b string) int {
	am, an, ap, okA := semverParts(a)
	bm, bn, bp, okB := semverParts(b)
	if !okA || !okB {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
	if am != bm {
		if am < bm {
			return -1
		}
		return 1
	}
	if an != bn {
		if an < bn {
			return -1
		}
		return 1
	}
	if ap != bp {
		if ap < bp {
			return -1
		}
		return 1
	}
	return 0
}

func semverParts(value string) (int, int, int, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "v")
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, false
	}
	return major, minor, patch, true
}
