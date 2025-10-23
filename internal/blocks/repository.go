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
		GetIdentifier:      func() string { return "name" },
		GetIdentifierValue: func(d *Definition) string { return d.Name },
	})
}

// NewInstanceRepository creates a repository for Instance entities.
func NewInstanceRepository(db *bun.DB) repository.Repository[*Instance] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Instance]{
		NewRecord:          func() *Instance { return &Instance{} },
		GetID:              func(inst *Instance) uuid.UUID { return inst.ID },
		SetID:              func(inst *Instance, id uuid.UUID) { inst.ID = id },
		GetIdentifier:      func() string { return "" },
		GetIdentifierValue: func(*Instance) string { return "" },
	})
}

// NewTranslationRepository creates a repository for Translation entities.
func NewTranslationRepository(db *bun.DB) repository.Repository[*Translation] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Translation]{
		NewRecord:          func() *Translation { return &Translation{} },
		GetID:              func(tr *Translation) uuid.UUID { return tr.ID },
		SetID:              func(tr *Translation, id uuid.UUID) { tr.ID = id },
		GetIdentifier:      func() string { return "" },
		GetIdentifierValue: func(*Translation) string { return "" },
	})
}
