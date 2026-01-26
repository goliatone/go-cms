package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/translationconfig"
	"github.com/google/uuid"
)

func TestTranslationAdminApplySettingsUpdatesRuntimeEnforcement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := cms.DefaultConfig()

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("cms.New() error = %v", err)
	}

	typeRepo := module.Container().ContentTypeRepository()
	seedTypes, ok := typeRepo.(interface{ Put(*content.ContentType) error })
	if !ok {
		t.Fatalf("expected seedable content type repository, got %T", typeRepo)
	}
	contentTypeID := uuid.New()
	if err := seedTypes.Put(&content.ContentType{ID: contentTypeID, Name: "article", Slug: "article"}); err != nil {
		t.Fatalf("seed content type: %v", err)
	}

	authorID := uuid.New()
	contentSvc := module.Content()
	record, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "hello-world",
		Status:        "draft",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Hello World"},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := module.Pages()
	page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  record.ID,
		TemplateID: uuid.New(),
		Slug:       "hello-world",
		Status:     "draft",
		CreatedBy:  authorID,
		UpdatedBy:  authorID,
		Translations: []pages.PageTranslationInput{
			{Locale: "en", Title: "Hello World", Path: "/hello-world"},
		},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	if _, err := pageSvc.Update(ctx, pages.UpdatePageRequest{
		ID:        page.ID,
		Status:    "published",
		UpdatedBy: authorID,
	}); !errors.Is(err, pages.ErrNoPageTranslations) {
		t.Fatalf("expected ErrNoPageTranslations, got %v", err)
	}

	if _, err := contentSvc.Update(ctx, content.UpdateContentRequest{
		ID:        record.ID,
		Status:    "published",
		UpdatedBy: authorID,
	}); !errors.Is(err, content.ErrNoTranslations) {
		t.Fatalf("expected ErrNoTranslations, got %v", err)
	}

	admin := module.TranslationAdmin()
	if admin == nil {
		t.Fatalf("expected translation admin service to be initialised")
	}

	if err := admin.ApplySettings(ctx, translationconfig.Settings{
		TranslationsEnabled: true,
		RequireTranslations: false,
	}); err != nil {
		t.Fatalf("ApplySettings() error = %v", err)
	}

	verifyCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	waitFor(t, verifyCtx, func() error {
		if module.TranslationsRequired() {
			return fmt.Errorf("translations still required")
		}
		return nil
	})

	if _, err := pageSvc.Update(ctx, pages.UpdatePageRequest{
		ID:        page.ID,
		Status:    "published",
		UpdatedBy: authorID,
	}); err != nil {
		t.Fatalf("expected status-only update to succeed after toggle, got %v", err)
	}

	if _, err := contentSvc.Update(ctx, content.UpdateContentRequest{
		ID:        record.ID,
		Status:    "published",
		UpdatedBy: authorID,
	}); err != nil {
		t.Fatalf("expected content status-only update to succeed after toggle, got %v", err)
	}

	if err := admin.ApplySettings(ctx, translationconfig.Settings{
		TranslationsEnabled: false,
		RequireTranslations: true,
	}); err != nil {
		t.Fatalf("ApplySettings() disable error = %v", err)
	}

	verifyCtx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
	defer cancel2()

	waitFor(t, verifyCtx2, func() error {
		if module.TranslationsEnabled() {
			return fmt.Errorf("translations still enabled")
		}
		return nil
	})

	if _, err := pageSvc.UpdateTranslation(ctx, pages.UpdatePageTranslationRequest{
		PageID:    page.ID,
		Locale:    "en",
		Title:     "Hello",
		Path:      "/hello-world",
		UpdatedBy: authorID,
	}); !errors.Is(err, pages.ErrPageTranslationsDisabled) {
		t.Fatalf("expected ErrPageTranslationsDisabled, got %v", err)
	}

	if _, err := contentSvc.UpdateTranslation(ctx, content.UpdateContentTranslationRequest{
		ContentID: record.ID,
		Locale:    "en",
		Title:     "Hello",
		Content:   map[string]any{"body": "updated"},
		UpdatedBy: authorID,
	}); !errors.Is(err, content.ErrContentTranslationsDisabled) {
		t.Fatalf("expected ErrContentTranslationsDisabled, got %v", err)
	}
}
