package generator_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	ditesting "github.com/goliatone/go-cms/internal/di/testing"
	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/identity"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

const (
	storageOpWrite = "generator.write"
	storageOpRead  = "generator.read"
)

func TestIntegrationBuildWithMemoryRepositories(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 12, 1, 9, 0, 0, 0, time.UTC)

	cfg := runtimeconfig.DefaultConfig()
	cfg.Cache.Enabled = false
	cfg.Features.Themes = true
	cfg.Generator.Enabled = true
	cfg.Generator.OutputDir = "dist"
	cfg.Generator.BaseURL = "https://example.test"
	cfg.Generator.GenerateSitemap = true
	cfg.Generator.GenerateRobots = true
	cfg.Generator.GenerateFeeds = true
	cfg.Generator.Menus = map[string]string{}
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en", "es"}
	cfg.Generator.CopyAssets = false
	cfg.Themes.DefaultTheme = "aurora"
	cfg.Themes.DefaultVariant = "contrast"

	renderer := &integrationRenderer{}
	container, memStorage, err := ditesting.NewGeneratorContainer(cfg, di.WithTemplate(renderer))
	if err != nil {
		t.Fatalf("build container: %v", err)
	}

	themeSvc := container.ThemeService().(themes.Service)
	template, _ := registerThemeFixtures(t, ctx, themeSvc)

	contentRepo := container.ContentRepository()
	contentTypeRepo := container.ContentTypeRepository()
	localeRepo := container.LocaleRepository()

	enLocale, err := localeRepo.GetByCode(ctx, "en")
	if err != nil {
		t.Fatalf("lookup en locale: %v", err)
	}
	esLocale, err := localeRepo.GetByCode(ctx, "es")
	if err != nil {
		t.Fatalf("lookup es locale: %v", err)
	}

	envID := cmsenv.IDForKey(cmsenv.DefaultKey)
	contentTypeID := uuid.New()
	if contentTypeRepo == nil {
		t.Fatal("content type repository is nil")
	}
	_, err = contentTypeRepo.Create(ctx, &content.ContentType{
		ID:            contentTypeID,
		Name:          "page",
		Slug:          "page",
		Status:        content.ContentTypeStatusActive,
		EnvironmentID: envID,
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	contentID := uuid.New()
	contentRecord := &content.Content{
		ID:             contentID,
		ContentTypeID:  contentTypeID,
		CurrentVersion: 1,
		Status:         "published",
		Slug:           "company",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		CreatedAt:      now,
		UpdatedAt:      now,
		EnvironmentID:  envID,
		Metadata: map[string]any{
			"template_id": template.ID.String(),
		},
		IsVisible: true,
		Translations: []*content.ContentTranslation{
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  enLocale.ID,
				Title:     "Company",
				Content: map[string]any{
					"body": "english",
					"path": "/company",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  esLocale.ID,
				Title:     "Empresa",
				Content: map[string]any{
					"body": "espanol",
					"path": "/es/empresa",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	if _, err := contentRepo.Create(ctx, contentRecord); err != nil {
		t.Fatalf("create content: %v", err)
	}

	svc := container.GeneratorService()
	result, err := svc.Build(ctx, generator.BuildOptions{})
	if err != nil {
		t.Fatalf("integration build: %v", err)
	}
	if result == nil {
		t.Fatalf("expected result")
	}
	if result.PagesBuilt != 2 {
		t.Fatalf("expected 2 pages built, got %d", result.PagesBuilt)
	}
	if len(result.Diagnostics) != 2 {
		t.Fatalf("expected diagnostics for two pages, got %d", len(result.Diagnostics))
	}
	if result.Duration == 0 {
		t.Fatalf("expected non-zero duration")
	}
	if result.Metrics.RenderDuration == 0 {
		t.Fatalf("expected render metrics recorded")
	}
	if result.FeedsBuilt == 0 {
		t.Fatalf("expected feed artifacts to be generated")
	}

	writes := memStorage.ExecCalls()
	pageWrites := 0
	expectedFeeds := map[string]struct{}{
		path.Join(cfg.Generator.OutputDir, "feeds/en.rss.xml"):  {},
		path.Join(cfg.Generator.OutputDir, "feeds/en.atom.xml"): {},
		path.Join(cfg.Generator.OutputDir, "feeds/es.rss.xml"):  {},
		path.Join(cfg.Generator.OutputDir, "feeds/es.atom.xml"): {},
		path.Join(cfg.Generator.OutputDir, "feed.xml"):          {},
		path.Join(cfg.Generator.OutputDir, "feed.atom.xml"):     {},
	}
	var sitemapWritten bool
	for _, call := range writes {
		if call.Query != "generator.write" {
			continue
		}
		if len(call.Args) == 0 {
			continue
		}
		target, ok := call.Args[0].(string)
		if !ok {
			continue
		}
		if strings.HasSuffix(target, "index.html") {
			pageWrites++
		}
		category, _ := call.Args[3].(string)
		if category == "sitemap" && strings.HasSuffix(target, "sitemap.xml") {
			sitemapWritten = true
		}
		if category == "feed" {
			if _, exists := expectedFeeds[target]; exists {
				delete(expectedFeeds, target)
			}
		}
	}
	if pageWrites != 2 {
		t.Fatalf("expected 2 page writes, got %d", pageWrites)
	}
	if !sitemapWritten {
		t.Fatalf("expected sitemap artifact to be written")
	}
	if len(expectedFeeds) != 0 {
		t.Fatalf("missing feed writes: %v", expectedFeeds)
	}
}

func TestIntegrationBuildFeedsIncrementalWithSQLite(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 11, 20, 12, 0, 0, 0, time.UTC)

	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)
	registerGeneratorModels(t, bunDB)

	cfg := runtimeconfig.DefaultConfig()
	cfg.Cache.Enabled = false
	cfg.Features.Themes = true
	cfg.Generator.Enabled = true
	cfg.Generator.OutputDir = "dist"
	cfg.Generator.BaseURL = "https://example.test"
	cfg.Generator.GenerateSitemap = true
	cfg.Generator.GenerateFeeds = true
	cfg.Generator.GenerateRobots = false
	cfg.Generator.CopyAssets = false
	cfg.Generator.Incremental = true
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en", "es"}

	renderer := &integrationRenderer{}

	baseContainer, err := di.NewContainer(cfg, di.WithBunDB(bunDB), di.WithTemplate(renderer))
	if err != nil {
		t.Fatalf("build base container: %v", err)
	}
	contentWrapper := newHydratingContentService(baseContainer.ContentService(), bunDB)

	storage := newRecordingStorage()

	container, _, err := ditesting.NewGeneratorContainer(cfg,
		di.WithBunDB(bunDB),
		di.WithTemplate(renderer),
		di.WithGeneratorStorage(storage),
		di.WithContentService(contentWrapper),
	)
	if err != nil {
		t.Fatalf("build container: %v", err)
	}

	enLocaleID := identity.LocaleUUID("en")
	esLocaleID := identity.LocaleUUID("es")

	envID := cmsenv.IDForKey(cmsenv.DefaultKey)
	contentTypeID := uuid.New()
	contentType := &content.ContentType{
		ID:            contentTypeID,
		Name:          "page",
		Slug:          "page",
		Status:        content.ContentTypeStatusActive,
		Schema:        map[string]any{"fields": []map[string]any{{"name": "body", "type": "richtext"}}},
		CreatedAt:     now,
		UpdatedAt:     now,
		EnvironmentID: envID,
	}
	if _, err := bunDB.NewInsert().Model(contentType).Exec(ctx); err != nil {
		t.Fatalf("insert content type: %v", err)
	}

	themeSvc := container.ThemeService().(themes.Service)
	template, _ := registerThemeFixtures(t, ctx, themeSvc)

	authorID := uuid.New()

	publishedAt := now.Add(-6 * time.Hour)
	contentID := uuid.New()
	contentRecord := &content.Content{
		ID:             contentID,
		ContentTypeID:  contentTypeID,
		CurrentVersion: 1,
		Status:         "published",
		Slug:           "news",
		PublishedAt:    &publishedAt,
		CreatedBy:      authorID,
		UpdatedBy:      authorID,
		CreatedAt:      now,
		UpdatedAt:      now,
		EnvironmentID:  envID,
		Metadata: map[string]any{
			"template_id": template.ID.String(),
		},
		IsVisible: true,
		Translations: []*content.ContentTranslation{
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  enLocaleID,
				Title:     "News",
				Summary:   strPtr("Latest company news"),
				Content: map[string]any{
					"body": "english body",
					"path": "/news",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  esLocaleID,
				Title:     "Noticias",
				Summary:   strPtr("Últimas noticias"),
				Content: map[string]any{
					"body": "spanish body",
					"path": "/es/noticias",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	if _, err := bunDB.NewInsert().Model(contentRecord).Exec(ctx); err != nil {
		t.Fatalf("insert content: %v", err)
	}
	for _, tr := range contentRecord.Translations {
		if _, err := bunDB.NewInsert().Model(tr).Exec(ctx); err != nil {
			t.Fatalf("insert content translation: %v", err)
		}
	}

	svc := container.GeneratorService()

	buildOpts := generator.BuildOptions{
		PageIDs: []uuid.UUID{contentID},
	}

	firstResult, err := svc.Build(ctx, buildOpts)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	if firstResult.FeedsBuilt == 0 {
		t.Fatalf("expected feeds generated on first build; pages_built=%d errors=%d diagnostics=%d", firstResult.PagesBuilt, len(firstResult.Errors), len(firstResult.Diagnostics))
	}

	initialCalls := len(storage.ExecCalls())

	secondResult, err := svc.Build(ctx, buildOpts)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if secondResult.FeedsBuilt == 0 {
		t.Fatalf("expected feeds generated on incremental build")
	}

	newCalls := storage.ExecCalls()[initialCalls:]
	pageWrites := 0
	feedWrites := 0
	sitemapWrites := 0
	for _, call := range newCalls {
		if call.Query != "generator.write" || len(call.Args) < 4 {
			continue
		}
		target, _ := call.Args[0].(string)
		category, _ := call.Args[3].(string)
		if category == "page" && strings.HasSuffix(target, "index.html") {
			pageWrites++
		}
		if category == "feed" {
			feedWrites++
		}
		if category == "sitemap" {
			sitemapWrites++
		}
	}
	if pageWrites != 0 {
		t.Fatalf("expected no page writes on incremental build, got %d", pageWrites)
	}
	if feedWrites == 0 {
		t.Fatalf("expected feed rewrites on incremental build")
	}
	if sitemapWrites == 0 {
		t.Fatalf("expected sitemap rewrite on incremental build")
	}
}

func registerThemeFixtures(t *testing.T, ctx context.Context, svc themes.Service) (*themes.Template, *themes.Theme) {
	t.Helper()
	theme, err := svc.RegisterTheme(ctx, themes.RegisterThemeInput{
		Name:      "aurora",
		Version:   "1.0.0",
		ThemePath: "testdata/theme",
	})
	if err != nil {
		t.Fatalf("register theme: %v", err)
	}
	tmpl, err := svc.RegisterTemplate(ctx, themes.RegisterTemplateInput{
		ThemeID:      theme.ID,
		Name:         "page",
		Slug:         "page",
		TemplatePath: "themes/aurora/page.html",
		Regions: map[string]themes.TemplateRegion{
			"main": {
				Name:          "Main",
				AcceptsBlocks: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("register template: %v", err)
	}
	return tmpl, theme
}

func registerGeneratorModels(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()

	models := []any{
		(*content.Locale)(nil),
		(*content.ContentType)(nil),
		(*content.Content)(nil),
		(*content.ContentTranslation)(nil),
		(*content.ContentVersion)(nil),
		(*themes.Theme)(nil),
		(*themes.Template)(nil),
	}

	for _, model := range models {
		if _, err := db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
			t.Fatalf("create table %T: %v", model, err)
		}
	}

	registerBlockTables(t, db)
}

func registerBlockTables(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS block_definitions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT,
			description TEXT,
			icon TEXT,
			category TEXT,
			status TEXT,
			schema TEXT,
			ui_schema TEXT,
			schema_version TEXT,
			defaults TEXT,
			editor_style_url TEXT,
			frontend_style_url TEXT,
			environment_id TEXT,
			deleted_at TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS block_definition_versions (
			id TEXT PRIMARY KEY,
			definition_id TEXT NOT NULL,
			schema_version TEXT NOT NULL,
			schema TEXT NOT NULL,
			defaults TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS block_instances (
			id TEXT PRIMARY KEY,
			page_id TEXT,
			region TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			definition_id TEXT NOT NULL,
			configuration TEXT,
			is_global BOOLEAN DEFAULT FALSE,
			current_version INTEGER NOT NULL DEFAULT 1,
			published_version INTEGER,
			published_at TEXT,
			published_by TEXT,
			created_by TEXT NOT NULL,
			updated_by TEXT NOT NULL,
			deleted_at TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS block_versions (
			id TEXT PRIMARY KEY,
			block_instance_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			status TEXT NOT NULL,
			snapshot TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at TEXT,
			published_at TEXT,
			published_by TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS block_translations (
			id TEXT PRIMARY KEY,
			block_instance_id TEXT NOT NULL,
			locale_id TEXT NOT NULL,
			content TEXT,
			media_bindings TEXT,
			attribute_overrides TEXT,
			deleted_at TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("create block table: %v", err)
		}
	}
}

type recordingStorage struct {
	mu        sync.Mutex
	files     map[string][]byte
	execCalls []ditesting.ExecCall
}

func newRecordingStorage() *recordingStorage {
	return &recordingStorage{
		files: make(map[string][]byte),
	}
}

func (s *recordingStorage) Exec(ctx context.Context, query string, args ...any) (interfaces.Result, error) {
	isWrite := query == storageOpWrite
	if isWrite && len(args) >= 1 {
		path, _ := args[0].(string)
		if reader, ok := args[1].(io.Reader); ok && reader != nil {
			data, err := io.ReadAll(reader)
			if err != nil {
				return nil, err
			}
			args[1] = bytes.NewReader(data)
			if path != "" {
				s.mu.Lock()
				s.files[path] = data
				s.mu.Unlock()
			}
		}
	}
	s.recordExec(query, args, false)
	return recordingResult{}, nil
}

func (s *recordingStorage) Query(ctx context.Context, query string, args ...any) (interfaces.Rows, error) {
	if query == storageOpRead && len(args) > 0 {
		path, _ := args[0].(string)
		if data := s.lookup(path); data != nil {
			return &byteRows{data: data}, nil
		}
	}
	return &byteRows{}, nil
}

func (s *recordingStorage) Transaction(ctx context.Context, fn func(interfaces.Transaction) error) error {
	if fn == nil {
		return nil
	}
	tx := &recordingTx{storage: s}
	if err := fn(tx); err != nil {
		tx.rollback = true
		return err
	}
	tx.commit = true
	return nil
}

func (s *recordingStorage) ExecCalls() []ditesting.ExecCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	calls := make([]ditesting.ExecCall, len(s.execCalls))
	copy(calls, s.execCalls)
	return calls
}

func (s *recordingStorage) recordExec(query string, args []any, inTx bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	clonedArgs := append([]any(nil), args...)
	s.execCalls = append(s.execCalls, ditesting.ExecCall{
		Query:         query,
		Args:          clonedArgs,
		InTransaction: inTx,
	})
}

func (s *recordingStorage) lookup(path string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.files[path]
	if !ok {
		return nil
	}
	copied := make([]byte, len(data))
	copy(copied, data)
	return copied
}

type recordingTx struct {
	storage  *recordingStorage
	commit   bool
	rollback bool
}

func (tx *recordingTx) Exec(ctx context.Context, query string, args ...any) (interfaces.Result, error) {
	if tx.storage == nil {
		return nil, errors.New("recording tx: storage not configured")
	}
	if query == storageOpWrite && len(args) >= 1 {
		path, _ := args[0].(string)
		if reader, ok := args[1].(io.Reader); ok && reader != nil {
			data, err := io.ReadAll(reader)
			if err != nil {
				return nil, err
			}
			args[1] = bytes.NewReader(data)
			if path != "" {
				tx.storage.mu.Lock()
				tx.storage.files[path] = data
				tx.storage.mu.Unlock()
			}
		}
	}
	tx.storage.recordExec(query, args, true)
	return recordingResult{}, nil
}

func (tx *recordingTx) Query(ctx context.Context, query string, args ...any) (interfaces.Rows, error) {
	if tx.storage == nil {
		return &byteRows{}, errors.New("recording tx: storage not configured")
	}
	return tx.storage.Query(ctx, query, args...)
}

func (tx *recordingTx) Transaction(context.Context, func(interfaces.Transaction) error) error {
	return errors.New("recording tx: nested transactions not supported")
}

func (tx *recordingTx) Commit() error {
	tx.commit = true
	return nil
}

func (tx *recordingTx) Rollback() error {
	tx.rollback = true
	return nil
}

type recordingResult struct{}

func (recordingResult) RowsAffected() (int64, error) { return 0, nil }

func (recordingResult) LastInsertId() (int64, error) { return 0, nil }

type byteRows struct {
	data []byte
	read bool
}

func (r *byteRows) Next() bool {
	if r == nil || r.read {
		return false
	}
	r.read = true
	return true
}

func (r *byteRows) Scan(dest ...any) error {
	if r == nil || !r.read {
		return errors.New("recording rows: call Next before Scan")
	}
	if len(dest) == 0 {
		return nil
	}
	switch target := dest[0].(type) {
	case *[]byte:
		*target = append((*target)[:0], r.data...)
	case *string:
		*target = string(r.data)
	default:
		return fmt.Errorf("recording rows: unsupported scan destination %T", dest[0])
	}
	return nil
}

func (r *byteRows) Close() error { return nil }

type hydratingContentService struct {
	content.Service
	db *bun.DB
}

func newHydratingContentService(delegate content.Service, db *bun.DB) content.Service {
	if delegate == nil || db == nil {
		return delegate
	}
	return &hydratingContentService{Service: delegate, db: db}
}

func (s *hydratingContentService) Get(ctx context.Context, id uuid.UUID, opts ...content.ContentGetOption) (*content.Content, error) {
	record, err := s.Service.Get(ctx, id, opts...)
	if err != nil || record == nil {
		return record, err
	}
	return s.hydrate(ctx, record)
}

func (s *hydratingContentService) hydrate(ctx context.Context, record *content.Content) (*content.Content, error) {
	if record == nil || s.db == nil || record.ID == uuid.Nil {
		return record, nil
	}
	var translations []*content.ContentTranslation
	if err := s.db.NewSelect().
		Model(&translations).
		Where("content_id = ?", record.ID).
		Scan(ctx); err != nil {
		return nil, err
	}
	if len(translations) == 0 {
		return record, nil
	}
	cloned := *record
	cloned.Translations = make([]*content.ContentTranslation, len(translations))
	for i, tr := range translations {
		if tr == nil {
			continue
		}
		copyTr := *tr
		cloned.Translations[i] = &copyTr
	}
	return &cloned, nil
}

func strPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	v := value
	return &v
}

type integrationRenderer struct{}

func (integrationRenderer) Render(name string, data any, out ...io.Writer) (string, error) {
	return integrationRenderer{}.RenderTemplate(name, data, out...)
}

func (integrationRenderer) RenderTemplate(name string, data any, out ...io.Writer) (string, error) {
	ctx, ok := data.(generator.TemplateContext)
	if !ok {
		return "", fmt.Errorf("unexpected template context %T", data)
	}
	return fmt.Sprintf("<html><body>%s-%s</body></html>", name, ctx.Page.Locale.Code), nil
}

func (integrationRenderer) RenderString(templateContent string, data any, out ...io.Writer) (string, error) {
	return integrationRenderer{}.RenderTemplate(templateContent, data, out...)
}

func (integrationRenderer) RegisterFilter(string, func(any, any) (any, error)) error {
	return nil
}

func (integrationRenderer) GlobalContext(any) error { return nil }
