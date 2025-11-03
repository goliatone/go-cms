package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	ditesting "github.com/goliatone/go-cms/internal/di/testing"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/scheduler"
	"github.com/goliatone/go-cms/internal/themes"
	workflowsimple "github.com/goliatone/go-cms/internal/workflow/simple"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
)

type workflowStateFixture struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Terminal    bool   `json:"terminal"`
	Initial     bool   `json:"initial"`
}

type workflowTransitionFixture struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	From        string `json:"from"`
	To          string `json:"to"`
	Guard       string `json:"guard"`
}

type workflowGeneratorFixture struct {
	Definition struct {
		Entity      string                      `json:"entity"`
		States      []workflowStateFixture      `json:"states"`
		Transitions []workflowTransitionFixture `json:"transitions"`
	} `json:"definition"`
	Page struct {
		Slug  string `json:"slug"`
		Path  string `json:"path"`
		Title string `json:"title"`
	} `json:"page"`
	Transitions []struct {
		Status string `json:"status"`
	} `json:"transitions"`
	Expectations struct {
		InitialStatus string `json:"initial_status"`
		Route         string `json:"route"`
	} `json:"expectations"`
}

func TestWorkflowIntegration_GeneratorPropagatesStatuses(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	data, err := testsupport.LoadFixture(filepath.Join("testdata", "workflow_generator.json"))
	if err != nil {
		t.Fatalf("load workflow generator fixture: %v", err)
	}

	var fixture workflowGeneratorFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("unmarshal workflow generator fixture: %v", err)
	}

	cfg := cms.DefaultConfig()
	cfg.Features.Themes = true
	cfg.Features.Versioning = true
	cfg.Generator.Enabled = true
	cfg.Generator.GenerateSitemap = false
	cfg.Generator.GenerateRobots = false
	cfg.Generator.GenerateFeeds = false
	cfg.Generator.OutputDir = "dist"
	cfg.Generator.BaseURL = "https://example.test"
	cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{
		{
			Entity:      fixture.Definition.Entity,
			States:      convertFixtureStates(fixture.Definition.States),
			Transitions: convertFixtureTransitions(fixture.Definition.Transitions),
		},
	}

	renderer := integrationRenderer{}
	container, _, err := ditesting.NewGeneratorContainer(cfg, di.WithTemplate(renderer))
	if err != nil {
		t.Fatalf("new generator container: %v", err)
	}

	themeSvc := container.ThemeService().(themes.Service)
	template, _ := registerThemeFixtures(t, ctx, themeSvc)

	typeRepo := container.ContentTypeRepository()
	seedTypes, ok := typeRepo.(interface{ Put(*content.ContentType) })
	if !ok {
		t.Fatalf("expected seedable content type repository, got %T", typeRepo)
	}
	contentTypeID := uuid.New()
	seedTypes.Put(&content.ContentType{ID: contentTypeID, Name: "workflow"})

	localeRepo := container.LocaleRepository()
	seedLocales, ok := localeRepo.(interface{ Put(*content.Locale) })
	if !ok {
		t.Fatalf("expected seedable locale repository, got %T", localeRepo)
	}
	enLocaleID := uuid.New()
	seedLocales.Put(&content.Locale{ID: enLocaleID, Code: "en", Display: "English"})

	contentSvc := container.ContentService()
	authorID := uuid.New()
	contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          fixture.Page.Slug,
		Status:        string(domain.StatusDraft),
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: fixture.Page.Title},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := container.PageService()
	translations := []pages.PageTranslationInput{
		{Locale: "en", Title: fixture.Page.Title, Path: fixture.Page.Path},
	}
	page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:    contentRecord.ID,
		TemplateID:   template.ID,
		Slug:         fixture.Page.Slug,
		Status:       string(domain.StatusDraft),
		CreatedBy:    authorID,
		UpdatedBy:    authorID,
		Translations: append([]pages.PageTranslationInput(nil), translations...),
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	initialState := deriveInitialState(fixture.Definition.States)
	if !stringsEqualFold(page.Status, initialState) {
		t.Fatalf("initial status mismatch: want %q got %q", initialState, page.Status)
	}

	genSvc := container.GeneratorService()
	if genSvc == nil {
		t.Fatalf("expected generator service to be configured")
	}

	resultBefore, err := genSvc.Build(ctx, generator.BuildOptions{Locales: []string{"en"}})
	if err != nil {
		t.Fatalf("initial build: %v", err)
	}
	if resultBefore.PagesBuilt != 0 {
		t.Fatalf("expected no pages built before publish, got %d", resultBefore.PagesBuilt)
	}

	for idx, step := range fixture.Transitions {
		page, err = pageSvc.Update(ctx, pages.UpdatePageRequest{
			ID:           page.ID,
			Status:       step.Status,
			UpdatedBy:    authorID,
			Translations: append([]pages.PageTranslationInput(nil), translations...),
		})
		if err != nil {
			t.Fatalf("transition %d to %q failed: %v", idx, step.Status, err)
		}
	}

	finalStatus := fixture.Transitions[len(fixture.Transitions)-1].Status
	if !stringsEqualFold(page.Status, finalStatus) {
		t.Fatalf("final status mismatch: want %q got %q", finalStatus, page.Status)
	}

	resultAfter, err := genSvc.Build(ctx, generator.BuildOptions{Locales: []string{"en"}})
	if err != nil {
		t.Fatalf("published build: %v", err)
	}
	if resultAfter.PagesBuilt != 1 {
		t.Fatalf("expected one page built after publish, got %d", resultAfter.PagesBuilt)
	}
	if len(resultAfter.Rendered) == 0 {
		t.Fatalf("expected rendered page metadata")
	}
	rendered := resultAfter.Rendered[0]
	if rendered.Route != fixture.Expectations.Route {
		t.Fatalf("route mismatch: want %q got %q", fixture.Expectations.Route, rendered.Route)
	}
	pageSource := rendered.Metadata.Sources["page"]
	if !strings.Contains(pageSource, "published") {
		t.Fatalf("page source metadata should include published state, got %q", pageSource)
	}
}

type workflowScheduleFixture struct {
	PublishAt   string `json:"publish_at"`
	UnpublishAt string `json:"unpublish_at"`
	ScheduledBy string `json:"scheduled_by"`
	Page        struct {
		ID         string `json:"id"`
		ContentID  string `json:"content_id"`
		TemplateID string `json:"template_id"`
		Slug       string `json:"slug"`
		Status     string `json:"status"`
		Path       string `json:"path"`
		Title      string `json:"title"`
	} `json:"page"`
	Golden string `json:"golden"`
}

func TestWorkflowIntegration_SchedulingLogsDeterministic(t *testing.T) {
	t.Helper()

	data, err := testsupport.LoadFixture(filepath.Join("testdata", "workflow_schedule_fixture.json"))
	if err != nil {
		t.Fatalf("load workflow schedule fixture: %v", err)
	}
	var fixture workflowScheduleFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("unmarshal workflow schedule fixture: %v", err)
	}

	publishAt, err := time.Parse(time.RFC3339, fixture.PublishAt)
	if err != nil {
		t.Fatalf("parse publish_at: %v", err)
	}
	unpublishAt, err := time.Parse(time.RFC3339, fixture.UnpublishAt)
	if err != nil {
		t.Fatalf("parse unpublish_at: %v", err)
	}
	scheduledBy, err := uuid.Parse(fixture.ScheduledBy)
	if err != nil {
		t.Fatalf("parse scheduled_by: %v", err)
	}
	pageID, err := uuid.Parse(fixture.Page.ID)
	if err != nil {
		t.Fatalf("parse page id: %v", err)
	}
	contentID, err := uuid.Parse(fixture.Page.ContentID)
	if err != nil {
		t.Fatalf("parse content id: %v", err)
	}
	templateID, err := uuid.Parse(fixture.Page.TemplateID)
	if err != nil {
		t.Fatalf("parse template id: %v", err)
	}

	workflowClock := func() time.Time {
		return publishAt
	}

	engine := newDeterministicWorkflowEngine(workflowClock)

	rec := newIntegrationRecordingProvider()
	cfg := cms.DefaultConfig()
	cfg.Features.Logger = true
	cfg.Features.Versioning = true
	cfg.Features.Scheduling = true
	cfg.Workflow.Provider = "custom"

	container, err := di.NewContainer(cfg,
		di.WithWorkflowEngine(engine),
		di.WithScheduler(scheduler.NewInMemory(scheduler.WithClock(workflowClock))),
		di.WithLoggerProvider(rec),
	)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	localeRepo := container.LocaleRepository()
	seedLocales, ok := localeRepo.(interface{ Put(*content.Locale) })
	if !ok {
		t.Fatalf("expected seedable locale repository, got %T", localeRepo)
	}
	enLocaleID := uuid.New()
	seedLocales.Put(&content.Locale{ID: enLocaleID, Code: "en", Display: "English"})

	pageRepo := container.PageRepository()

	now := workflowClock()
	record := &pages.Page{
		ID:         pageID,
		ContentID:  contentID,
		TemplateID: templateID,
		Slug:       fixture.Page.Slug,
		Status:     fixture.Page.Status,
		CreatedBy:  scheduledBy,
		UpdatedBy:  scheduledBy,
		CreatedAt:  now,
		UpdatedAt:  now,
		Translations: []*pages.PageTranslation{
			{
				ID:        uuid.New(),
				PageID:    pageID,
				LocaleID:  enLocaleID,
				Title:     fixture.Page.Title,
				Path:      fixture.Page.Path,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	if _, err := pageRepo.Create(context.Background(), record); err != nil {
		t.Fatalf("seed page: %v", err)
	}

	pageSvc := container.PageService()
	updated, err := pageSvc.Schedule(context.Background(), pages.SchedulePageRequest{
		PageID:      pageID,
		PublishAt:   &publishAt,
		UnpublishAt: &unpublishAt,
		ScheduledBy: scheduledBy,
	})
	if err != nil {
		t.Fatalf("schedule page: %v", err)
	}
	if !stringsEqualFold(updated.Status, string(domain.WorkflowStateScheduled)) {
		t.Fatalf("expected scheduled status, got %q", updated.Status)
	}

	rawEntries := rec.Entries()
	var captured []recordedEntry
	allowedMessages := map[string]struct{}{
		"workflow event emitted":      {},
		"page publish job enqueued":   {},
		"page unpublish job enqueued": {},
		"page schedule updated":       {},
	}
	for _, entry := range rawEntries {
		if _, ok := allowedMessages[entry.msg]; !ok {
			continue
		}
		captured = append(captured, entry)
	}
	if len(captured) == 0 {
		t.Fatalf("no schedule logs captured; recorded entries: %+v", rawEntries)
	}

	got := normalizeLogEntries(captured)
	goldenPath := filepath.Join("testdata", fixture.Golden)
	var want []logGoldenEntry
	if err := testsupport.LoadGolden(goldenPath, &want); err != nil {
		t.Fatalf("load golden logs: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		wantJSON, _ := json.MarshalIndent(want, "", "  ")
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		t.Fatalf("log entries mismatch\nwant: %s\n got: %s", wantJSON, gotJSON)
	}
}

func convertFixtureStates(states []workflowStateFixture) []cms.WorkflowStateConfig {
	result := make([]cms.WorkflowStateConfig, len(states))
	for i, state := range states {
		result[i] = cms.WorkflowStateConfig{
			Name:        state.Name,
			Description: state.Description,
			Terminal:    state.Terminal,
			Initial:     state.Initial,
		}
	}
	return result
}

func convertFixtureTransitions(transitions []workflowTransitionFixture) []cms.WorkflowTransitionConfig {
	result := make([]cms.WorkflowTransitionConfig, len(transitions))
	for i, transition := range transitions {
		result[i] = cms.WorkflowTransitionConfig{
			Name:        transition.Name,
			Description: transition.Description,
			From:        transition.From,
			To:          transition.To,
			Guard:       transition.Guard,
		}
	}
	return result
}

func deriveInitialState(states []workflowStateFixture) string {
	for _, state := range states {
		if state.Initial {
			return state.Name
		}
	}
	if len(states) == 0 {
		return string(domain.WorkflowStateDraft)
	}
	return states[0].Name
}

func stringsEqualFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
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

func registerThemeFixtures(t *testing.T, ctx context.Context, svc themes.Service) (*themes.Template, *themes.Theme) {
	t.Helper()
	theme, err := svc.RegisterTheme(ctx, themes.RegisterThemeInput{
		Name:      "aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
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

type deterministicWorkflowEngine struct {
	inner interfaces.WorkflowEngine
	clock func() time.Time
}

func newDeterministicWorkflowEngine(clock func() time.Time) *deterministicWorkflowEngine {
	if clock == nil {
		clock = time.Now
	}
	return &deterministicWorkflowEngine{
		inner: workflowsimple.New(workflowsimple.WithClock(clock)),
		clock: clock,
	}
}

func (e *deterministicWorkflowEngine) Transition(ctx context.Context, input interfaces.TransitionInput) (*interfaces.TransitionResult, error) {
	result, err := e.inner.Transition(ctx, input)
	if err != nil || result == nil {
		return result, err
	}
	if domain.NormalizeWorkflowState(string(result.ToState)) == domain.WorkflowStateScheduled {
		payload := map[string]any{}
		if input.ActorID != uuid.Nil {
			payload["scheduled_by"] = input.ActorID.String()
		}
		result.Events = append(result.Events, interfaces.WorkflowEvent{
			Name:      "page.scheduled",
			Timestamp: e.clock(),
			Payload:   payload,
		})
	}
	return result, nil
}

func (e *deterministicWorkflowEngine) AvailableTransitions(ctx context.Context, query interfaces.TransitionQuery) ([]interfaces.WorkflowTransition, error) {
	return e.inner.AvailableTransitions(ctx, query)
}

func (e *deterministicWorkflowEngine) RegisterWorkflow(ctx context.Context, definition interfaces.WorkflowDefinition) error {
	return e.inner.RegisterWorkflow(ctx, definition)
}

type logGoldenEntry struct {
	Level  string         `json:"level"`
	Msg    string         `json:"msg"`
	Fields map[string]any `json:"fields"`
}

type recordedEntry struct {
	level  string
	msg    string
	fields map[string]any
}

type integrationRecordingProvider struct {
	entries []recordedEntry
}

func newIntegrationRecordingProvider() *integrationRecordingProvider {
	return &integrationRecordingProvider{entries: []recordedEntry{}}
}

func (p *integrationRecordingProvider) GetLogger(name string) interfaces.Logger {
	return &integrationRecordingLogger{
		provider: p,
		fields: map[string]any{
			"logger": name,
		},
	}
}

func (p *integrationRecordingProvider) record(entry recordedEntry) {
	p.entries = append(p.entries, entry)
}

func (p *integrationRecordingProvider) Entries() []recordedEntry {
	out := make([]recordedEntry, len(p.entries))
	copy(out, p.entries)
	return out
}

type integrationRecordingLogger struct {
	provider *integrationRecordingProvider
	fields   map[string]any
}

var _ interfaces.Logger = (*integrationRecordingLogger)(nil)

func (l *integrationRecordingLogger) Trace(msg string, args ...any) { l.log("TRACE", msg, args...) }
func (l *integrationRecordingLogger) Debug(msg string, args ...any) { l.log("DEBUG", msg, args...) }
func (l *integrationRecordingLogger) Info(msg string, args ...any)  { l.log("INFO", msg, args...) }
func (l *integrationRecordingLogger) Warn(msg string, args ...any)  { l.log("WARN", msg, args...) }
func (l *integrationRecordingLogger) Error(msg string, args ...any) { l.log("ERROR", msg, args...) }
func (l *integrationRecordingLogger) Fatal(msg string, args ...any) { l.log("FATAL", msg, args...) }

func (l *integrationRecordingLogger) WithFields(fields map[string]any) interfaces.Logger {
	if len(fields) == 0 {
		return l
	}
	merged := cloneFields(l.fields)
	for key, value := range fields {
		merged[key] = value
	}
	return &integrationRecordingLogger{
		provider: l.provider,
		fields:   merged,
	}
}

func (l *integrationRecordingLogger) WithContext(context.Context) interfaces.Logger {
	return &integrationRecordingLogger{
		provider: l.provider,
		fields:   cloneFields(l.fields),
	}
}

func (l *integrationRecordingLogger) log(level, msg string, args ...any) {
	fields := cloneFields(l.fields)
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok || key == "" {
			continue
		}
		fields[key] = args[i+1]
	}
	if _, exists := fields["module"]; !exists {
		fields["module"] = "cms.pages"
	}
	l.provider.record(recordedEntry{
		level:  level,
		msg:    msg,
		fields: fields,
	})
}

func cloneFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return map[string]any{}
	}
	copied := make(map[string]any, len(fields))
	for key, value := range fields {
		copied[key] = value
	}
	return copied
}

func normalizeLogEntries(entries []recordedEntry) []logGoldenEntry {
	result := make([]logGoldenEntry, 0, len(entries))
	for _, entry := range entries {
		normalized := make(map[string]any, len(entry.fields))
		keys := make([]string, 0, len(entry.fields))
		for key := range entry.fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			normalized[key] = sanitizeValue(entry.fields[key])
		}
		result = append(result, logGoldenEntry{
			Level:  entry.level,
			Msg:    entry.msg,
			Fields: normalized,
		})
	}
	return result
}

func sanitizeValue(value any) any {
	switch v := value.(type) {
	case uuid.UUID:
		return v.String()
	case *uuid.UUID:
		if v == nil {
			return ""
		}
		return v.String()
	case time.Time:
		if v.IsZero() {
			return ""
		}
		return v.UTC().Format(time.RFC3339)
	case *time.Time:
		if v == nil {
			return ""
		}
		return v.UTC().Format(time.RFC3339)
	case interfaces.WorkflowState:
		return string(v)
	case fmt.Stringer:
		return v.String()
	case map[string]any:
		normalized := make(map[string]any, len(v))
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			normalized[key] = sanitizeValue(v[key])
		}
		return normalized
	case []any:
		cloned := make([]any, len(v))
		for i, item := range v {
			cloned[i] = sanitizeValue(item)
		}
		return cloned
	default:
		return v
	}
}
