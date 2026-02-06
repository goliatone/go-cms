package content

import (
	"context"
	"errors"
	"testing"

	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

func TestContentServiceAvailableLocalesResolvesCodes(t *testing.T) {
	ctx := context.Background()
	contentRepo := NewMemoryContentRepository()
	typeRepo := NewMemoryContentTypeRepository()
	localeRepo := NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&Locale{ID: localeID, Code: "en", Display: "English"})

	svc := NewService(contentRepo, typeRepo, localeRepo)

	record := &Content{
		ID:            uuid.New(),
		ContentTypeID: uuid.New(),
		Slug:          "post",
		Status:        "draft",
		Translations: []*ContentTranslation{
			{
				ID:       uuid.New(),
				LocaleID: localeID,
				Title:    "Hello",
				Content:  map[string]any{},
			},
		},
	}

	if _, err := contentRepo.Create(ctx, record); err != nil {
		t.Fatalf("create content: %v", err)
	}

	locales, err := svc.AvailableLocales(ctx, record.ID, interfaces.TranslationCheckOptions{})
	if err != nil {
		t.Fatalf("AvailableLocales error: %v", err)
	}
	if len(locales) != 1 || locales[0] != "en" {
		t.Fatalf("expected locales [en], got %v", locales)
	}
}

func TestContentServiceCheckTranslationsAcceptsLocaleID(t *testing.T) {
	ctx := context.Background()
	contentRepo := NewMemoryContentRepository()
	typeRepo := NewMemoryContentTypeRepository()
	localeRepo := NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&Locale{ID: localeID, Code: "en", Display: "English"})

	svc := NewService(contentRepo, typeRepo, localeRepo)

	record := &Content{
		ID:            uuid.New(),
		ContentTypeID: uuid.New(),
		Slug:          "post",
		Status:        "draft",
		Translations: []*ContentTranslation{
			{
				ID:       uuid.New(),
				LocaleID: localeID,
				Title:    "Hello",
				Content:  map[string]any{},
			},
		},
	}

	if _, err := contentRepo.Create(ctx, record); err != nil {
		t.Fatalf("create content: %v", err)
	}

	missing, err := svc.CheckTranslations(ctx, record.ID, []string{localeID.String()}, interfaces.TranslationCheckOptions{})
	if err != nil {
		t.Fatalf("CheckTranslations error: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("expected no missing locales, got %v", missing)
	}
}

func TestContentServiceCheckTranslationsRequiresEnvironmentMatchForNilEnvID(t *testing.T) {
	ctx := context.Background()
	contentRepo := NewMemoryContentRepository()
	typeRepo := NewMemoryContentTypeRepository()
	localeRepo := NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&Locale{ID: localeID, Code: "en", Display: "English"})

	svc := NewService(contentRepo, typeRepo, localeRepo)

	record := &Content{
		ID:            uuid.New(),
		ContentTypeID: uuid.New(),
		Slug:          "post",
		Status:        "draft",
		Translations: []*ContentTranslation{
			{
				ID:       uuid.New(),
				LocaleID: localeID,
				Title:    "Hello",
				Content:  map[string]any{},
			},
		},
	}

	if _, err := contentRepo.Create(ctx, record); err != nil {
		t.Fatalf("create content: %v", err)
	}

	contentRepo.mu.Lock()
	contentRepo.contents[record.ID].EnvironmentID = uuid.Nil
	contentRepo.mu.Unlock()

	_, err := svc.CheckTranslations(ctx, record.ID, []string{"en"}, interfaces.TranslationCheckOptions{Environment: "staging"})
	if err == nil || !errors.Is(err, cmsenv.ErrEnvironmentNotFound) {
		t.Fatalf("expected ErrEnvironmentNotFound, got %v", err)
	}
}
