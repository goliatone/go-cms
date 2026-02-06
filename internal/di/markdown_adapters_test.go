package di

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

type markdownContentFixture struct {
	ctx  context.Context
	svc  interfaces.ContentService
	slug string
}

func TestMarkdownContentAdapterMissingTranslationAllowed(t *testing.T) {
	fixture := newMarkdownContentFixture(t, []content.ContentTranslationInput{
		{
			Locale: "es",
			Title:  "Hola",
			Content: map[string]any{
				"body": "Hola mundo",
			},
		},
	})

	record, err := fixture.svc.GetBySlug(fixture.ctx, fixture.slug, interfaces.ContentReadOptions{
		Locale:                   "en",
		AllowMissingTranslations: true,
		IncludeAvailableLocales:  true,
	})
	if err != nil {
		t.Fatalf("get content: %v", err)
	}
	if record == nil {
		t.Fatalf("expected record, got nil")
	}

	meta := record.Translation.Meta
	if meta.RequestedLocale != "en" {
		t.Fatalf("expected requested locale %q, got %q", "en", meta.RequestedLocale)
	}
	if meta.PrimaryLocale != "es" {
		t.Fatalf("expected primary locale %q, got %q", "es", meta.PrimaryLocale)
	}
	if meta.PrimaryLocale != "es" {
		t.Fatalf("expected primary locale %q, got %q", "es", meta.PrimaryLocale)
	}
	if meta.ResolvedLocale != "" {
		t.Fatalf("expected empty resolved locale, got %q", meta.ResolvedLocale)
	}
	if !meta.MissingRequestedLocale {
		t.Fatalf("expected missing requested locale")
	}
	if meta.FallbackUsed {
		t.Fatalf("expected fallback not used")
	}
	if !reflect.DeepEqual(meta.AvailableLocales, []string{"es"}) {
		t.Fatalf("expected available locales [\"es\"], got %#v", meta.AvailableLocales)
	}
	if record.Translation.Requested != nil {
		t.Fatalf("expected nil requested translation, got %#v", record.Translation.Requested)
	}
	if record.Translation.Resolved != nil {
		t.Fatalf("expected nil resolved translation, got %#v", record.Translation.Resolved)
	}
}

func TestMarkdownContentAdapterFallbackMetadata(t *testing.T) {
	fixture := newMarkdownContentFixture(t, []content.ContentTranslationInput{
		{
			Locale: "es",
			Title:  "Hola",
			Content: map[string]any{
				"body": "Hola mundo",
			},
		},
	})

	record, err := fixture.svc.GetBySlug(fixture.ctx, fixture.slug, interfaces.ContentReadOptions{
		Locale:                   "en",
		FallbackLocale:           "es",
		AllowMissingTranslations: true,
		IncludeAvailableLocales:  true,
	})
	if err != nil {
		t.Fatalf("get content: %v", err)
	}
	if record == nil {
		t.Fatalf("expected record, got nil")
	}

	meta := record.Translation.Meta
	if meta.RequestedLocale != "en" {
		t.Fatalf("expected requested locale %q, got %q", "en", meta.RequestedLocale)
	}
	if meta.ResolvedLocale != "es" {
		t.Fatalf("expected resolved locale %q, got %q", "es", meta.ResolvedLocale)
	}
	if !meta.MissingRequestedLocale {
		t.Fatalf("expected missing requested locale")
	}
	if !meta.FallbackUsed {
		t.Fatalf("expected fallback used")
	}
	if !reflect.DeepEqual(meta.AvailableLocales, []string{"es"}) {
		t.Fatalf("expected available locales [\"es\"], got %#v", meta.AvailableLocales)
	}
	if record.Translation.Requested != nil {
		t.Fatalf("expected nil requested translation, got %#v", record.Translation.Requested)
	}
	if record.Translation.Resolved == nil {
		t.Fatalf("expected resolved translation")
	}
	if record.Translation.Resolved.Locale != "es" {
		t.Fatalf("expected resolved locale %q, got %q", "es", record.Translation.Resolved.Locale)
	}
	if record.Translation.Resolved.Title != "Hola" {
		t.Fatalf("expected resolved title %q, got %q", "Hola", record.Translation.Resolved.Title)
	}
}

func TestMarkdownContentAdapterMissingTranslationError(t *testing.T) {
	fixture := newMarkdownContentFixture(t, []content.ContentTranslationInput{
		{
			Locale: "es",
			Title:  "Hola",
			Content: map[string]any{
				"body": "Hola mundo",
			},
		},
	})

	_, err := fixture.svc.GetBySlug(fixture.ctx, fixture.slug, interfaces.ContentReadOptions{
		Locale: "en",
	})
	if !errors.Is(err, interfaces.ErrTranslationMissing) {
		t.Fatalf("expected ErrTranslationMissing, got %v", err)
	}
}

func newMarkdownContentFixture(t *testing.T, translations []content.ContentTranslationInput) markdownContentFixture {
	t.Helper()

	ctx := context.Background()
	contentID := uuid.New()
	contentTypeID := uuid.New()
	contentType := &content.ContentType{
		ID:   contentTypeID,
		Name: "article",
		Slug: "article",
	}
	seenLocales := map[string]struct{}{}
	primaryLocale := ""
	translationRecords := make([]*content.ContentTranslation, 0, len(translations))
	for _, tr := range translations {
		code := strings.TrimSpace(tr.Locale)
		if code == "" {
			continue
		}
		if _, ok := seenLocales[code]; ok {
			continue
		}
		seenLocales[code] = struct{}{}
		if primaryLocale == "" {
			primaryLocale = code
		}
		locale := &content.Locale{
			ID:       uuid.New(),
			Code:     code,
			Display:  code,
			IsActive: true,
		}
		translationRecords = append(translationRecords, &content.ContentTranslation{
			ID:        uuid.New(),
			ContentID: contentID,
			LocaleID:  locale.ID,
			Locale:    locale,
			Title:     tr.Title,
			Summary:   tr.Summary,
			Content:   tr.Content,
		})
	}

	slug := "content-" + uuid.NewString()
	record := &content.Content{
		ID:            contentID,
		ContentTypeID: contentTypeID,
		Slug:          slug,
		Status:        string(domain.StatusDraft),
		PrimaryLocale: primaryLocale,
		Translations:  translationRecords,
		Type:          contentType,
	}

	adapter := newMarkdownContentServiceAdapter(&stubContentService{
		records: []*content.Content{record},
	})
	return markdownContentFixture{
		ctx:  ctx,
		svc:  adapter,
		slug: slug,
	}
}

type stubContentService struct {
	records []*content.Content
}

func (s *stubContentService) Create(context.Context, content.CreateContentRequest) (*content.Content, error) {
	return nil, errors.New("stub content service")
}

func (s *stubContentService) Get(context.Context, uuid.UUID) (*content.Content, error) {
	return nil, errors.New("stub content service")
}

func (s *stubContentService) List(context.Context, ...string) ([]*content.Content, error) {
	return s.records, nil
}

func (s *stubContentService) CheckTranslations(context.Context, uuid.UUID, []string, interfaces.TranslationCheckOptions) ([]string, error) {
	return nil, errors.New("stub content service")
}

func (s *stubContentService) AvailableLocales(context.Context, uuid.UUID, interfaces.TranslationCheckOptions) ([]string, error) {
	return nil, errors.New("stub content service")
}

func (s *stubContentService) Update(context.Context, content.UpdateContentRequest) (*content.Content, error) {
	return nil, errors.New("stub content service")
}

func (s *stubContentService) Delete(context.Context, content.DeleteContentRequest) error {
	return errors.New("stub content service")
}

func (s *stubContentService) UpdateTranslation(context.Context, content.UpdateContentTranslationRequest) (*content.ContentTranslation, error) {
	return nil, errors.New("stub content service")
}

func (s *stubContentService) DeleteTranslation(context.Context, content.DeleteContentTranslationRequest) error {
	return errors.New("stub content service")
}

func (s *stubContentService) Schedule(context.Context, content.ScheduleContentRequest) (*content.Content, error) {
	return nil, errors.New("stub content service")
}

func (s *stubContentService) CreateDraft(context.Context, content.CreateContentDraftRequest) (*content.ContentVersion, error) {
	return nil, errors.New("stub content service")
}

func (s *stubContentService) PublishDraft(context.Context, content.PublishContentDraftRequest) (*content.ContentVersion, error) {
	return nil, errors.New("stub content service")
}

func (s *stubContentService) PreviewDraft(context.Context, content.PreviewContentDraftRequest) (*content.ContentPreview, error) {
	return nil, errors.New("stub content service")
}

func (s *stubContentService) ListVersions(context.Context, uuid.UUID) ([]*content.ContentVersion, error) {
	return nil, errors.New("stub content service")
}

func (s *stubContentService) RestoreVersion(context.Context, content.RestoreContentVersionRequest) (*content.ContentVersion, error) {
	return nil, errors.New("stub content service")
}
