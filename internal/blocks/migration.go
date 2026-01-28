package blocks

import cmsschema "github.com/goliatone/go-cms/internal/schema"

// MigrationFunc transforms a block payload between schema versions.
type MigrationFunc func(map[string]any) (map[string]any, error)

// MigrationStep describes a single block schema migration hop.
type MigrationStep struct {
	From  string
	To    string
	Apply MigrationFunc
}

// Migrator applies ordered migration steps for block schemas.
type Migrator struct {
	inner *cmsschema.Migrator
}

// NewMigrator constructs an empty block schema migrator.
func NewMigrator() *Migrator {
	return &Migrator{inner: cmsschema.NewMigrator()}
}

// Register adds a migration step for a block definition.
func (m *Migrator) Register(slug, from, to string, fn MigrationFunc) error {
	if m == nil {
		return cmsschema.ErrInvalidSchemaVersion
	}
	if m.inner == nil {
		m.inner = cmsschema.NewMigrator()
	}
	return m.inner.Register(slug, from, to, cmsschema.MigrationFunc(fn))
}

// Migrate applies migration steps until the target version is reached.
func (m *Migrator) Migrate(slug, from, to string, payload map[string]any) (map[string]any, error) {
	if m == nil || m.inner == nil {
		return nil, cmsschema.ErrInvalidSchemaVersion
	}
	return m.inner.Migrate(slug, from, to, payload)
}
