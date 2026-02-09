package cms_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/content"
	"github.com/goliatone/go-cms/pages"
	"github.com/google/uuid"
)

func TestModule_AdminPageRead_DTOCompletenessWithoutDataInference(t *testing.T) {
	t.Parallel()

	cfg := cms.DefaultConfig()
	cfg.I18N.Locales = []string{"en"}

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	ctx := context.Background()
	actor := uuid.New()

	contentTypeRecord, err := module.ContentTypes().Create(ctx, content.CreateContentTypeRequest{
		Name:      "Page",
		Schema:    map[string]any{"fields": []any{"title", "body", "meta_title", "meta_description"}},
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	contentRecord, err := module.Content().Create(ctx, content.CreateContentRequest{
		ContentTypeID:            contentTypeRecord.ID,
		Slug:                     "home-content",
		Status:                   "draft",
		CreatedBy:                actor,
		UpdatedBy:                actor,
		AllowMissingTranslations: true,
		Translations: []content.ContentTranslationInput{
			{
				Locale: "en",
				Title:  "Content Home",
				Content: map[string]any{
					"body":             "Home body",
					"meta_title":       "Meta title from content",
					"meta_description": "Meta description from content",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageRecord, err := module.Pages().Create(ctx, pages.CreatePageRequest{
		ContentID:                contentRecord.ID,
		TemplateID:               uuid.New(),
		Slug:                     "home",
		Status:                   "draft",
		CreatedBy:                actor,
		UpdatedBy:                actor,
		AllowMissingTranslations: true,
		Translations: []pages.PageTranslationInput{
			{
				Locale: "en",
				Title:  "Home",
				Path:   "/",
			},
		},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	record, err := module.AdminPageRead().Get(ctx, pageRecord.ID.String(), cms.AdminPageGetOptions{
		Locale:         "en",
		IncludeContent: true,
		IncludeData:    false,
	})
	if err != nil {
		t.Fatalf("admin page get: %v", err)
	}
	if record == nil {
		t.Fatalf("expected admin page record")
	}

	if record.Title != "Home" {
		t.Fatalf("expected title Home, got %q", record.Title)
	}
	if record.Slug != "home" {
		t.Fatalf("expected slug home, got %q", record.Slug)
	}
	if record.Path != "/" {
		t.Fatalf("expected path /, got %q", record.Path)
	}
	if record.Status != "draft" {
		t.Fatalf("expected status draft, got %q", record.Status)
	}
	if record.MetaTitle != "Meta title from content" {
		t.Fatalf("expected meta title from top-level field, got %q", record.MetaTitle)
	}
	if record.MetaDescription != "Meta description from content" {
		t.Fatalf("expected meta description from top-level field, got %q", record.MetaDescription)
	}
	if record.Data != nil {
		t.Fatalf("expected Data to be nil when IncludeData=false, got %#v", record.Data)
	}
}
