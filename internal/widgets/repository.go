package widgets

import (
	repository "github.com/goliatone/go-repository-bun"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// NewDefinitionRepository creates a repository for widget definitions.
func NewDefinitionRepository(db *bun.DB) repository.Repository[*Definition] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Definition]{
		NewRecord:          func() *Definition { return &Definition{} },
		GetID:              func(def *Definition) uuid.UUID { return def.ID },
		SetID:              func(def *Definition, id uuid.UUID) { def.ID = id },
		GetIdentifier:      func() string { return "name" },
		GetIdentifierValue: func(def *Definition) string { return def.Name },
	})
}

// NewInstanceRepository creates a repository for widget instances.
func NewInstanceRepository(db *bun.DB) repository.Repository[*Instance] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Instance]{
		NewRecord:          func() *Instance { return &Instance{} },
		GetID:              func(inst *Instance) uuid.UUID { return inst.ID },
		SetID:              func(inst *Instance, id uuid.UUID) { inst.ID = id },
		GetIdentifier:      func() string { return "id" },
		GetIdentifierValue: func(inst *Instance) string { return inst.ID.String() },
	})
}

// NewTranslationRepository creates a repository for widget translations.
func NewTranslationRepository(db *bun.DB) repository.Repository[*Translation] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Translation]{
		NewRecord:          func() *Translation { return &Translation{} },
		GetID:              func(tr *Translation) uuid.UUID { return tr.ID },
		SetID:              func(tr *Translation, id uuid.UUID) { tr.ID = id },
		GetIdentifier:      func() string { return "id" },
		GetIdentifierValue: func(tr *Translation) string { return tr.ID.String() },
	})
}
