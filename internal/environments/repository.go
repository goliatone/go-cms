package environments

import (
	repository "github.com/goliatone/go-repository-bun"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// NewEnvironmentRepository creates a repository for environment records.
func NewEnvironmentRepository(db *bun.DB) repository.Repository[*Environment] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Environment]{
		NewRecord: func() *Environment { return &Environment{} },
		GetID: func(env *Environment) uuid.UUID {
			return env.ID
		},
		SetID: func(env *Environment, id uuid.UUID) {
			env.ID = id
		},
		GetIdentifier: func() string {
			return "key"
		},
		GetIdentifierValue: func(env *Environment) string {
			return env.Key
		},
	})
}
