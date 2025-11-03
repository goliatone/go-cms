package cms_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/media"
	cmsscheduler "github.com/goliatone/go-cms/internal/scheduler"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestModule_Phase6FeaturesWithBunAndCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)
	registerPhase6Models(t, bunDB)

	localeID := uuid.New()
	typeID := uuid.New()
	seedPhase6Fixtures(t, bunDB, localeID, typeID)

	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en"}
	cfg.Cache.Enabled = true
	cfg.Cache.DefaultTTL = 50 * time.Millisecond
	cfg.Features.Versioning = true
	cfg.Features.Scheduling = true
	cfg.Features.MediaLibrary = true
	cfg.Features.AdvancedCache = true

	mediaProvider := &stubMediaProvider{}
	cacheProvider := newCacheSpy()

	module, err := cms.New(cfg,
		di.WithBunDB(bunDB),
		di.WithMedia(mediaProvider),
		di.WithCacheProvider(cacheProvider),
	)
	if err != nil {
		t.Fatalf("new cms module: %v", err)
	}

	contentSvc := module.Content()
	now := time.Now().UTC()
	authorID := uuid.New()

	contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "phase6-integration",
		Status:        "draft",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Phase 6 integration",
				Content: map[string]any{"body": "initial"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	draft, err := contentSvc.CreateDraft(ctx, content.CreateContentDraftRequest{
		ContentID: contentRecord.ID,
		Snapshot: content.ContentVersionSnapshot{
			Fields: map[string]any{"headline": "Draft headline"},
			Translations: []content.ContentVersionTranslationSnapshot{
				{
					Locale:  "en",
					Title:   "Draft title",
					Content: map[string]any{"body": "draft body"},
				},
			},
		},
		CreatedBy: authorID,
		UpdatedBy: authorID,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if draft.Version != 1 {
		t.Fatalf("expected draft version 1 got %d", draft.Version)
	}

	if _, err := contentSvc.PublishDraft(ctx, content.PublishContentDraftRequest{
		ContentID:   contentRecord.ID,
		Version:     draft.Version,
		PublishedBy: authorID,
		PublishedAt: &now,
	}); err != nil {
		t.Fatalf("publish draft: %v", err)
	}

	publishAt := time.Now().Add(-time.Minute)
	scheduled, err := contentSvc.Schedule(ctx, content.ScheduleContentRequest{
		ContentID:   contentRecord.ID,
		PublishAt:   &publishAt,
		ScheduledBy: authorID,
	})
	if err != nil {
		t.Fatalf("schedule content: %v", err)
	}
	if scheduled.Status != string(domain.StatusScheduled) {
		t.Fatalf("expected scheduled status, got %s", scheduled.Status)
	}

	if _, err := module.Scheduler().GetByKey(ctx, cmsscheduler.ContentPublishJobKey(contentRecord.ID)); err != nil {
		t.Fatalf("expected publish job to be queued: %v", err)
	}

	container := module.Container()
	worker := jobs.NewWorker(module.Scheduler(), container.ContentRepository(), container.PageRepository())
	if err := worker.Process(ctx); err != nil {
		t.Fatalf("process scheduler jobs: %v", err)
	}

	jobAfter, err := module.Scheduler().GetByKey(ctx, cmsscheduler.ContentPublishJobKey(contentRecord.ID))
	if err != nil && !errors.Is(err, interfaces.ErrJobNotFound) {
		t.Fatalf("inspect publish job: %v", err)
	}
	t.Logf("publish job status after worker: %v", statusOrNil(jobAfter))

	rawAfter, err := container.ContentRepository().GetByID(ctx, contentRecord.ID)
	if err != nil {
		t.Fatalf("fetch raw content after worker: %v", err)
	}
	if rawAfter.Status != string(domain.StatusPublished) {
		t.Fatalf("expected published status after processing, got %s", rawAfter.Status)
	}
	if rawAfter.PublishedAt == nil || rawAfter.PublishedAt.IsZero() {
		t.Fatalf("expected published_at stamped after processing")
	}

	if _, err := module.Scheduler().GetByKey(ctx, cmsscheduler.ContentPublishJobKey(contentRecord.ID)); !errors.Is(err, interfaces.ErrJobNotFound) {
		t.Fatalf("expected publish job to be completed, got %v", err)
	}

	after, err := contentSvc.Get(ctx, contentRecord.ID)
	if err != nil {
		t.Fatalf("get content after worker: %v", err)
	}
	if after.Status != string(domain.StatusPublished) {
		t.Fatalf("expected published status via service, got %s", after.Status)
	}

	bindings := media.BindingSet{
		"hero": {
			{
				Slot: "hero",
				Reference: interfaces.MediaReference{
					ID: "asset-123",
				},
				Required: []string{"thumb"},
			},
		},
	}

	resolvedFirst, err := module.Media().ResolveBindings(ctx, bindings, media.ResolveOptions{})
	if err != nil {
		t.Fatalf("resolve bindings first time: %v", err)
	}
	if len(resolvedFirst["hero"]) == 0 {
		t.Fatalf("expected resolved bindings for hero slot")
	}

	if _, err := module.Media().ResolveBindings(ctx, bindings, media.ResolveOptions{}); err != nil {
		t.Fatalf("resolve bindings second time: %v", err)
	}

	if mediaProvider.resolveCalls() != 1 {
		t.Fatalf("expected media provider called once, got %d", mediaProvider.resolveCalls())
	}
	if cacheProvider.gets == 0 || cacheProvider.sets == 0 {
		t.Fatalf("expected cache provider to be used (gets=%d, sets=%d)", cacheProvider.gets, cacheProvider.sets)
	}
}

func TestModuleWidgetsDisabledReturnsNoOp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := cms.DefaultConfig()
	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	widgetSvc := module.Widgets()
	if widgetSvc == nil {
		t.Fatalf("expected widget service instance")
	}
	if _, err := widgetSvc.RegisterDefinition(ctx, widgets.RegisterDefinitionInput{Name: "any"}); !errors.Is(err, widgets.ErrFeatureDisabled) {
		t.Fatalf("expected ErrFeatureDisabled, got %v", err)
	}
	if module.Pages() == nil {
		t.Fatalf("expected page service even when widgets disabled")
	}
}

func TestModuleWidgetsAndThemesEnabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := cms.DefaultConfig()
	cfg.Features.Widgets = true
	cfg.Features.Themes = true

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	definitions, err := module.Widgets().ListDefinitions(ctx)
	if err != nil {
		t.Fatalf("list widget definitions: %v", err)
	}
	if len(definitions) == 0 {
		t.Fatalf("expected built-in widget definition when widgets enabled")
	}

	themes := module.Themes()
	if themes == nil {
		t.Fatalf("expected theme service instance")
	}
	list, err := themes.ListThemes(ctx)
	if err != nil {
		t.Fatalf("list themes with memory repositories: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no themes registered by default, got %d", len(list))
	}
}

func TestModuleContentRetentionLimitEnforced(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := cms.DefaultConfig()
	cfg.Features.Versioning = true
	cfg.Retention.Content = 1

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	typeRepo := module.Container().ContentTypeRepository()
	seeder, ok := typeRepo.(interface{ Put(*content.ContentType) })
	if !ok {
		t.Fatalf("content type repository not seedable")
	}
	contentTypeID := uuid.New()
	seeder.Put(&content.ContentType{ID: contentTypeID, Name: "article"})

	author := uuid.New()
	contentSvc := module.Content()
	created, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "retention-module",
		Status:        string(domain.StatusDraft),
		CreatedBy:     author,
		UpdatedBy:     author,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Retention Module",
				Content: map[string]any{"body": "draft"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	snapshot := content.ContentVersionSnapshot{
		Fields: map[string]any{"headline": "first"},
		Translations: []content.ContentVersionTranslationSnapshot{
			{Locale: "en", Title: "Draft", Content: map[string]any{"body": "draft"}},
		},
	}
	if _, err := contentSvc.CreateDraft(ctx, content.CreateContentDraftRequest{
		ContentID: created.ID,
		Snapshot:  snapshot,
		CreatedBy: author,
	}); err != nil {
		t.Fatalf("first draft: %v", err)
	}
	_, err = contentSvc.CreateDraft(ctx, content.CreateContentDraftRequest{
		ContentID: created.ID,
		Snapshot:  snapshot,
		CreatedBy: author,
	})
	if !errors.Is(err, content.ErrContentVersionRetentionExceeded) {
		t.Fatalf("expected ErrContentVersionRetentionExceeded, got %v", err)
	}
}

func registerPhase6Models(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()
	models := []any{
		(*content.Locale)(nil),
		(*content.ContentType)(nil),
		(*content.Content)(nil),
		(*content.ContentTranslation)(nil),
		(*content.ContentVersion)(nil),
	}
	for _, model := range models {
		if _, err := db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
			t.Fatalf("create table %T: %v", model, err)
		}
	}
}

func seedPhase6Fixtures(t *testing.T, db *bun.DB, localeID, typeID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	locale := &content.Locale{
		ID:        localeID,
		Code:      "en",
		Display:   "English",
		IsActive:  true,
		IsDefault: true,
	}
	if _, err := db.NewInsert().Model(locale).Exec(ctx); err != nil {
		t.Fatalf("insert locale: %v", err)
	}

	ct := &content.ContentType{
		ID:     typeID,
		Name:   "page",
		Schema: map[string]any{"fields": []map[string]any{{"name": "body", "type": "richtext"}}},
	}
	if _, err := db.NewInsert().Model(ct).Exec(ctx); err != nil {
		t.Fatalf("insert content type: %v", err)
	}
}

type stubMediaProvider struct {
	mu    sync.Mutex
	calls int
}

func (s *stubMediaProvider) resolveCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *stubMediaProvider) Resolve(_ context.Context, req interfaces.MediaResolveRequest) (*interfaces.MediaAsset, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	asset := &interfaces.MediaAsset{
		Reference: req.Reference,
		Source: &interfaces.MediaResource{
			URL: "https://cdn.example.com/" + req.Reference.ID,
		},
		Renditions: map[string]*interfaces.MediaResource{
			"thumb": {
				URL: "https://cdn.example.com/" + req.Reference.ID + "/thumb",
			},
		},
		Metadata: interfaces.MediaMetadata{
			ID:   req.Reference.ID,
			Name: "Example asset",
		},
	}
	return asset, nil
}

func (s *stubMediaProvider) ResolveBatch(ctx context.Context, reqs []interfaces.MediaResolveRequest) (map[string]*interfaces.MediaAsset, error) {
	result := make(map[string]*interfaces.MediaAsset, len(reqs))
	for _, req := range reqs {
		asset, err := s.Resolve(ctx, req)
		if err != nil {
			return nil, err
		}
		key := req.Reference.ID
		if key == "" {
			key = req.Reference.Path
		}
		result[key] = asset
	}
	return result, nil
}

func (s *stubMediaProvider) Invalidate(context.Context, ...interfaces.MediaReference) error {
	return nil
}

type cacheSpy struct {
	mu    sync.Mutex
	data  map[string]any
	gets  int
	sets  int
	drops int
}

func newCacheSpy() *cacheSpy {
	return &cacheSpy{
		data: make(map[string]any),
	}
}

func statusOrNil(job *interfaces.Job) string {
	if job == nil {
		return "<nil>"
	}
	return string(job.Status)
}

func (c *cacheSpy) Get(_ context.Context, key string) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gets++
	return c.data[key], nil
}

func (c *cacheSpy) Set(_ context.Context, key string, value any, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sets++
	c.data[key] = value
	return nil
}

func (c *cacheSpy) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	c.drops++
	return nil
}

func (c *cacheSpy) Clear(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]any)
	c.drops++
	return nil
}
