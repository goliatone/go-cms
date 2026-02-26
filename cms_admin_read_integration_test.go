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

func TestModule_AdminPageRead_LocaleMetadataStableAcrossFallback(t *testing.T) {
	t.Parallel()

	cfg := cms.DefaultConfig()
	cfg.I18N.Locales = []string{"en", "es"}

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	ctx := context.Background()
	actor := uuid.New()

	contentTypeRecord, err := module.ContentTypes().Create(ctx, content.CreateContentTypeRequest{
		Name:   "Page",
		Schema: map[string]any{"fields": []any{"title", "body"}},
		Capabilities: map[string]any{
			"delivery": map[string]any{
				"kind": "page",
				"menu": map[string]any{
					"location": "site.main",
				},
			},
		},
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
				Locale: "es",
				Title:  "Inicio",
				Content: map[string]any{
					"body": "Contenido",
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
				Locale: "es",
				Title:  "Inicio",
				Path:   "/inicio",
			},
		},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	record, err := module.AdminPageRead().Get(ctx, pageRecord.ID.String(), cms.AdminPageGetOptions{
		Locale:                   "en",
		FallbackLocale:           "es",
		AllowMissingTranslations: true,
		IncludeData:              true,
	})
	if err != nil {
		t.Fatalf("admin page get: %v", err)
	}
	if record == nil || record.Data == nil {
		t.Fatalf("expected admin page data payload")
	}

	expectString := func(key, want string) {
		t.Helper()
		got, ok := record.Data[key].(string)
		if !ok || got != want {
			t.Fatalf("expected data[%s]=%q got %#v", key, want, record.Data[key])
		}
	}
	expectString("requested_locale", "en")
	expectString("resolved_locale", "es")
	expectString("translation_group_id", record.TranslationGroupID.String())

	missing, ok := record.Data["missing_requested_locale"].(bool)
	if !ok || !missing {
		t.Fatalf("expected missing_requested_locale=true got %#v", record.Data["missing_requested_locale"])
	}
	locales, ok := record.Data["available_locales"].([]string)
	if !ok || len(locales) != 1 || locales[0] != "es" {
		t.Fatalf("expected available_locales [es], got %#v", record.Data["available_locales"])
	}
}
