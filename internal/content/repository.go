package content

import (
	"github.com/goliatone/go-repository-bun"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func NewLocaleRepository(db *bun.DB) repository.Repository[*Locale] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Locale]{
		NewRecord: func() *Locale { return &Locale{} },
		GetID: func(l *Locale) uuid.UUID {
			return l.ID
		},
		SetID: func(l *Locale, id uuid.UUID) {
			l.ID = id
		},
		GetIdentifier: func() string {
			return "code"
		},
		GetIdentifierValue: func(l *Locale) string {
			return l.Code
		},
	})
}

func NewContentTypeRepository(db *bun.DB) repository.Repository[*ContentType] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*ContentType]{
		NewRecord: func() *ContentType { return &ContentType{} },
		GetID: func(ct *ContentType) uuid.UUID {
			return ct.ID
		},
		SetID: func(ct *ContentType, id uuid.UUID) {
			ct.ID = id
		},
		GetIdentifier: func() string {
			return "name"
		},
		GetIdentifierValue: func(ct *ContentType) string {
			return ct.Name
		},
	})
}

func NewContentRepository(db *bun.DB) repository.Repository[*Content] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Content]{
		NewRecord: func() *Content { return &Content{} },
		GetID: func(c *Content) uuid.UUID {
			return c.ID
		},
		SetID: func(c *Content, id uuid.UUID) {
			c.ID = id
		},
		GetIdentifier: func() string {
			return "slug"
		},
		GetIdentifierValue: func(c *Content) string {
			return c.Slug
		},
	})
}

func NewContentTranslationRepository(db *bun.DB) repository.Repository[*ContentTranslation] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*ContentTranslation]{
		NewRecord: func() *ContentTranslation { return &ContentTranslation{} },
		GetID: func(ct *ContentTranslation) uuid.UUID {
			return ct.ID
		},
		SetID: func(ct *ContentTranslation, id uuid.UUID) {
			ct.ID = id
		},
		GetIdentifier: func() string {
			return "id"
		},
		GetIdentifierValue: func(ct *ContentTranslation) string {
			if ct == nil {
				return ""
			}
			return ct.ID.String()
		},
	})
}

// NewContentVersionRepository creates a repository for ContentVersion entities.
func NewContentVersionRepository(db *bun.DB) repository.Repository[*ContentVersion] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*ContentVersion]{
		NewRecord: func() *ContentVersion { return &ContentVersion{} },
		GetID: func(cv *ContentVersion) uuid.UUID {
			return cv.ID
		},
		SetID: func(cv *ContentVersion, id uuid.UUID) {
			cv.ID = id
		},
		GetIdentifier: func() string {
			return "id"
		},
		GetIdentifierValue: func(cv *ContentVersion) string {
			if cv == nil {
				return ""
			}
			return cv.ID.String()
		},
	})
}
