package themes

import (
	repository "github.com/goliatone/go-repository-bun"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// NewThemeRepository creates a repository for themes.
func NewThemeRepository(db *bun.DB) repository.Repository[*Theme] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Theme]{
		NewRecord:          func() *Theme { return &Theme{} },
		GetID:              func(theme *Theme) uuid.UUID { return theme.ID },
		SetID:              func(theme *Theme, id uuid.UUID) { theme.ID = id },
		GetIdentifier:      func() string { return "name" },
		GetIdentifierValue: func(theme *Theme) string { return theme.Name },
	})
}

// NewTemplateRepository creates a repository for templates.
func NewTemplateRepository(db *bun.DB) repository.Repository[*Template] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Template]{
		NewRecord:          func() *Template { return &Template{} },
		GetID:              func(tpl *Template) uuid.UUID { return tpl.ID },
		SetID:              func(tpl *Template, id uuid.UUID) { tpl.ID = id },
		GetIdentifier:      func() string { return "slug" },
		GetIdentifierValue: func(tpl *Template) string { return tpl.Slug },
	})
}
