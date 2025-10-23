package menus

import (
	repository "github.com/goliatone/go-repository-bun"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// NewMenuRepository creates a repository for Menu entities.
func NewMenuRepository(db *bun.DB) repository.Repository[*Menu] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Menu]{
		NewRecord: func() *Menu { return &Menu{} },
		GetID: func(m *Menu) uuid.UUID {
			return m.ID
		},
		SetID: func(m *Menu, id uuid.UUID) {
			m.ID = id
		},
		GetIdentifier: func() string {
			return "code"
		},
		GetIdentifierValue: func(m *Menu) string {
			return m.Code
		},
	})
}

// NewMenuItemRepository creates a repository for MenuItem entities.
func NewMenuItemRepository(db *bun.DB) repository.Repository[*MenuItem] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*MenuItem]{
		NewRecord: func() *MenuItem { return &MenuItem{} },
		GetID: func(item *MenuItem) uuid.UUID {
			return item.ID
		},
		SetID: func(item *MenuItem, id uuid.UUID) {
			item.ID = id
		},
		GetIdentifier: func() string {
			return "id"
		},
		GetIdentifierValue: func(item *MenuItem) string {
			return item.ID.String()
		},
	})
}

// NewMenuItemTranslationRepository creates a repository for MenuItemTranslation entities.
func NewMenuItemTranslationRepository(db *bun.DB) repository.Repository[*MenuItemTranslation] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*MenuItemTranslation]{
		NewRecord: func() *MenuItemTranslation { return &MenuItemTranslation{} },
		GetID: func(tr *MenuItemTranslation) uuid.UUID {
			return tr.ID
		},
		SetID: func(tr *MenuItemTranslation, id uuid.UUID) {
			tr.ID = id
		},
		GetIdentifier: func() string {
			return "id"
		},
		GetIdentifierValue: func(item *MenuItemTranslation) string {
			return item.ID.String()
		},
	})
}
