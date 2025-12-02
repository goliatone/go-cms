package content_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/pkg/activity"
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

func TestServiceCreateWithoutTranslationsWhenOptional(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	svc := content.NewService(
		contentStore,
		typeStore,
		localeStore,
		content.WithRequireTranslations(false),
	)

	ctx := context.Background()
	record, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "optional",
		Status:        string(domain.StatusDraft),
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
	})
	if err != nil {
		t.Fatalf("create content without translations: %v", err)
	}
	if len(record.Translations) != 0 {
		t.Fatalf("expected zero translations, got %d", len(record.Translations))
	}

	updated, err := svc.Update(ctx, content.UpdateContentRequest{
		ID:        record.ID,
		Status:    string(domain.StatusPublished),
		UpdatedBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("update content without translations: %v", err)
	}
	if updated.Status != string(domain.StatusPublished) {
		t.Fatalf("expected status %q got %q", domain.StatusPublished, updated.Status)
	}
	if len(updated.Translations) != 0 {
		t.Fatalf("expected zero translations after update, got %d", len(updated.Translations))
	}
}

func TestServiceCreateAllowMissingTranslationsOverride(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	svc := content.NewService(contentStore, typeStore, localeStore)

	ctx := context.Background()
	record, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID:            contentTypeID,
		Slug:                     "allow-override",
		Status:                   string(domain.StatusDraft),
		CreatedBy:                uuid.New(),
		UpdatedBy:                uuid.New(),
		AllowMissingTranslations: true,
	})
	if err != nil {
		t.Fatalf("create content with allow missing: %v", err)
	}
	if len(record.Translations) != 0 {
		t.Fatalf("expected zero translations, got %d", len(record.Translations))
	}

	if _, err := svc.Update(ctx, content.UpdateContentRequest{
		ID:                       record.ID,
		Status:                   string(domain.StatusPublished),
		UpdatedBy:                uuid.New(),
		AllowMissingTranslations: true,
	}); err != nil {
		t.Fatalf("update content with allow missing: %v", err)
	}

	_, err = svc.Update(ctx, content.UpdateContentRequest{
		ID:        record.ID,
		Status:    string(domain.StatusDraft),
		UpdatedBy: uuid.New(),
	})
	if !errors.Is(err, content.ErrNoTranslations) {
		t.Fatalf("expected ErrNoTranslations without override, got %v", err)
	}
}

func TestServiceListVersionsWithTranslationlessContent(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	svc := content.NewService(
		contentStore,
		typeStore,
		localeStore,
		content.WithRequireTranslations(false),
		content.WithVersioningEnabled(true),
	)

	ctx := context.Background()
	record, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "optional",
		Status:        string(domain.StatusDraft),
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	versions, err := svc.ListVersions(ctx, record.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("expected zero versions, got %d", len(versions))
	}
}

func TestServiceUpdateTranslation(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	enLocale := uuid.New()
	localeStore.Put(&content.Locale{ID: enLocale, Code: "en", Display: "English"})

	svc := content.NewService(contentStore, typeStore, localeStore)

	record, err := svc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "update-translation",
		Status:        string(domain.StatusDraft),
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Original",
			Content: map[string]any{
				"body": "hello",
			},
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	editorID := uuid.New()
	updated, err := svc.UpdateTranslation(context.Background(), content.UpdateContentTranslationRequest{
		ContentID: record.ID,
		Locale:    "en",
		Title:     "Updated",
		Content: map[string]any{
			"body": "updated body",
		},
		UpdatedBy: editorID,
	})
	if err != nil {
		t.Fatalf("update translation: %v", err)
	}
	if updated == nil {
		t.Fatalf("expected translation result")
	}
	if updated.Title != "Updated" {
		t.Fatalf("expected title Updated got %q", updated.Title)
	}
	if updated.Content["body"] != "updated body" {
		t.Fatalf("expected updated body field")
	}

	reloaded, err := svc.Get(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("reload content: %v", err)
	}
	if reloaded.UpdatedBy != editorID {
		t.Fatalf("expected updated_by %s got %s", editorID, reloaded.UpdatedBy)
	}
}

func TestServiceDeleteTranslationRequiresMinimum(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	svc := content.NewService(contentStore, typeStore, localeStore)

	record, err := svc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "delete-translation-required",
		Status:        string(domain.StatusDraft),
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Only",
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	err = svc.DeleteTranslation(context.Background(), content.DeleteContentTranslationRequest{
		ContentID: record.ID,
		Locale:    "en",
		DeletedBy: uuid.New(),
	})
	if !errors.Is(err, content.ErrNoTranslations) {
		t.Fatalf("expected ErrNoTranslations got %v", err)
	}
}

func TestServiceDeleteTranslationOptional(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	svc := content.NewService(
		contentStore,
		typeStore,
		localeStore,
		content.WithRequireTranslations(false),
	)

	record, err := svc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "delete-translation-optional",
		Status:        string(domain.StatusDraft),
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Only",
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	if err := svc.DeleteTranslation(context.Background(), content.DeleteContentTranslationRequest{
		ContentID: record.ID,
		Locale:    "en",
		DeletedBy: uuid.New(),
	}); err != nil {
		t.Fatalf("delete translation: %v", err)
	}

	reloaded, err := svc.Get(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("reload content: %v", err)
	}
	if len(reloaded.Translations) != 0 {
		t.Fatalf("expected no translations got %d", len(reloaded.Translations))
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

func TestServiceUpdateReplacesTranslations(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	enLocale := uuid.New()
	esLocale := uuid.New()
	localeStore.Put(&content.Locale{ID: enLocale, Code: "en", Display: "English"})
	localeStore.Put(&content.Locale{ID: esLocale, Code: "es", Display: "Spanish"})

	svc := content.NewService(contentStore, typeStore, localeStore, content.WithClock(func() time.Time {
		return time.Unix(0, 0)
	}))

	ctx := context.Background()
	authorID := uuid.New()
	created, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "company",
		Status:        "draft",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Company",
				Content: map[string]any{"body": "Hello"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	enTranslationID := created.Translations[0].ID

	updated, err := svc.Update(ctx, content.UpdateContentRequest{
		ID:        created.ID,
		Status:    "published",
		UpdatedBy: authorID,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Company Updated",
				Content: map[string]any{"body": "Updated"},
			},
			{
				Locale:  "es",
				Title:   "Empresa",
				Content: map[string]any{"body": "Hola"},
			},
		},
	})
	if err != nil {
		t.Fatalf("update content: %v", err)
	}

	if updated.Status != "published" {
		t.Fatalf("expected status published got %s", updated.Status)
	}
	if len(updated.Translations) != 2 {
		t.Fatalf("expected 2 translations got %d", len(updated.Translations))
	}

	var enFound, esFound bool
	for _, tr := range updated.Translations {
		if tr.LocaleID == enLocale {
			enFound = true
			if tr.ID != enTranslationID {
				t.Fatalf("expected en translation ID %s got %s", enTranslationID, tr.ID)
			}
			if tr.Title != "Company Updated" {
				t.Fatalf("expected updated title got %s", tr.Title)
			}
		}
		if tr.LocaleID == esLocale {
			esFound = true
			if tr.Title != "Empresa" {
				t.Fatalf("expected es title got %s", tr.Title)
			}
		}
	}
	if !enFound || !esFound {
		t.Fatalf("expected both locales present (en=%v es=%v)", enFound, esFound)
	}
}

func TestServiceDeleteHard(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})
	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	svc := content.NewService(contentStore, typeStore, localeStore)
	ctx := context.Background()
	record, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "delete-me",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Delete"}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	err = svc.Delete(ctx, content.DeleteContentRequest{ID: record.ID, HardDelete: true})
	if err != nil {
		t.Fatalf("delete content: %v", err)
	}

	_, err = svc.Get(ctx, record.ID)
	var notFound *content.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected not found error got %v", err)
	}
}

func TestServiceDeleteSoftUnsupported(t *testing.T) {
	svc := content.NewService(content.NewMemoryContentRepository(), content.NewMemoryContentTypeRepository(), content.NewMemoryLocaleRepository())
	err := svc.Delete(context.Background(), content.DeleteContentRequest{ID: uuid.New(), HardDelete: false})
	if !errors.Is(err, content.ErrContentSoftDeleteUnsupported) {
		t.Fatalf("expected soft delete unsupported error got %v", err)
	}
}

func TestServiceCreateEmitsActivityEvent(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})
	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	hook := &activity.CaptureHook{}
	emitter := activity.NewEmitter(activity.Hooks{hook}, activity.Config{
		Enabled: true,
		Channel: "cms",
	})

	actorID := uuid.New()
	svc := content.NewService(
		contentStore,
		typeStore,
		localeStore,
		content.WithActivityEmitter(emitter),
	)

	ctx := context.Background()
	record, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "activity-hooks",
		Status:        string(domain.StatusDraft),
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	if len(hook.Events) != 1 {
		t.Fatalf("expected 1 activity event, got %d", len(hook.Events))
	}
	event := hook.Events[0]
	if event.Verb != "create" {
		t.Fatalf("expected verb create got %q", event.Verb)
	}
	if event.ActorID != actorID.String() {
		t.Fatalf("expected actor %s got %s", actorID, event.ActorID)
	}
	if event.ObjectType != "content" || event.ObjectID != record.ID.String() {
		t.Fatalf("unexpected object fields: %s %s", event.ObjectType, event.ObjectID)
	}
	if event.Channel != "cms" {
		t.Fatalf("expected channel cms got %q", event.Channel)
	}
	if event.OccurredAt.IsZero() {
		t.Fatalf("expected occurred_at to be set")
	}
	if slug, ok := event.Metadata["slug"].(string); !ok || slug != "activity-hooks" {
		t.Fatalf("expected slug metadata got %v", event.Metadata["slug"])
	}
	locales, ok := event.Metadata["locales"].([]string)
	if !ok || len(locales) != 1 || locales[0] != "en" {
		t.Fatalf("expected locales metadata [en] got %v", event.Metadata["locales"])
	}
	if metaTypeID, ok := event.Metadata["content_type_id"].(string); !ok || metaTypeID != contentTypeID.String() {
		t.Fatalf("expected content_type_id %s got %v", contentTypeID, event.Metadata["content_type_id"])
	}
}

func TestServiceCreateSkipsActivityOnError(t *testing.T) {
	hook := &activity.CaptureHook{}
	emitter := activity.NewEmitter(activity.Hooks{hook}, activity.Config{Enabled: true})

	svc := content.NewService(
		content.NewMemoryContentRepository(),
		content.NewMemoryContentTypeRepository(),
		content.NewMemoryLocaleRepository(),
		content.WithActivityEmitter(emitter),
	)

	_, err := svc.Create(context.Background(), content.CreateContentRequest{
		Slug:      "missing-type",
		CreatedBy: uuid.New(),
		UpdatedBy: uuid.New(),
	})
	if err == nil {
		t.Fatalf("expected error for missing content type")
	}
	if len(hook.Events) != 0 {
		t.Fatalf("expected no activity events, got %d", len(hook.Events))
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

func TestServiceVersionLifecycle(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	typeStore.Put(&content.ContentType{ID: contentTypeID, Name: "article"})

	enLocale := uuid.New()
	esLocale := uuid.New()
	localeStore.Put(&content.Locale{ID: enLocale, Code: "en", Display: "English"})
	localeStore.Put(&content.Locale{ID: esLocale, Code: "es", Display: "Spanish"})

	fixedNow := time.Date(2024, 5, 1, 8, 0, 0, 0, time.UTC)
	svc := content.NewService(
		contentStore,
		typeStore,
		localeStore,
		content.WithClock(func() time.Time { return fixedNow }),
		content.WithVersioningEnabled(true),
		content.WithVersionRetentionLimit(5),
	)

	ctx := context.Background()
	authorID := uuid.New()
	baseContent, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "versioned-article",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Versioned Article"},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	snapshot := content.ContentVersionSnapshot{
		Fields: map[string]any{"category": "news"},
		Translations: []content.ContentVersionTranslationSnapshot{
			{Locale: "en", Title: "Draft EN", Content: map[string]any{"body": "Hello"}},
			{Locale: "es", Title: "Borrador ES", Content: map[string]any{"body": "Hola"}},
		},
	}

	draft, err := svc.CreateDraft(ctx, content.CreateContentDraftRequest{
		ContentID: baseContent.ID,
		Snapshot:  snapshot,
		CreatedBy: authorID,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	if draft.Version != 1 {
		t.Fatalf("expected version 1 got %d", draft.Version)
	}
	if draft.Status != domain.StatusDraft {
		t.Fatalf("expected draft status got %s", draft.Status)
	}

	publisherID := uuid.New()
	publishAt := time.Date(2024, 5, 2, 9, 0, 0, 0, time.UTC)
	published, err := svc.PublishDraft(ctx, content.PublishContentDraftRequest{
		ContentID:   baseContent.ID,
		Version:     draft.Version,
		PublishedBy: publisherID,
		PublishedAt: &publishAt,
	})
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}

	if published.Status != domain.StatusPublished {
		t.Fatalf("expected published status got %s", published.Status)
	}
	if published.PublishedAt == nil || !published.PublishedAt.Equal(publishAt) {
		t.Fatalf("expected published_at %v got %v", publishAt, published.PublishedAt)
	}
	if published.PublishedBy == nil || *published.PublishedBy != publisherID {
		t.Fatalf("expected published_by %s", publisherID)
	}

	base := published.Version
	secondSnapshot := content.ContentVersionSnapshot{
		Fields:       map[string]any{"category": "news", "priority": "high"},
		Translations: []content.ContentVersionTranslationSnapshot{{Locale: "en", Title: "Draft EN v2", Content: map[string]any{"body": "Updated"}}},
	}

	draftTwo, err := svc.CreateDraft(ctx, content.CreateContentDraftRequest{
		ContentID:   baseContent.ID,
		Snapshot:    secondSnapshot,
		CreatedBy:   authorID,
		UpdatedBy:   authorID,
		BaseVersion: &base,
	})
	if err != nil {
		t.Fatalf("create second draft: %v", err)
	}
	if draftTwo.Version != 2 {
		t.Fatalf("expected version 2 got %d", draftTwo.Version)
	}

	secondPublisher := uuid.New()
	secondPublished, err := svc.PublishDraft(ctx, content.PublishContentDraftRequest{
		ContentID:   baseContent.ID,
		Version:     draftTwo.Version,
		PublishedBy: secondPublisher,
	})
	if err != nil {
		t.Fatalf("publish second draft: %v", err)
	}
	if secondPublished.Status != domain.StatusPublished {
		t.Fatalf("expected published status for second version got %s", secondPublished.Status)
	}

	versions, err := svc.ListVersions(ctx, baseContent.ID)
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
	if len(versions[0].Snapshot.Translations) != 2 {
		t.Fatalf("expected 2 translations in snapshot got %d", len(versions[0].Snapshot.Translations))
	}

	restorer := uuid.New()
	restored, err := svc.RestoreVersion(ctx, content.RestoreContentVersionRequest{
		ContentID:  baseContent.ID,
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
		t.Fatalf("expected restored version draft got %s", restored.Status)
	}

	allVersions, err := svc.ListVersions(ctx, baseContent.ID)
	if err != nil {
		t.Fatalf("list versions after restore: %v", err)
	}
	if len(allVersions) != 3 {
		t.Fatalf("expected 3 versions got %d", len(allVersions))
	}
	if allVersions[2].Status != domain.StatusDraft {
		t.Fatalf("expected newest version draft got %s", allVersions[2].Status)
	}

	updatedContent, err := svc.Get(ctx, baseContent.ID)
	if err != nil {
		t.Fatalf("get content: %v", err)
	}
	if updatedContent.PublishedVersion == nil || *updatedContent.PublishedVersion != 2 {
		t.Fatalf("expected published version pointer to 2 got %v", updatedContent.PublishedVersion)
	}
	if updatedContent.CurrentVersion != 3 {
		t.Fatalf("expected current version 3 got %d", updatedContent.CurrentVersion)
	}
}
