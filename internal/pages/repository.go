package pages

import (
	"github.com/goliatone/go-repository-bun"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func NewPageRepository(db *bun.DB) repository.Repository[*Page] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*Page]{
		NewRecord: func() *Page { return &Page{} },
		GetID: func(p *Page) uuid.UUID {
			return p.ID
		},
		SetID: func(p *Page, id uuid.UUID) {
			p.ID = id
		},
		GetIdentifier: func() string {
			return "slug"
		},
		GetIdentifierValue: func(p *Page) string {
			return p.Slug
		},
	})
}

func NewPageTranslationRepository(db *bun.DB) repository.Repository[*PageTranslation] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*PageTranslation]{
		NewRecord: func() *PageTranslation { return &PageTranslation{} },
		GetID: func(pt *PageTranslation) uuid.UUID {
			return pt.ID
		},
		SetID: func(pt *PageTranslation, id uuid.UUID) {
			pt.ID = id
		},
		GetIdentifier: func() string {
			return "path"
		},
		GetIdentifierValue: func(pt *PageTranslation) string {
			return pt.Path
		},
	})
}

func NewPageVersionRepository(db *bun.DB) repository.Repository[*PageVersion] {
	return repository.MustNewRepository(db, repository.ModelHandlers[*PageVersion]{
		NewRecord: func() *PageVersion { return &PageVersion{} },
		GetID: func(pv *PageVersion) uuid.UUID {
			return pv.ID
		},
		SetID: func(pv *PageVersion, id uuid.UUID) {
			pv.ID = id
		},
		GetIdentifier: func() string {
			return ""
		},
		GetIdentifierValue: func(*PageVersion) string {
			return ""
		},
	})
}
