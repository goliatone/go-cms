package migrations

import (
	"strings"
	"sync"

	cmsschema "github.com/goliatone/go-cms/internal/schema"
)

// MigrationFunc transforms a content payload between schema versions.
type MigrationFunc func(map[string]any) (map[string]any, error)

// Registry stores content type schema migrations and exposes a runner.
type Registry struct {
	mu       sync.RWMutex
	migrator *cmsschema.Migrator
}

// NewRegistry constructs an empty content schema migration registry.
func NewRegistry() *Registry {
	return &Registry{migrator: cmsschema.NewMigrator()}
}

// Register adds a migration step for a content type schema version.
func (r *Registry) Register(slug, from, to string, fn MigrationFunc) error {
	if r == nil || fn == nil {
		return cmsschema.ErrInvalidSchemaVersion
	}
	fromVersion, err := cmsschema.ParseVersion(from)
	if err != nil {
		return err
	}
	toVersion, err := cmsschema.ParseVersion(to)
	if err != nil {
		return err
	}
	normalizedSlug := strings.TrimSpace(slug)
	if normalizedSlug == "" {
		normalizedSlug = strings.TrimSpace(fromVersion.Slug)
	}
	if normalizedSlug == "" {
		normalizedSlug = strings.TrimSpace(toVersion.Slug)
	}
	if normalizedSlug == "" {
		return cmsschema.ErrInvalidSchemaVersion
	}
	if fromVersion.Slug != "" && fromVersion.Slug != normalizedSlug {
		return cmsschema.ErrInvalidSchemaVersion
	}
	if toVersion.Slug != "" && toVersion.Slug != normalizedSlug {
		return cmsschema.ErrInvalidSchemaVersion
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.migrator == nil {
		r.migrator = cmsschema.NewMigrator()
	}
	return r.migrator.Register(normalizedSlug, fromVersion.String(), toVersion.String(), cmsschema.MigrationFunc(fn))
}

// Migrate applies registered migration steps.
func (r *Registry) Migrate(slug, from, to string, payload map[string]any) (map[string]any, error) {
	if r == nil || r.migrator == nil {
		return nil, cmsschema.ErrInvalidSchemaVersion
	}
	return r.migrator.Migrate(strings.TrimSpace(slug), from, to, payload)
}

// Migrator returns the underlying migrator runner.
func (r *Registry) Migrator() *cmsschema.Migrator {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.migrator == nil {
		r.migrator = cmsschema.NewMigrator()
	}
	return r.migrator
}
