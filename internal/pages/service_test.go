package pages_test

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/internal/workflow"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

type fakeWorkflowEngine struct {
	states []interfaces.WorkflowState
	calls  []interfaces.TransitionInput
	events [][]interfaces.WorkflowEvent
}

func (f *fakeWorkflowEngine) Transition(ctx context.Context, input interfaces.TransitionInput) (*interfaces.TransitionResult, error) {
	f.calls = append(f.calls, input)

	var target interfaces.WorkflowState
	if len(f.states) > 0 {
		target = f.states[0]
		f.states = f.states[1:]
	} else if strings.TrimSpace(string(input.TargetState)) != "" {
		target = interfaces.WorkflowState(domain.NormalizeWorkflowState(string(input.TargetState)))
	} else {
		target = input.CurrentState
	}

	var emitted []interfaces.WorkflowEvent
	if len(f.events) > 0 {
		emitted = f.events[0]
		f.events = f.events[1:]
	}

	return &interfaces.TransitionResult{
		EntityID:    input.EntityID,
		EntityType:  input.EntityType,
		Transition:  input.Transition,
		FromState:   input.CurrentState,
		ToState:     target,
		CompletedAt: time.Unix(0, 0).UTC(),
		ActorID:     input.ActorID,
		Metadata:    input.Metadata,
		Events:      emitted,
	}, nil
}

func (f *fakeWorkflowEngine) AvailableTransitions(ctx context.Context, query interfaces.TransitionQuery) ([]interfaces.WorkflowTransition, error) {
	return nil, nil
}

func (f *fakeWorkflowEngine) RegisterWorkflow(ctx context.Context, definition interfaces.WorkflowDefinition) error {
	return nil
}

type logEntry struct {
	msg    string
	fields map[string]any
}

type logStorage struct {
	entries []logEntry
}

type recordingLogger struct {
	store  *logStorage
	fields map[string]any
}

func newRecordingLogger() *recordingLogger {
	return &recordingLogger{
		store:  &logStorage{},
		fields: map[string]any{},
	}
}

func (l *recordingLogger) Trace(string, ...any) {}
func (l *recordingLogger) Debug(string, ...any) {}
func (l *recordingLogger) Warn(string, ...any)  {}
func (l *recordingLogger) Error(string, ...any) {}
func (l *recordingLogger) Fatal(string, ...any) {}

func (l *recordingLogger) Info(msg string, kv ...any) {
	fields := maps.Clone(l.fields)
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			continue
		}
		fields[key] = kv[i+1]
	}
	l.store.entries = append(l.store.entries, logEntry{
		msg:    msg,
		fields: fields,
	})
}

func (l *recordingLogger) WithFields(fields map[string]any) interfaces.Logger {
	merged := maps.Clone(l.fields)
	for k, v := range fields {
		merged[k] = v
	}
	return &recordingLogger{
		store:  l.store,
		fields: merged,
	}
}

func (l *recordingLogger) WithContext(context.Context) interfaces.Logger {
	return &recordingLogger{
		store:  l.store,
		fields: maps.Clone(l.fields),
	}
}

func (l *recordingLogger) entries() []logEntry {
	return l.store.entries
}

func TestPageServiceCreateSuccess(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	typeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: typeID, Name: "page"})

	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	createdContent, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "welcome",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Welcome",
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := pages.NewService(pageStore, contentStore, localeStore, pages.WithPageClock(func() time.Time {
		return time.Unix(0, 0)
	}))

	req := pages.CreatePageRequest{
		ContentID:  createdContent.ID,
		TemplateID: uuid.New(),
		Slug:       "home",
		CreatedBy:  uuid.New(),
		UpdatedBy:  uuid.New(),
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Home",
			Path:   "/",
		}},
	}

	result, err := pageSvc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	if result.Slug != req.Slug {
		t.Fatalf("expected slug %q got %q", req.Slug, result.Slug)
	}

	if len(result.Translations) != 1 {
		t.Fatalf("expected 1 translation got %d", len(result.Translations))
	}

	if result.Translations[0].Path != "/" {
		t.Fatalf("expected path '/' got %q", result.Translations[0].Path)
	}
}

func TestPageServiceUpdateTranslation(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	contentTypeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})
	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "update-page-translation",
		Status:        string(domain.StatusDraft),
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Body",
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	svc := pages.NewService(pageStore, contentStore, localeStore)
	page, err := svc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:    contentRecord.ID,
		TemplateID:   uuid.New(),
		Slug:         "update-translation",
		Status:       string(domain.StatusDraft),
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
		Translations: []pages.PageTranslationInput{{Locale: "en", Title: "Hello", Path: "/hello"}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	editor := uuid.New()
	translation, err := svc.UpdateTranslation(context.Background(), pages.UpdatePageTranslationRequest{
		PageID:    page.ID,
		Locale:    "en",
		Title:     "Updated Title",
		Path:      "/hello-updated",
		Summary:   ptr("Updated summary"),
		UpdatedBy: editor,
	})
	if err != nil {
		t.Fatalf("update translation: %v", err)
	}
	if translation.Path != "/hello-updated" {
		t.Fatalf("expected updated path, got %s", translation.Path)
	}
	if translation.Title != "Updated Title" {
		t.Fatalf("expected updated title, got %s", translation.Title)
	}

	reloaded, err := svc.Get(context.Background(), page.ID)
	if err != nil {
		t.Fatalf("get page: %v", err)
	}
	if reloaded.UpdatedBy != editor {
		t.Fatalf("expected updated_by %s got %s", editor, reloaded.UpdatedBy)
	}
	if reloaded.Translations[0].Path != "/hello-updated" {
		t.Fatalf("expected stored translation path to update")
	}
}

func TestPageServiceDeleteTranslationRequiresMinimum(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	contentTypeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})
	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "delete-page-translation",
		Status:        string(domain.StatusDraft),
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Body",
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	svc := pages.NewService(pageStore, contentStore, localeStore)
	page, err := svc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:    contentRecord.ID,
		TemplateID:   uuid.New(),
		Slug:         "delete-translation",
		Status:       string(domain.StatusDraft),
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
		Translations: []pages.PageTranslationInput{{Locale: "en", Title: "Hello", Path: "/delete"}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	err = svc.DeleteTranslation(context.Background(), pages.DeletePageTranslationRequest{
		PageID:    page.ID,
		Locale:    "en",
		DeletedBy: uuid.New(),
	})
	if !errors.Is(err, pages.ErrNoPageTranslations) {
		t.Fatalf("expected ErrNoPageTranslations got %v", err)
	}
}

func TestPageServiceMovePreventsCycle(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	contentTypeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})
	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	parentContent, _ := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "parent-content",
		Status:        string(domain.StatusDraft),
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Parent Body"}},
	})
	childContent, _ := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "child-content",
		Status:        string(domain.StatusDraft),
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Child Body"}},
	})

	svc := pages.NewService(pageStore, contentStore, localeStore)
	parentPage, err := svc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:    parentContent.ID,
		TemplateID:   uuid.New(),
		Slug:         "parent",
		Status:       string(domain.StatusDraft),
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
		Translations: []pages.PageTranslationInput{{Locale: "en", Title: "Parent", Path: "/parent"}},
	})
	if err != nil {
		t.Fatalf("create parent page: %v", err)
	}

	childPage, err := svc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:    childContent.ID,
		TemplateID:   uuid.New(),
		ParentID:     &parentPage.ID,
		Slug:         "child",
		Status:       string(domain.StatusDraft),
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
		Translations: []pages.PageTranslationInput{{Locale: "en", Title: "Child", Path: "/parent/child"}},
	})
	if err != nil {
		t.Fatalf("create child page: %v", err)
	}

	_, err = svc.Move(context.Background(), pages.MovePageRequest{
		PageID:      parentPage.ID,
		NewParentID: &childPage.ID,
		ActorID:     uuid.New(),
	})
	if !errors.Is(err, pages.ErrPageParentCycle) {
		t.Fatalf("expected ErrPageParentCycle got %v", err)
	}
}

func TestPageServiceDuplicateCreatesUniqueSlug(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	contentTypeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})
	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, _ := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "duplicate-content",
		Status:        string(domain.StatusDraft),
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Body"}},
	})

	svc := pages.NewService(pageStore, contentStore, localeStore)
	original, err := svc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:    contentRecord.ID,
		TemplateID:   uuid.New(),
		Slug:         "landing",
		Status:       string(domain.StatusDraft),
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
		Translations: []pages.PageTranslationInput{{Locale: "en", Title: "Landing", Path: "/landing"}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	clone, err := svc.Duplicate(context.Background(), pages.DuplicatePageRequest{
		PageID:    original.ID,
		CreatedBy: uuid.New(),
		UpdatedBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("duplicate page: %v", err)
	}

	if clone.ID == original.ID {
		t.Fatalf("expected new page id, got same")
	}
	if clone.Slug == original.Slug {
		t.Fatalf("expected slug change, both %s", clone.Slug)
	}
	if !strings.HasSuffix(clone.Slug, "-copy") {
		t.Fatalf("expected slug to include -copy suffix, got %s", clone.Slug)
	}
	if len(clone.Translations) != len(original.Translations) {
		t.Fatalf("expected translation count match, got %d vs %d", len(clone.Translations), len(original.Translations))
	}
	if clone.Translations[0].Path == original.Translations[0].Path {
		t.Fatalf("expected duplicated path to differ")
	}
}

func ptr(value string) *string {
	return &value
}

func TestPageServiceCreateWithoutTranslationsWhenOptional(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	typeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: typeID, Name: "page"})

	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	createdContent, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "landing",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Landing"},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := pages.NewService(
		pageStore,
		contentStore,
		localeStore,
		pages.WithPageClock(func() time.Time { return time.Unix(0, 0) }),
		pages.WithRequireTranslations(false),
	)

	createReq := pages.CreatePageRequest{
		ContentID:  createdContent.ID,
		TemplateID: uuid.New(),
		Slug:       "landing",
		CreatedBy:  uuid.New(),
		UpdatedBy:  uuid.New(),
	}

	ctx := context.Background()
	page, err := pageSvc.Create(ctx, createReq)
	if err != nil {
		t.Fatalf("create page without translations: %v", err)
	}
	if len(page.Translations) != 0 {
		t.Fatalf("expected zero translations, got %d", len(page.Translations))
	}

	updateReq := pages.UpdatePageRequest{
		ID:        page.ID,
		Status:    string(domain.StatusPublished),
		UpdatedBy: uuid.New(),
	}

	updated, err := pageSvc.Update(ctx, updateReq)
	if err != nil {
		t.Fatalf("update page without translations: %v", err)
	}
	if updated.Status != string(domain.StatusPublished) {
		t.Fatalf("expected status %q got %q", domain.StatusPublished, updated.Status)
	}
	if len(updated.Translations) != 0 {
		t.Fatalf("expected zero translations after update, got %d", len(updated.Translations))
	}
}

func TestPageServiceAllowMissingTranslationsOverride(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	typeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: typeID, Name: "page"})

	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "override",
		Status:        string(domain.StatusDraft),
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{
			{
				Locale: "en",
				Title:  "Override Content",
				Content: map[string]any{
					"body": "Draft body",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("seed content: %v", err)
	}

	pageSvc := pages.NewService(pageStore, contentStore, localeStore, pages.WithPageClock(func() time.Time {
		return time.Unix(0, 0)
	}))

	page, err := pageSvc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:                contentRecord.ID,
		TemplateID:               uuid.New(),
		Slug:                     "override",
		Status:                   string(domain.StatusDraft),
		CreatedBy:                uuid.New(),
		UpdatedBy:                uuid.New(),
		AllowMissingTranslations: true,
	})
	if err != nil {
		t.Fatalf("create page with allow missing: %v", err)
	}
	if len(page.Translations) != 0 {
		t.Fatalf("expected zero translations, got %d", len(page.Translations))
	}

	if _, err := pageSvc.Update(context.Background(), pages.UpdatePageRequest{
		ID:                       page.ID,
		Status:                   string(domain.StatusPublished),
		UpdatedBy:                uuid.New(),
		AllowMissingTranslations: true,
	}); err != nil {
		t.Fatalf("update page with allow missing: %v", err)
	}

	_, err = pageSvc.Update(context.Background(), pages.UpdatePageRequest{
		ID:        page.ID,
		Status:    string(domain.StatusDraft),
		UpdatedBy: uuid.New(),
	})
	if !errors.Is(err, pages.ErrNoPageTranslations) {
		t.Fatalf("expected ErrNoPageTranslations without override, got %v", err)
	}
}

func TestPageServiceListVersionsWithTranslationlessPage(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	typeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: typeID, Name: "page"})

	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	createdContent, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "landing",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Landing"},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := pages.NewService(
		pageStore,
		contentStore,
		localeStore,
		pages.WithRequireTranslations(false),
		pages.WithPageVersioningEnabled(true),
	)

	page, err := pageSvc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:  createdContent.ID,
		TemplateID: uuid.New(),
		Slug:       "landing",
		CreatedBy:  uuid.New(),
		UpdatedBy:  uuid.New(),
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	versions, err := pageSvc.ListVersions(context.Background(), page.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("expected zero versions, got %d", len(versions))
	}
}

func TestPageServiceCreateDuplicateSlug(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	typeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: typeID, Name: "page"})
	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "baseline",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Baseline"}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := pages.NewService(pageStore, contentStore, localeStore)

	if _, err := pageSvc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:    contentRecord.ID,
		TemplateID:   uuid.New(),
		Slug:         "news",
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
		Translations: []pages.PageTranslationInput{{Locale: "en", Title: "News", Path: "/news"}},
	}); err != nil {
		t.Fatalf("first page create: %v", err)
	}

	_, err = pageSvc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:    contentRecord.ID,
		TemplateID:   uuid.New(),
		Slug:         "news",
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
		Translations: []pages.PageTranslationInput{{Locale: "en", Title: "News Again", Path: "/news-2"}},
	})
	if !errors.Is(err, pages.ErrSlugExists) {
		t.Fatalf("expected ErrSlugExists got %v", err)
	}
}

func TestPageServiceCreateUnknownLocale(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	typeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: typeID, Name: "page"})
	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "article",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Article"}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := pages.NewService(pageStore, contentStore, localeStore)

	_, err = pageSvc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:    contentRecord.ID,
		TemplateID:   uuid.New(),
		Slug:         "article",
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
		Translations: []pages.PageTranslationInput{{Locale: "es", Title: "Articulo", Path: "/articulo"}},
	})
	if !errors.Is(err, pages.ErrUnknownLocale) {
		t.Fatalf("expected ErrUnknownLocale got %v", err)
	}
}

func TestPageServiceVersionLifecycle(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	contentTypeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	createdContent, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "page-versioned",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Versioned"}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	fixedNow := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	pageSvc := pages.NewService(
		pageStore,
		contentStore,
		localeStore,
		pages.WithPageClock(func() time.Time { return fixedNow }),
		pages.WithPageVersioningEnabled(true),
		pages.WithPageVersionRetentionLimit(5),
	)

	ctx := context.Background()
	pageAuthor := uuid.New()
	page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  createdContent.ID,
		TemplateID: uuid.New(),
		Slug:       "landing",
		CreatedBy:  pageAuthor,
		UpdatedBy:  pageAuthor,
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Landing",
			Path:   "/landing",
		}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	blockID := uuid.New()
	instanceID := uuid.New()
	draftSnapshot := pages.PageVersionSnapshot{
		Regions: map[string][]pages.PageBlockPlacement{
			"hero": {{
				Region:     "hero",
				Position:   0,
				BlockID:    blockID,
				InstanceID: instanceID,
				Snapshot:   map[string]any{"headline": "Draft"},
			}},
		},
		Metadata: map[string]any{"notes": "initial"},
	}

	firstDraft, err := pageSvc.CreateDraft(ctx, pages.CreatePageDraftRequest{
		PageID:    page.ID,
		Snapshot:  draftSnapshot,
		CreatedBy: pageAuthor,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if firstDraft.Version != 1 {
		t.Fatalf("expected version 1 got %d", firstDraft.Version)
	}
	if firstDraft.Status != domain.StatusDraft {
		t.Fatalf("expected draft status got %s", firstDraft.Status)
	}

	publishUser := uuid.New()
	publishedAt := time.Date(2024, 3, 16, 10, 0, 0, 0, time.UTC)
	published, err := pageSvc.PublishDraft(ctx, pages.PublishPagePublishRequest{
		PageID:      page.ID,
		Version:     firstDraft.Version,
		PublishedBy: publishUser,
		PublishedAt: &publishedAt,
	})
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}
	if published.Status != domain.StatusPublished {
		t.Fatalf("expected published status got %s", published.Status)
	}

	secondSnapshot := pages.PageVersionSnapshot{
		Regions: map[string][]pages.PageBlockPlacement{
			"hero": {{
				Region:     "hero",
				Position:   0,
				BlockID:    blockID,
				InstanceID: instanceID,
				Snapshot:   map[string]any{"headline": "Updated"},
			}},
		},
	}
	baseVersion := published.Version
	secondDraft, err := pageSvc.CreateDraft(ctx, pages.CreatePageDraftRequest{
		PageID:      page.ID,
		Snapshot:    secondSnapshot,
		CreatedBy:   pageAuthor,
		UpdatedBy:   pageAuthor,
		BaseVersion: &baseVersion,
	})
	if err != nil {
		t.Fatalf("create second draft: %v", err)
	}
	if secondDraft.Version != 2 {
		t.Fatalf("expected second draft version 2 got %d", secondDraft.Version)
	}

	secondPublisher := uuid.New()
	secondPublished, err := pageSvc.PublishDraft(ctx, pages.PublishPagePublishRequest{
		PageID:      page.ID,
		Version:     secondDraft.Version,
		PublishedBy: secondPublisher,
	})
	if err != nil {
		t.Fatalf("publish second draft: %v", err)
	}
	if secondPublished.Status != domain.StatusPublished {
		t.Fatalf("expected second version published got %s", secondPublished.Status)
	}

	versions, err := pageSvc.ListVersions(ctx, page.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions got %d", len(versions))
	}
	if versions[0].Status != domain.StatusArchived {
		t.Fatalf("expected first version archived got %s", versions[0].Status)
	}
	if versions[1].Status != domain.StatusPublished {
		t.Fatalf("expected second version published got %s", versions[1].Status)
	}

	restorer := uuid.New()
	restored, err := pageSvc.RestoreVersion(ctx, pages.RestorePageVersionRequest{
		PageID:     page.ID,
		Version:    1,
		RestoredBy: restorer,
	})
	if err != nil {
		t.Fatalf("restore version: %v", err)
	}
	if restored.Version != 3 {
		t.Fatalf("expected restored version 3 got %d", restored.Version)
	}
	if restored.Status != domain.StatusDraft {
		t.Fatalf("expected restored draft status got %s", restored.Status)
	}

	allVersions, err := pageSvc.ListVersions(ctx, page.ID)
	if err != nil {
		t.Fatalf("list versions after restore: %v", err)
	}
	if len(allVersions) != 3 {
		t.Fatalf("expected 3 versions got %d", len(allVersions))
	}
	if allVersions[2].Status != domain.StatusDraft {
		t.Fatalf("expected newest version draft got %s", allVersions[2].Status)
	}

	updatedPage, err := pageSvc.Get(ctx, page.ID)
	if err != nil {
		t.Fatalf("get page: %v", err)
	}
	if updatedPage.PublishedVersion == nil || *updatedPage.PublishedVersion != 2 {
		t.Fatalf("expected published version pointer 2 got %v", updatedPage.PublishedVersion)
	}
	if updatedPage.CurrentVersion != 3 {
		t.Fatalf("expected current version 3 got %d", updatedPage.CurrentVersion)
	}
}

func TestPageServiceWorkflowCustomEngine(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	typeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: typeID, Name: "page"})

	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "workflow",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Workflow",
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	fake := &fakeWorkflowEngine{
		states: []interfaces.WorkflowState{
			interfaces.WorkflowState("review"),
			interfaces.WorkflowState("approved"),
			interfaces.WorkflowState("published"),
		},
	}

	actorID := uuid.New()
	pageSvc := pages.NewService(pageStore, contentStore, localeStore, pages.WithWorkflowEngine(fake))

	createReq := pages.CreatePageRequest{
		ContentID:  contentRecord.ID,
		TemplateID: uuid.New(),
		Slug:       "workflow-page",
		Status:     "review",
		CreatedBy:  actorID,
		UpdatedBy:  actorID,
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Workflow Page",
			Path:   "/workflow",
		}},
	}

	createdPage, err := pageSvc.Create(context.Background(), createReq)
	if err != nil {
		t.Fatalf("create page: %v", err)
	}
	if createdPage.Status != "review" {
		t.Fatalf("expected status review got %s", createdPage.Status)
	}
	if createdPage.EffectiveStatus != domain.Status("review") {
		t.Fatalf("expected effective status review got %s", createdPage.EffectiveStatus)
	}

	updateReq := pages.UpdatePageRequest{
		ID:        createdPage.ID,
		Status:    "approved",
		UpdatedBy: actorID,
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Workflow Page",
			Path:   "/workflow",
		}},
	}

	updatedPage, err := pageSvc.Update(context.Background(), updateReq)
	if err != nil {
		t.Fatalf("update page: %v", err)
	}
	if updatedPage.Status != "approved" {
		t.Fatalf("expected status approved got %s", updatedPage.Status)
	}

	publishUpdate := pages.UpdatePageRequest{
		ID:        createdPage.ID,
		Status:    "published",
		UpdatedBy: actorID,
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Workflow Page",
			Path:   "/workflow",
		}},
	}

	finalPage, err := pageSvc.Update(context.Background(), publishUpdate)
	if err != nil {
		t.Fatalf("final update: %v", err)
	}
	if finalPage.Status != "published" {
		t.Fatalf("expected status published got %s", finalPage.Status)
	}
	if finalPage.EffectiveStatus != domain.StatusPublished {
		t.Fatalf("expected effective status published got %s", finalPage.EffectiveStatus)
	}

	if len(fake.calls) != 3 {
		t.Fatalf("expected 3 workflow calls got %d", len(fake.calls))
	}
	if fake.calls[0].EntityType != workflow.EntityTypePage {
		t.Fatalf("expected entity type %s got %s", workflow.EntityTypePage, fake.calls[0].EntityType)
	}
	if op, ok := fake.calls[0].Metadata["operation"]; !ok || op != "create" {
		t.Fatalf("expected first metadata operation create got %v", op)
	}
	if fake.calls[1].CurrentState != interfaces.WorkflowState("review") {
		t.Fatalf("expected second call from review got %s", fake.calls[1].CurrentState)
	}
	if fake.calls[2].CurrentState != interfaces.WorkflowState("approved") {
		t.Fatalf("expected third call from approved got %s", fake.calls[2].CurrentState)
	}
	if slug, ok := fake.calls[0].Metadata["slug"].(string); !ok || slug != "workflow-page" {
		t.Fatalf("expected slug metadata workflow-page got %v", slug)
	}
	if len(fake.states) != 0 {
		t.Fatalf("expected fake workflow states exhausted, remaining %d", len(fake.states))
	}
}

func TestPageServiceUpdateReplacesTranslations(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	contentTypeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	enLocale := uuid.New()
	esLocale := uuid.New()
	localeStore.Put(&content.Locale{ID: enLocale, Code: "en", Display: "English"})
	localeStore.Put(&content.Locale{ID: esLocale, Code: "es", Display: "Spanish"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "update-page",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Update"}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := pages.NewService(pageStore, contentStore, localeStore, pages.WithPageClock(func() time.Time {
		return time.Unix(0, 0)
	}))

	page, err := pageSvc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:  contentRecord.ID,
		TemplateID: uuid.New(),
		Slug:       "update-page",
		CreatedBy:  uuid.New(),
		UpdatedBy:  uuid.New(),
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Update",
			Path:   "/update",
		}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	newTemplate := uuid.New()
	updated, err := pageSvc.Update(context.Background(), pages.UpdatePageRequest{
		ID:         page.ID,
		TemplateID: &newTemplate,
		Status:     "published",
		UpdatedBy:  uuid.New(),
		Translations: []pages.PageTranslationInput{
			{Locale: "en", Title: "Update EN", Path: "/update"},
			{Locale: "es", Title: "Actualizar ES", Path: "/es/actualizar"},
		},
	})
	if err != nil {
		t.Fatalf("update page: %v", err)
	}

	if updated.TemplateID != newTemplate {
		t.Fatalf("expected template %s got %s", newTemplate, updated.TemplateID)
	}
	if updated.Status != "published" {
		t.Fatalf("expected status published got %s", updated.Status)
	}
	if len(updated.Translations) != 2 {
		t.Fatalf("expected 2 translations got %d", len(updated.Translations))
	}

	var hasEnglish, hasSpanish bool
	for _, tr := range updated.Translations {
		switch tr.LocaleID {
		case enLocale:
			hasEnglish = true
			if tr.Path != "/update" {
				t.Fatalf("expected en path /update got %s", tr.Path)
			}
		case esLocale:
			hasSpanish = true
			if tr.Path != "/es/actualizar" {
				t.Fatalf("expected es path /es/actualizar got %s", tr.Path)
			}
		}
	}
	if !hasEnglish || !hasSpanish {
		t.Fatalf("expected translations for both locales (en=%v es=%v)", hasEnglish, hasSpanish)
	}
}

func TestPageServiceDeleteHard(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	contentTypeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})
	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "delete-page",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Delete"}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := pages.NewService(pageStore, contentStore, localeStore)
	page, err := pageSvc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:    contentRecord.ID,
		TemplateID:   uuid.New(),
		Slug:         "delete-page",
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
		Translations: []pages.PageTranslationInput{{Locale: "en", Title: "Delete", Path: "/delete"}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	err = pageSvc.Delete(context.Background(), pages.DeletePageRequest{ID: page.ID, HardDelete: true})
	if err != nil {
		t.Fatalf("delete page: %v", err)
	}

	_, err = pageSvc.Get(context.Background(), page.ID)
	var notFound *pages.PageNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected not found error got %v", err)
	}
}

func TestPageServiceDeleteSoftUnsupported(t *testing.T) {
	pageSvc := pages.NewService(pages.NewMemoryPageRepository(), content.NewMemoryContentRepository(), content.NewMemoryLocaleRepository())
	err := pageSvc.Delete(context.Background(), pages.DeletePageRequest{ID: uuid.New(), HardDelete: false})
	if !errors.Is(err, pages.ErrPageSoftDeleteUnsupported) {
		t.Fatalf("expected ErrPageSoftDeleteUnsupported got %v", err)
	}
}

func TestPageServiceListIncludesBlocks(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	contentTypeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})
	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "landing",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Landing"}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	blockDefRepo := blocks.NewMemoryDefinitionRepository()
	blockInstRepo := blocks.NewMemoryInstanceRepository()
	blockTransRepo := blocks.NewMemoryTranslationRepository()

	blockSvc := blocks.NewService(blockDefRepo, blockInstRepo, blockTransRepo)

	definition, err := blockSvc.RegisterDefinition(context.Background(), blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	pageID := uuid.New()
	pageSvc := pages.NewService(pageStore, contentStore, localeStore, pages.WithBlockService(blockSvc), pages.WithPageClock(func() time.Time {
		return time.Unix(0, 0)
	}), pages.WithIDGenerator(func() uuid.UUID { return pageID }))

	createdPage, err := pageSvc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:  contentRecord.ID,
		TemplateID: uuid.New(),
		Slug:       "landing",
		CreatedBy:  uuid.New(),
		UpdatedBy:  uuid.New(),
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Landing",
			Path:   "/landing",
		}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	instance, err := blockSvc.CreateInstance(context.Background(), blocks.CreateInstanceInput{
		DefinitionID: definition.ID,
		PageID:       &createdPage.ID,
		Region:       "hero",
		Position:     0,
		Configuration: map[string]any{
			"layout": "full",
		},
		CreatedBy: uuid.New(),
		UpdatedBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("create block instance: %v", err)
	}

	if _, err := blockSvc.AddTranslation(context.Background(), blocks.AddTranslationInput{
		BlockInstanceID: instance.ID,
		LocaleID:        localeID,
		Content: map[string]any{
			"title": "Hero Title",
		},
	}); err != nil {
		t.Fatalf("add block translation: %v", err)
	}

	pagesList, err := pageSvc.List(context.Background())
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	if len(pagesList) != 1 {
		t.Fatalf("expected one page got %d", len(pagesList))
	}
	if len(pagesList[0].Blocks) != 1 {
		t.Fatalf("expected blocks to be attached")
	}
	if pagesList[0].Blocks[0].Region != "hero" {
		t.Fatalf("unexpected block region %s", pagesList[0].Blocks[0].Region)
	}
}

func TestPageServiceListIncludesWidgets(t *testing.T) {
	ctx := context.Background()

	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	contentTypeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	authorID := uuid.New()
	contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "features",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Features",
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	now := time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)
	widgetSvc := widgets.NewService(
		widgets.NewMemoryDefinitionRepository(),
		widgets.NewMemoryInstanceRepository(),
		widgets.NewMemoryTranslationRepository(),
		widgets.WithAreaDefinitionRepository(widgets.NewMemoryAreaDefinitionRepository()),
		widgets.WithAreaPlacementRepository(widgets.NewMemoryAreaPlacementRepository()),
		widgets.WithClock(func() time.Time { return now }),
	)

	definition, err := widgetSvc.RegisterDefinition(ctx, widgets.RegisterDefinitionInput{
		Name: "promo_banner",
		Schema: map[string]any{
			"fields": []any{
				map[string]any{"name": "headline"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register widget definition: %v", err)
	}

	instance, err := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
		DefinitionID:  definition.ID,
		Configuration: map[string]any{"headline": "Limited time offer"},
		Position:      0,
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
	})
	if err != nil {
		t.Fatalf("create widget instance: %v", err)
	}

	if _, err := widgetSvc.RegisterAreaDefinition(ctx, widgets.RegisterAreaDefinitionInput{
		Code:  "sidebar.primary",
		Name:  "Primary Sidebar",
		Scope: widgets.AreaScopeGlobal,
	}); err != nil {
		t.Fatalf("register area definition: %v", err)
	}

	if _, err := widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
		AreaCode:   "sidebar.primary",
		InstanceID: instance.ID,
	}); err != nil {
		t.Fatalf("assign widget to area: %v", err)
	}

	pageID := uuid.New()
	pageSvc := pages.NewService(
		pageStore,
		contentStore,
		localeStore,
		pages.WithWidgetService(widgetSvc),
		pages.WithPageClock(func() time.Time { return now }),
		pages.WithIDGenerator(func() uuid.UUID { return pageID }),
	)

	createdPage, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  contentRecord.ID,
		TemplateID: uuid.New(),
		Slug:       "features",
		CreatedBy:  authorID,
		UpdatedBy:  authorID,
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Features",
			Path:   "/features",
		}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}
	if createdPage.ID != pageID {
		t.Fatalf("expected deterministic page ID")
	}

	results, err := pageSvc.List(ctx)
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one page, got %d", len(results))
	}

	areaWidgets, ok := results[0].Widgets["sidebar.primary"]
	if !ok {
		t.Fatalf("expected widgets for sidebar.primary")
	}
	if len(areaWidgets) != 1 {
		t.Fatalf("expected one widget resolved, got %d", len(areaWidgets))
	}
	if areaWidgets[0] == nil || areaWidgets[0].Instance == nil {
		t.Fatalf("expected resolved widget instance")
	}
	if areaWidgets[0].Instance.ID != instance.ID {
		t.Fatalf("expected widget instance %s, got %s", instance.ID, areaWidgets[0].Instance.ID)
	}
}

func TestPageServiceLogsWorkflowEvents(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	typeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: typeID, Name: "page"})

	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	createdContent, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "workflow-events",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Workflow events"},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	fake := &fakeWorkflowEngine{
		events: [][]interfaces.WorkflowEvent{
			{
				{
					Name:      "audit.recorded",
					Timestamp: time.Unix(123, 0).UTC(),
					Payload:   map[string]any{"scope": "pages"},
				},
			},
		},
	}

	logger := newRecordingLogger()
	pageSvc := pages.NewService(pageStore, contentStore, localeStore,
		pages.WithWorkflowEngine(fake),
		pages.WithLogger(logger),
	)

	req := pages.CreatePageRequest{
		ContentID:  createdContent.ID,
		TemplateID: uuid.New(),
		Slug:       "workflow-events",
		CreatedBy:  uuid.New(),
		UpdatedBy:  uuid.New(),
		Translations: []pages.PageTranslationInput{
			{Locale: "en", Title: "Workflow events", Path: "/workflow-events"},
		},
	}

	if _, err := pageSvc.Create(context.Background(), req); err != nil {
		t.Fatalf("create page: %v", err)
	}

	found := false
	for _, entry := range logger.entries() {
		if entry.msg != "workflow event emitted" {
			continue
		}
		if entry.fields["workflow_event"] == "audit.recorded" {
			payload, ok := entry.fields["workflow_event_payload"].(map[string]any)
			if !ok || payload["scope"] != "pages" {
				t.Fatalf("expected payload scope pages, got %+v", entry.fields["workflow_event_payload"])
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected workflow event to be logged, got %#v", logger.entries())
	}
}
