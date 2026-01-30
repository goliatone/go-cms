package blocks

import (
	"github.com/goliatone/go-repository-bun"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func NewDefinitionRepository(db *bun.DB) repository.Repository[*Definition] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Definition]{
		NewRecord:          func() *Definition { return &Definition{} },
		GetID:              func(d *Definition) uuid.UUID { return d.ID },
		SetID:              func(d *Definition, id uuid.UUID) { d.ID = id },
		GetIdentifier:      func() string { return "slug" },
		GetIdentifierValue: func(d *Definition) string { return d.Slug },
	})
}

// NewDefinitionVersionRepository creates a repository for DefinitionVersion entities.
func NewDefinitionVersionRepository(db *bun.DB) repository.Repository[*DefinitionVersion] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*DefinitionVersion]{
		NewRecord:          func() *DefinitionVersion { return &DefinitionVersion{} },
		GetID:              func(v *DefinitionVersion) uuid.UUID { return v.ID },
		SetID:              func(v *DefinitionVersion, id uuid.UUID) { v.ID = id },
		GetIdentifier:      func() string { return "schema_version" },
		GetIdentifierValue: func(v *DefinitionVersion) string { return v.SchemaVersion },
	})
}

// NewInstanceRepository creates a repository for Instance entities.
func NewInstanceRepository(db *bun.DB) repository.Repository[*Instance] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Instance]{
		NewRecord:          func() *Instance { return &Instance{} },
		GetID:              func(inst *Instance) uuid.UUID { return inst.ID },
		SetID:              func(inst *Instance, id uuid.UUID) { inst.ID = id },
		GetIdentifier:      func() string { return "id" },
		GetIdentifierValue: func(inst *Instance) string { return inst.ID.String() },
	})
}

// NewTranslationRepository creates a repository for Translation entities.
func NewTranslationRepository(db *bun.DB) repository.Repository[*Translation] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Translation]{
		NewRecord:          func() *Translation { return &Translation{} },
		GetID:              func(tr *Translation) uuid.UUID { return tr.ID },
		SetID:              func(tr *Translation, id uuid.UUID) { tr.ID = id },
		GetIdentifier:      func() string { return "id" },
		GetIdentifierValue: func(tr *Translation) string { return tr.ID.String() },
	})
}

// NewInstanceVersionRepository creates a repository for InstanceVersion entities.
func NewInstanceVersionRepository(db *bun.DB) repository.Repository[*InstanceVersion] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*InstanceVersion]{
		NewRecord: func() *InstanceVersion { return &InstanceVersion{} },
		GetID: func(iv *InstanceVersion) uuid.UUID {
			return iv.ID
		},
		SetID: func(iv *InstanceVersion, id uuid.UUID) {
			iv.ID = id
		},
		GetIdentifier: func() string {
			return "id"
		},
		GetIdentifierValue: func(iv *InstanceVersion) string {
			if iv == nil {
				return ""
			}
			return iv.ID.String()
		},
	})
}
