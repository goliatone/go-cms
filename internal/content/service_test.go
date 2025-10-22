package content_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/google/uuid"
)

func TestServiceCreateSuccess(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{
		ID:   contentTypeID,
		Name: "page",
		Schema: map[string]any{
			"fields": []any{"body"},
		},
	})

	enID := uuid.New()
	localeStore.Put(&content.Locale{
		ID:      enID,
		Code:    "en",
		Display: "English",
	})

	svc := content.NewService(contentStore, typeStore, localeStore, content.WithClock(func() time.Time {
		return time.Unix(0, 0)
	}))

	req := content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "about-us",
		Status:        "draft",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{
			{
				Locale: "en",
				Title:  "About us",
				Content: map[string]any{
					"body": "Welcome",
				},
			},
		},
	}

	ctx := context.Background()
	result, err := svc.Create(ctx, req)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if result.Slug != req.Slug {
		t.Fatalf("expected slug %q got %q", req.Slug, result.Slug)
	}

	if len(result.Translations) != 1 {
		t.Fatalf("expected 1 translation got %d", len(result.Translations))
	}

	if result.Translations[0].LocaleID != enID {
		t.Fatalf("expected locale ID %s got %s", enID, result.Translations[0].LocaleID)
	}
}

func TestServiceCreateDuplicateSlug(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})
	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	svc := content.NewService(contentStore, typeStore, localeStore)

	ctx := context.Background()
	_, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "about",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "About"},
		},
	})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "about",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "About again"},
		},
	})
	if !errors.Is(err, content.ErrSlugExists) {
		t.Fatalf("expected ErrSlugExists got %v", err)
	}
}

func TestServiceCreateUnknownLocale(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	svc := content.NewService(contentStore, typeStore, localeStore)

	_, err := svc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "welcome",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "es", Title: "Hola"},
		},
	})
	if !errors.Is(err, content.ErrUnknownLocale) {
		t.Fatalf("expected ErrUnknownLocale got %v", err)
	}
}
