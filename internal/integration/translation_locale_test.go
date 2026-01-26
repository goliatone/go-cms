package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/google/uuid"
)

func TestTranslationDefaultLocaleRequired(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en", "es"}
	cfg.I18N.RequireTranslations = true
	cfg.I18N.DefaultLocaleRequired = true

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("cms.New() error = %v", err)
	}

	typeRepo := module.Container().ContentTypeRepository()
	seedTypes, ok := typeRepo.(interface {
		Put(*content.ContentType) error
	})
	if !ok {
		t.Fatalf("expected seedable content type repository, got %T", typeRepo)
	}
	contentTypeID := uuid.New()
	if err := seedTypes.Put(&content.ContentType{ID: contentTypeID, Name: "article", Slug: "article"}); err != nil {
		t.Fatalf("seed content type: %v", err)
	}

	authorID := uuid.New()
	contentSvc := module.Content()

	if _, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "missing-translations",
		Status:        "draft",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
	}); !errors.Is(err, content.ErrNoTranslations) {
		t.Fatalf("expected ErrNoTranslations, got %v", err)
	}

	if _, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "missing-default-locale",
		Status:        "draft",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{Locale: "es", Title: "Hola"},
		},
	}); !errors.Is(err, content.ErrDefaultLocaleRequired) {
		t.Fatalf("expected ErrDefaultLocaleRequired, got %v", err)
	}

	contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "default-locale-ok",
		Status:        "draft",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("create content with default locale: %v", err)
	}

	pageSvc := module.Pages()
	if _, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  contentRecord.ID,
		TemplateID: uuid.New(),
		Slug:       "missing-default-page",
		Status:     "draft",
		CreatedBy:  authorID,
		UpdatedBy:  authorID,
		Translations: []pages.PageTranslationInput{
			{Locale: "es", Title: "Articulo", Path: "/articulo"},
		},
	}); !errors.Is(err, pages.ErrDefaultLocaleRequired) {
		t.Fatalf("expected ErrDefaultLocaleRequired, got %v", err)
	}
}
