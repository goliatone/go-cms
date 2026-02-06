package pages

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

func TestPageServiceAvailableLocalesResolvesCodes(t *testing.T) {
	ctx := context.Background()
	pageRepo := NewMemoryPageRepository()
	contentRepo := content.NewMemoryContentRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	svc := NewService(pageRepo, contentRepo, localeRepo)

	page := &Page{
		ID:         uuid.New(),
		ContentID:  uuid.New(),
		TemplateID: uuid.New(),
		Slug:       "home",
		Status:     "draft",
		Translations: []*PageTranslation{
			{
				ID:       uuid.New(),
				LocaleID: localeID,
				Title:    "Home",
				Path:     "/home",
			},
		},
	}

	if _, err := pageRepo.Create(ctx, page); err != nil {
		t.Fatalf("create page: %v", err)
	}

	locales, err := svc.AvailableLocales(ctx, page.ID, interfaces.TranslationCheckOptions{})
	if err != nil {
		t.Fatalf("AvailableLocales error: %v", err)
	}
	if len(locales) != 1 || locales[0] != "en" {
		t.Fatalf("expected locales [en], got %v", locales)
	}
}

func TestPageServiceCheckTranslationsAcceptsLocaleID(t *testing.T) {
	ctx := context.Background()
	pageRepo := NewMemoryPageRepository()
	contentRepo := content.NewMemoryContentRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	svc := NewService(pageRepo, contentRepo, localeRepo)

	page := &Page{
		ID:         uuid.New(),
		ContentID:  uuid.New(),
		TemplateID: uuid.New(),
		Slug:       "home",
		Status:     "draft",
		Translations: []*PageTranslation{
			{
				ID:       uuid.New(),
				LocaleID: localeID,
				Title:    "Home",
				Path:     "/home",
			},
		},
	}

	if _, err := pageRepo.Create(ctx, page); err != nil {
		t.Fatalf("create page: %v", err)
	}

	missing, err := svc.CheckTranslations(ctx, page.ID, []string{localeID.String()}, interfaces.TranslationCheckOptions{})
	if err != nil {
		t.Fatalf("CheckTranslations error: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("expected no missing locales, got %v", missing)
	}
}

func TestPageServiceCheckTranslationsRequiresEnvironmentMatchForNilEnvID(t *testing.T) {
	ctx := context.Background()
	pageRepo := NewMemoryPageRepository()
	contentRepo := content.NewMemoryContentRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	svc := NewService(pageRepo, contentRepo, localeRepo)

	page := &Page{
		ID:         uuid.New(),
		ContentID:  uuid.New(),
		TemplateID: uuid.New(),
		Slug:       "home",
		Status:     "draft",
		Translations: []*PageTranslation{
			{
				ID:       uuid.New(),
				LocaleID: localeID,
				Title:    "Home",
				Path:     "/home",
			},
		},
	}

	if _, err := pageRepo.Create(ctx, page); err != nil {
		t.Fatalf("create page: %v", err)
	}

	pageRepo.mu.Lock()
	pageRepo.pages[page.ID].EnvironmentID = uuid.Nil
	pageRepo.mu.Unlock()

	_, err := svc.CheckTranslations(ctx, page.ID, []string{"en"}, interfaces.TranslationCheckOptions{Environment: "staging"})
	if err == nil || !errors.Is(err, cmsenv.ErrEnvironmentNotFound) {
		t.Fatalf("expected ErrEnvironmentNotFound, got %v", err)
	}
}
