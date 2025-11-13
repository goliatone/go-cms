package pages

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/media"
	cmsscheduler "github.com/goliatone/go-cms/internal/scheduler"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/internal/workflow"
	workflowsimple "github.com/goliatone/go-cms/internal/workflow/simple"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// Service describes page management capabilities.
type Service interface {
	Create(ctx context.Context, req CreatePageRequest) (*Page, error)
	Get(ctx context.Context, id uuid.UUID) (*Page, error)
	List(ctx context.Context) ([]*Page, error)
	Update(ctx context.Context, req UpdatePageRequest) (*Page, error)
	Delete(ctx context.Context, req DeletePageRequest) error
	UpdateTranslation(ctx context.Context, req UpdatePageTranslationRequest) (*PageTranslation, error)
	DeleteTranslation(ctx context.Context, req DeletePageTranslationRequest) error
	Move(ctx context.Context, req MovePageRequest) (*Page, error)
	Duplicate(ctx context.Context, req DuplicatePageRequest) (*Page, error)
	Schedule(ctx context.Context, req SchedulePageRequest) (*Page, error)
	CreateDraft(ctx context.Context, req CreatePageDraftRequest) (*PageVersion, error)
	PublishDraft(ctx context.Context, req PublishPagePublishRequest) (*PageVersion, error)
	ListVersions(ctx context.Context, pageID uuid.UUID) ([]*PageVersion, error)
	RestoreVersion(ctx context.Context, req RestorePageVersionRequest) (*PageVersion, error)
}

// CreatePageRequest captures the payload required to create a page.
type CreatePageRequest struct {
	ContentID                uuid.UUID
	TemplateID               uuid.UUID
	ParentID                 *uuid.UUID
	Slug                     string
	Status                   string
	CreatedBy                uuid.UUID
	UpdatedBy                uuid.UUID
	Translations             []PageTranslationInput
	AllowMissingTranslations bool
}

// PageTranslationInput represents localized routing information.
type PageTranslationInput struct {
	Locale        string
	Title         string
	Path          string
	Summary       *string
	MediaBindings media.BindingSet
}

// UpdatePageRequest captures the mutable fields for an existing page.
type UpdatePageRequest struct {
	ID                       uuid.UUID
	TemplateID               *uuid.UUID
	Status                   string
	UpdatedBy                uuid.UUID
	Translations             []PageTranslationInput
	AllowMissingTranslations bool
}

// DeletePageRequest captures the information required to delete a page.
type DeletePageRequest struct {
	ID         uuid.UUID
	DeletedBy  uuid.UUID
	HardDelete bool
}

// UpdatePageTranslationRequest mutates a specific translation for a page.
type UpdatePageTranslationRequest struct {
	PageID        uuid.UUID
	Locale        string
	Title         string
	Path          string
	Summary       *string
	MediaBindings media.BindingSet
	UpdatedBy     uuid.UUID
}

// DeletePageTranslationRequest removes a locale from a page.
type DeletePageTranslationRequest struct {
	PageID    uuid.UUID
	Locale    string
	DeletedBy uuid.UUID
}

// MovePageRequest updates the hierarchical parent for a page.
type MovePageRequest struct {
	PageID      uuid.UUID
	NewParentID *uuid.UUID
	ActorID     uuid.UUID
}

// DuplicatePageRequest clones a page, allowing optional overrides.
type DuplicatePageRequest struct {
	PageID    uuid.UUID
	Slug      string
	ParentID  *uuid.UUID
	Status    string
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// CreatePageDraftRequest captures the data required to create a page version draft.
type CreatePageDraftRequest struct {
	PageID      uuid.UUID
	Snapshot    PageVersionSnapshot
	CreatedBy   uuid.UUID
	UpdatedBy   uuid.UUID
	BaseVersion *int
}

// PublishPagePublishRequest captures the inputs required to publish a draft version.
type PublishPagePublishRequest struct {
	PageID      uuid.UUID
	Version     int
	PublishedBy uuid.UUID
	PublishedAt *time.Time
}

// RestorePageVersionRequest captures the request to restore a historical version as a draft.
type RestorePageVersionRequest struct {
	PageID     uuid.UUID
	Version    int
	RestoredBy uuid.UUID
}

// SchedulePageRequest captures scheduling input for page publish/unpublish windows.
type SchedulePageRequest struct {
	PageID      uuid.UUID
	PublishAt   *time.Time
	UnpublishAt *time.Time
	ScheduledBy uuid.UUID
}

var (
	ErrContentRequired            = errors.New("pages: content does not exist")
	ErrTemplateRequired           = errors.New("pages: template is required")
	ErrSlugRequired               = errors.New("pages: slug is required")
	ErrSlugInvalid                = errors.New("pages: slug contains invalid characters")
	ErrSlugExists                 = errors.New("pages: slug already exists")
	ErrPathExists                 = errors.New("pages: translation path already exists")
	ErrUnknownLocale              = errors.New("pages: unknown locale")
	ErrDuplicateLocale            = errors.New("pages: duplicate locale provided")
	ErrParentNotFound             = errors.New("pages: parent page not found")
	ErrNoPageTranslations         = errors.New("pages: at least one translation is required")
	ErrTemplateUnknown            = errors.New("pages: template not found")
	ErrPageRequired               = errors.New("pages: page id required")
	ErrVersioningDisabled         = errors.New("pages: versioning feature disabled")
	ErrPageVersionRequired        = errors.New("pages: version identifier required")
	ErrVersionAlreadyPublished    = errors.New("pages: version already published")
	ErrVersionRetentionExceeded   = errors.New("pages: version retention limit reached")
	ErrVersionConflict            = errors.New("pages: base version mismatch")
	ErrSchedulingDisabled         = errors.New("pages: scheduling feature disabled")
	ErrScheduleWindowInvalid      = errors.New("pages: publish_at must be before unpublish_at")
	ErrScheduleTimestampInvalid   = errors.New("pages: schedule timestamp is invalid")
	ErrPageMediaReferenceRequired = errors.New("pages: media reference requires id or path")
	ErrPageSoftDeleteUnsupported  = errors.New("pages: soft delete not supported")
	ErrPageTranslationsDisabled   = errors.New("pages: translations feature disabled")
	ErrPageTranslationNotFound    = errors.New("pages: translation not found")
	ErrPageParentCycle            = errors.New("pages: parent assignment creates hierarchy cycle")
	ErrPageDuplicateSlug          = errors.New("pages: unable to determine unique duplicate slug")
)

// PageRepository abstracts storage operations for pages.
type PageRepository interface {
	Create(ctx context.Context, record *Page) (*Page, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Page, error)
	GetBySlug(ctx context.Context, slug string) (*Page, error)
	List(ctx context.Context) ([]*Page, error)
	Update(ctx context.Context, record *Page) (*Page, error)
	ReplaceTranslations(ctx context.Context, pageID uuid.UUID, translations []*PageTranslation) error
	Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error
	CreateVersion(ctx context.Context, version *PageVersion) (*PageVersion, error)
	ListVersions(ctx context.Context, pageID uuid.UUID) ([]*PageVersion, error)
	GetVersion(ctx context.Context, pageID uuid.UUID, number int) (*PageVersion, error)
	GetLatestVersion(ctx context.Context, pageID uuid.UUID) (*PageVersion, error)
	UpdateVersion(ctx context.Context, version *PageVersion) (*PageVersion, error)
}

// LocaleRepository resolves locales.
type LocaleRepository interface {
	GetByCode(ctx context.Context, code string) (*content.Locale, error)
}

// ContentRepository allows lookups for existing content.
type ContentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*content.Content, error)
}

// PageNotFoundError indicates missing page records.
type PageNotFoundError struct {
	Key string
}

func (e *PageNotFoundError) Error() string {
	return fmt.Sprintf("page %q not found", e.Key)
}

type PageVersionNotFoundError struct {
	PageID  uuid.UUID
	Version int
}

func (e *PageVersionNotFoundError) Error() string {
	if e.PageID == uuid.Nil {
		return "page version not found"
	}
	if e.Version > 0 {
		return fmt.Sprintf("page version %s@%d not found", e.PageID.String(), e.Version)
	}
	return fmt.Sprintf("page version %s not found", e.PageID.String())
}

// ServiceOption mutates the service configuration.
type ServiceOption func(*pageService)

// WithPageClock overrides the internal clock.
func WithPageClock(clock func() time.Time) ServiceOption {
	return func(s *pageService) {
		s.now = clock
	}
}

// IDGenerator5 produces unique identifier for page entiteis
type IDGenerator func() uuid.UUID

func WithIDGenerator(generator IDGenerator) ServiceOption {
	return func(ps *pageService) {
		if generator != nil {
			ps.id = generator
		}
	}
}

type pageService struct {
	pages                 PageRepository
	content               ContentRepository
	locales               LocaleRepository
	blocks                blocks.Service
	widgets               widgets.Service
	themes                themes.Service
	media                 media.Service
	now                   func() time.Time
	id                    IDGenerator
	versioningEnabled     bool
	versionRetentionLimit int
	scheduler             interfaces.Scheduler
	schedulingEnabled     bool
	logger                interfaces.Logger
	workflow              interfaces.WorkflowEngine
	requireTranslations   bool
	translationsEnabled   bool
}

func WithBlockService(service blocks.Service) ServiceOption {
	return func(ps *pageService) {
		ps.blocks = service
	}
}

func WithWidgetService(svc widgets.Service) ServiceOption {
	return func(ps *pageService) {
		ps.widgets = svc
	}
}

func WithThemeService(svc themes.Service) ServiceOption {
	return func(ps *pageService) {
		ps.themes = svc
	}
}

// WithRequireTranslations controls whether the service enforces translation presence.
func WithRequireTranslations(required bool) ServiceOption {
	return func(ps *pageService) {
		ps.requireTranslations = required
	}
}

// WithTranslationsEnabled toggles translation handling entirely.
func WithTranslationsEnabled(enabled bool) ServiceOption {
	return func(ps *pageService) {
		ps.translationsEnabled = enabled
	}
}

func WithMediaService(svc media.Service) ServiceOption {
	return func(ps *pageService) {
		if svc != nil {
			ps.media = svc
		}
	}
}

// WithWorkflowEngine wires the workflow engine responsible for state transitions.
func WithWorkflowEngine(engine interfaces.WorkflowEngine) ServiceOption {
	return func(ps *pageService) {
		if engine != nil {
			ps.workflow = engine
		}
	}
}

// WithScheduler wires the scheduler used to enqueue publish/unpublish jobs.
func WithScheduler(scheduler interfaces.Scheduler) ServiceOption {
	return func(s *pageService) {
		if scheduler != nil {
			s.scheduler = scheduler
		}
	}
}

// WithSchedulingEnabled toggles scheduling workflows.
func WithSchedulingEnabled(enabled bool) ServiceOption {
	return func(s *pageService) {
		s.schedulingEnabled = enabled
	}
}

func WithLogger(logger interfaces.Logger) ServiceOption {
	return func(ps *pageService) {
		if logger != nil {
			ps.logger = logger
		}
	}
}

// WithPageVersioningEnabled toggles versioning specific capabilities.
func WithPageVersioningEnabled(enabled bool) ServiceOption {
	return func(s *pageService) {
		s.versioningEnabled = enabled
	}
}

// WithPageVersionRetentionLimit constrains how many versions are retained per page.
func WithPageVersionRetentionLimit(limit int) ServiceOption {
	return func(s *pageService) {
		if limit < 0 {
			limit = 0
		}
		s.versionRetentionLimit = limit
	}
}

// NewService constructs a page service with the required dependencies.
func NewService(pages PageRepository, contentRepo ContentRepository, locales LocaleRepository, opts ...ServiceOption) Service {
	s := &pageService{
		pages:               pages,
		content:             contentRepo,
		locales:             locales,
		now:                 time.Now,
		id:                  uuid.New,
		media:               media.NewNoOpService(),
		scheduler:           cmsscheduler.NewNoOp(),
		logger:              logging.PagesLogger(nil),
		workflow:            workflowsimple.New(),
		requireTranslations: true,
		translationsEnabled: true,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *pageService) log(ctx context.Context) interfaces.Logger {
	if ctx == nil {
		return s.logger
	}
	return s.logger.WithContext(ctx)
}

func (s *pageService) opLogger(ctx context.Context, operation string, extra map[string]any) interfaces.Logger {
	fields := map[string]any{"operation": operation}
	for key, value := range extra {
		fields[key] = value
	}
	return logging.WithFields(s.log(ctx), fields)
}

func (s *pageService) translationsRequired() bool {
	return s.translationsEnabled && s.requireTranslations
}

// Create registers a new page with translations and hierarchy rules.
func (s *pageService) Create(ctx context.Context, req CreatePageRequest) (*Page, error) {
	if (req.ContentID == uuid.UUID{}) {
		return nil, ErrContentRequired
	}

	if (req.TemplateID == uuid.UUID{}) {
		return nil, ErrTemplateRequired
	}

	slug := strings.TrimSpace(req.Slug)
	extra := map[string]any{
		"content_id":  req.ContentID,
		"template_id": req.TemplateID,
		"slug":        slug,
	}
	if req.ParentID != nil {
		extra["parent_id"] = *req.ParentID
	}
	logger := s.opLogger(ctx, "pages.create", extra)
	if slug == "" {
		return nil, ErrSlugRequired
	}
	if !isValidSlug(slug) {
		return nil, ErrSlugInvalid
	}

	if s.translationsRequired() && len(req.Translations) == 0 && !req.AllowMissingTranslations {
		return nil, ErrNoPageTranslations
	}

	if _, err := s.content.GetByID(ctx, req.ContentID); err != nil {
		logger.Error("page content lookup failed", "error", err)
		return nil, ErrContentRequired
	}

	if s.themes != nil {
		if _, err := s.themes.GetTemplate(ctx, req.TemplateID); err != nil {
			if errors.Is(err, themes.ErrFeatureDisabled) {
				// feature disabled, skip template validation
			} else if errors.Is(err, themes.ErrTemplateNotFound) {
				logger.Warn("page template not found", "template_id", req.TemplateID)
				return nil, ErrTemplateUnknown
			} else {
				logger.Error("page template lookup failed", "error", err)
				return nil, err
			}
		}
	}

	if existing, err := s.pages.GetBySlug(ctx, slug); err == nil && existing != nil {
		logger.Warn("page slug already exists", "existing_page_id", existing.ID)
		return nil, ErrSlugExists
	} else if err != nil {
		var notFound *PageNotFoundError
		if !errors.As(err, &notFound) {
			logger.Error("page slug lookup failed", "error", err)
			return nil, err
		}
	}

	now := s.now()
	page := &Page{
		ID:           s.id(),
		ContentID:    req.ContentID,
		ParentID:     req.ParentID,
		TemplateID:   req.TemplateID,
		Slug:         slug,
		Status:       string(domain.StatusDraft),
		CreatedBy:    req.CreatedBy,
		UpdatedBy:    req.UpdatedBy,
		CreatedAt:    now,
		UpdatedAt:    now,
		Translations: []*PageTranslation{},
	}

	if req.ParentID != nil {
		if _, err := s.pages.GetByID(ctx, *req.ParentID); err != nil {
			logger.Error("parent page lookup failed", "error", err)
			return nil, ErrParentNotFound
		}
	}

	var (
		existingPages []*Page
		err           error
	)
	if len(req.Translations) > 0 {
		existingPages, err = s.pages.List(ctx)
		if err != nil {
			logger.Error("page list for conflict check failed", "error", err)
			return nil, err
		}
		seenLocales := map[string]struct{}{}
		for _, tr := range req.Translations {
			code := strings.TrimSpace(tr.Locale)
			if code == "" {
				return nil, ErrUnknownLocale
			}
			if _, ok := seenLocales[code]; ok {
				return nil, ErrDuplicateLocale
			}

			locale, err := s.locales.GetByCode(ctx, code)
			if err != nil {
				logger.Error("page locale lookup failed", "error", err, "locale", code)
				return nil, ErrUnknownLocale
			}

			path := sanitizePath(tr.Path)
			if path == "" {
				logger.Warn("page path empty", "locale_id", locale.ID)
				return nil, ErrPathExists
			}
			if pathExists(existingPages, locale.ID, path) {
				logger.Warn("page path already exists", "locale_id", locale.ID, "path", path)
				return nil, ErrPathExists
			}
			if err := validatePageMediaBindings(tr.MediaBindings); err != nil {
				logger.Error("page media binding validation failed", "error", err)
				return nil, err
			}

			translation := &PageTranslation{
				ID:            s.id(),
				PageID:        page.ID,
				LocaleID:      locale.ID,
				Title:         tr.Title,
				Path:          path,
				Summary:       tr.Summary,
				MediaBindings: media.CloneBindingSet(tr.MediaBindings),
				CreatedAt:     now,
				UpdatedAt:     now,
			}

			page.Translations = append(page.Translations, translation)
			seenLocales[code] = struct{}{}
		}
	}

	targetState := requestedWorkflowState(req.Status)
	actorID := selectActor(req.UpdatedBy, req.CreatedBy)
	status, _, err := s.applyPageWorkflow(ctx, page, pageTransitionOptions{
		TargetState: targetState,
		ActorID:     actorID,
		Metadata: map[string]any{
			"operation": "create",
		},
	})
	if err != nil {
		logger.Error("page workflow transition failed", "error", err)
		return nil, err
	}
	page.Status = string(status)

	created, err := s.pages.Create(ctx, page)
	if err != nil {
		logger.Error("page repository create failed", "error", err)
		return nil, err
	}
	logger = logging.WithFields(logger, map[string]any{"page_id": created.ID})

	enriched, err := s.enrichPages(ctx, []*Page{created})
	if err != nil {
		logger.Error("page enrichment failed", "error", err)
		return nil, err
	}
	logger.Info("page created")
	return enriched[0], nil
}

// Get fetches a page by identifier.
func (s *pageService) Get(ctx context.Context, id uuid.UUID) (*Page, error) {
	logger := s.opLogger(ctx, "pages.get", map[string]any{"page_id": id})
	page, err := s.pages.GetByID(ctx, id)
	if err != nil {
		logger.Error("page lookup failed", "error", err)
		return nil, err
	}

	enriched, err := s.enrichPages(ctx, []*Page{page})
	if err != nil {
		logger.Error("page enrichment failed", "error", err)
		return nil, err
	}
	logger.Debug("page retrieved")
	return enriched[0], nil
}

// List returns all pages.
func (s *pageService) List(ctx context.Context) ([]*Page, error) {
	logger := s.opLogger(ctx, "pages.list", nil)
	pages, err := s.pages.List(ctx)
	if err != nil {
		logger.Error("page list failed", "error", err)
		return nil, err
	}
	enriched, err := s.enrichPages(ctx, pages)
	if err != nil {
		logger.Error("page enrichment failed", "error", err)
		return nil, err
	}
	logger.Debug("pages listed", "count", len(enriched))
	return enriched, nil
}

// Update mutates template, status, and translations for an existing page.
func (s *pageService) Update(ctx context.Context, req UpdatePageRequest) (*Page, error) {
	if req.ID == uuid.Nil {
		return nil, ErrPageRequired
	}
	if s.translationsRequired() && len(req.Translations) == 0 && !req.AllowMissingTranslations {
		return nil, ErrNoPageTranslations
	}

	logger := s.opLogger(ctx, "pages.update", map[string]any{
		"page_id": req.ID,
	})

	existing, err := s.pages.GetByID(ctx, req.ID)
	if err != nil {
		logger.Error("page lookup failed", "error", err)
		return nil, err
	}

	if req.TemplateID != nil {
		if *req.TemplateID == uuid.Nil {
			return nil, ErrTemplateRequired
		}
		if s.themes != nil {
			if _, err := s.themes.GetTemplate(ctx, *req.TemplateID); err != nil {
				if errors.Is(err, themes.ErrFeatureDisabled) {
					// Ignore when themes disabled.
				} else if errors.Is(err, themes.ErrTemplateNotFound) {
					logger.Warn("page template not found", "template_id", *req.TemplateID)
					return nil, ErrTemplateUnknown
				} else {
					logger.Error("page template lookup failed", "error", err)
					return nil, err
				}
			}
		}
		existing.TemplateID = *req.TemplateID
	}

	now := s.now()

	replaceTranslations := len(req.Translations) > 0
	var translations []*PageTranslation
	if replaceTranslations {
		allPages, err := s.pages.List(ctx)
		if err != nil {
			logger.Error("page list failed", "error", err)
			return nil, err
		}

		translations, err = s.buildPageTranslations(ctx, existing.ID, req.Translations, allPages, existing.Translations, now)
		if err != nil {
			logger.Error("page translations build failed", "error", err)
			return nil, err
		}
	}

	if req.UpdatedBy != uuid.Nil {
		existing.UpdatedBy = req.UpdatedBy
	}
	existing.UpdatedAt = now
	if replaceTranslations {
		existing.Translations = translations
	}

	targetState := requestedWorkflowState(req.Status)
	status, _, err := s.applyPageWorkflow(ctx, existing, pageTransitionOptions{
		TargetState: targetState,
		ActorID:     req.UpdatedBy,
		Metadata: map[string]any{
			"operation": "update",
		},
	})
	if err != nil {
		logger.Error("page workflow transition failed", "error", err)
		return nil, err
	}
	existing.Status = string(status)

	if replaceTranslations {
		if err := s.pages.ReplaceTranslations(ctx, existing.ID, translations); err != nil {
			logger.Error("page translations replace failed", "error", err)
			return nil, err
		}
	}

	updated, err := s.pages.Update(ctx, existing)
	if err != nil {
		logger.Error("page repository update failed", "error", err)
		return nil, err
	}

	enriched, err := s.enrichPages(ctx, []*Page{updated})
	if err != nil {
		logger.Error("page enrichment failed", "error", err)
		return nil, err
	}

	logger.Info("page updated")
	return enriched[0], nil
}

// Delete removes a page and associated scheduled jobs.
func (s *pageService) Delete(ctx context.Context, req DeletePageRequest) error {
	if req.ID == uuid.Nil {
		return ErrPageRequired
	}
	if !req.HardDelete {
		return ErrPageSoftDeleteUnsupported
	}

	logger := s.opLogger(ctx, "pages.delete", map[string]any{
		"page_id": req.ID,
	})

	if _, err := s.pages.GetByID(ctx, req.ID); err != nil {
		logger.Error("page lookup failed", "error", err)
		return err
	}

	if s.scheduler != nil {
		if err := s.scheduler.CancelByKey(ctx, cmsscheduler.PagePublishJobKey(req.ID)); err != nil && !errors.Is(err, interfaces.ErrJobNotFound) {
			logger.Warn("page publish job cancel failed", "error", err)
		}
		if err := s.scheduler.CancelByKey(ctx, cmsscheduler.PageUnpublishJobKey(req.ID)); err != nil && !errors.Is(err, interfaces.ErrJobNotFound) {
			logger.Warn("page unpublish job cancel failed", "error", err)
		}
	}

	if err := s.pages.Delete(ctx, req.ID, true); err != nil {
		logger.Error("page repository delete failed", "error", err)
		return err
	}

	logger.Info("page deleted")
	return nil
}

// UpdateTranslation mutates a single localized entry without replacing the full set.
func (s *pageService) UpdateTranslation(ctx context.Context, req UpdatePageTranslationRequest) (*PageTranslation, error) {
	if !s.translationsEnabled {
		return nil, ErrPageTranslationsDisabled
	}
	if req.PageID == uuid.Nil {
		return nil, ErrPageRequired
	}
	localeCode := strings.TrimSpace(req.Locale)
	if localeCode == "" {
		return nil, ErrUnknownLocale
	}

	logger := s.opLogger(ctx, "pages.translation.update", map[string]any{
		"page_id": req.PageID,
		"locale":  localeCode,
	})

	record, err := s.pages.GetByID(ctx, req.PageID)
	if err != nil {
		logger.Error("page lookup failed", "error", err)
		return nil, err
	}

	locale, err := s.locales.GetByCode(ctx, localeCode)
	if err != nil {
		logger.Error("locale lookup failed", "error", err)
		return nil, ErrUnknownLocale
	}

	var target *PageTranslation
	targetIdx := -1
	for idx, tr := range record.Translations {
		if tr == nil {
			continue
		}
		if tr.LocaleID == locale.ID {
			target = tr
			targetIdx = idx
			break
		}
	}
	if target == nil {
		return nil, ErrPageTranslationNotFound
	}

	allPages, err := s.pages.List(ctx)
	if err != nil {
		logger.Error("page list failed", "error", err)
		return nil, err
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = target.Title
	}

	pathInput := strings.TrimSpace(req.Path)
	if pathInput == "" {
		pathInput = target.Path
	}
	path := sanitizePath(pathInput)
	if path == "" {
		return nil, ErrPathExists
	}
	if pathExistsExcept(allPages, locale.ID, path, req.PageID) {
		return nil, ErrPathExists
	}

	var summary *string
	if req.Summary != nil {
		summary = cloneStringPtr(req.Summary)
	} else {
		summary = cloneStringPtr(target.Summary)
	}

	bindings := target.MediaBindings
	if req.MediaBindings != nil {
		if err := validatePageMediaBindings(req.MediaBindings); err != nil {
			return nil, err
		}
		bindings = media.CloneBindingSet(req.MediaBindings)
	} else {
		bindings = media.CloneBindingSet(target.MediaBindings)
	}

	now := s.now()
	updatedTranslation := &PageTranslation{
		ID:             target.ID,
		PageID:         req.PageID,
		LocaleID:       locale.ID,
		Title:          title,
		Path:           path,
		Summary:        summary,
		MediaBindings:  bindings,
		SEOTitle:       target.SEOTitle,
		SEODescription: target.SEODescription,
		CreatedAt:      target.CreatedAt,
		UpdatedAt:      now,
		Locale:         target.Locale,
	}

	translations := make([]*PageTranslation, len(record.Translations))
	for i, tr := range record.Translations {
		if i == targetIdx {
			translations[i] = updatedTranslation
			continue
		}
		translations[i] = tr
	}

	if err := s.pages.ReplaceTranslations(ctx, req.PageID, translations); err != nil {
		logger.Error("page translation replace failed", "error", err)
		return nil, err
	}

	record.Translations = translations
	record.UpdatedAt = now
	if req.UpdatedBy != uuid.Nil {
		record.UpdatedBy = req.UpdatedBy
	}
	if _, err := s.pages.Update(ctx, record); err != nil {
		logger.Error("page update failed after translation mutate", "error", err)
		return nil, err
	}

	logger.Info("page translation updated")
	return updatedTranslation, nil
}

// DeleteTranslation removes a translation for a page.
func (s *pageService) DeleteTranslation(ctx context.Context, req DeletePageTranslationRequest) error {
	if !s.translationsEnabled {
		return ErrPageTranslationsDisabled
	}
	if req.PageID == uuid.Nil {
		return ErrPageRequired
	}
	localeCode := strings.TrimSpace(req.Locale)
	if localeCode == "" {
		return ErrUnknownLocale
	}

	logger := s.opLogger(ctx, "pages.translation.delete", map[string]any{
		"page_id": req.PageID,
		"locale":  localeCode,
	})

	record, err := s.pages.GetByID(ctx, req.PageID)
	if err != nil {
		logger.Error("page lookup failed", "error", err)
		return err
	}
	if len(record.Translations) == 0 {
		return ErrPageTranslationNotFound
	}

	locale, err := s.locales.GetByCode(ctx, localeCode)
	if err != nil {
		logger.Error("locale lookup failed", "error", err)
		return ErrUnknownLocale
	}

	newTranslations := make([]*PageTranslation, 0, len(record.Translations))
	removed := false
	for _, tr := range record.Translations {
		if tr == nil {
			continue
		}
		if tr.LocaleID == locale.ID {
			removed = true
			continue
		}
		newTranslations = append(newTranslations, tr)
	}

	if !removed {
		return ErrPageTranslationNotFound
	}
	if s.translationsRequired() && len(newTranslations) == 0 {
		return ErrNoPageTranslations
	}

	if err := s.pages.ReplaceTranslations(ctx, req.PageID, newTranslations); err != nil {
		logger.Error("page translation replace failed", "error", err)
		return err
	}

	record.Translations = newTranslations
	record.UpdatedAt = s.now()
	if req.DeletedBy != uuid.Nil {
		record.UpdatedBy = req.DeletedBy
	}
	if _, err := s.pages.Update(ctx, record); err != nil {
		logger.Error("page update failed after translation delete", "error", err)
		return err
	}

	logger.Info("page translation deleted")
	return nil
}

// Move updates the parent of a page while preventing hierarchy cycles.
func (s *pageService) Move(ctx context.Context, req MovePageRequest) (*Page, error) {
	if req.PageID == uuid.Nil {
		return nil, ErrPageRequired
	}

	fields := map[string]any{
		"page_id": req.PageID,
	}
	if req.NewParentID != nil {
		fields["parent_id"] = *req.NewParentID
	}
	logger := s.opLogger(ctx, "pages.move", fields)

	record, err := s.pages.GetByID(ctx, req.PageID)
	if err != nil {
		logger.Error("page lookup failed", "error", err)
		return nil, err
	}

	if err := s.ensureValidParent(ctx, req.PageID, req.NewParentID); err != nil {
		logger.Error("page parent validation failed", "error", err)
		return nil, err
	}

	currentParent := uuid.Nil
	if record.ParentID != nil {
		currentParent = *record.ParentID
	}
	newParent := uuid.Nil
	if req.NewParentID != nil {
		newParent = *req.NewParentID
	}
	if currentParent == newParent {
		enriched, err := s.enrichPages(ctx, []*Page{record})
		if err != nil {
			return nil, err
		}
		return enriched[0], nil
	}

	if req.NewParentID == nil {
		record.ParentID = nil
	} else {
		parent := *req.NewParentID
		record.ParentID = &parent
	}
	record.UpdatedAt = s.now()
	if req.ActorID != uuid.Nil {
		record.UpdatedBy = req.ActorID
	}

	updated, err := s.pages.Update(ctx, record)
	if err != nil {
		logger.Error("page repository update failed", "error", err)
		return nil, err
	}

	enriched, err := s.enrichPages(ctx, []*Page{updated})
	if err != nil {
		logger.Error("page enrichment failed", "error", err)
		return nil, err
	}

	logger.Info("page moved")
	return enriched[0], nil
}

// Duplicate clones an existing page with a unique slug and paths.
func (s *pageService) Duplicate(ctx context.Context, req DuplicatePageRequest) (*Page, error) {
	if req.PageID == uuid.Nil {
		return nil, ErrPageRequired
	}

	logger := s.opLogger(ctx, "pages.duplicate", map[string]any{
		"page_id": req.PageID,
	})

	source, err := s.pages.GetByID(ctx, req.PageID)
	if err != nil {
		logger.Error("page lookup failed", "error", err)
		return nil, err
	}

	slug, err := s.generateDuplicateSlug(ctx, req.Slug, source.Slug)
	if err != nil {
		logger.Error("duplicate slug resolution failed", "error", err)
		return nil, err
	}

	parentID := source.ParentID
	if req.ParentID != nil {
		parentID = req.ParentID
	}

	newPageID := s.id()

	if err := s.ensureValidParent(ctx, newPageID, parentID); err != nil {
		logger.Error("duplicate parent validation failed", "error", err)
		return nil, err
	}

	if _, err := s.content.GetByID(ctx, source.ContentID); err != nil {
		logger.Error("duplicate content lookup failed", "error", err)
		return nil, ErrContentRequired
	}

	if s.themes != nil {
		if _, err := s.themes.GetTemplate(ctx, source.TemplateID); err != nil {
			if errors.Is(err, themes.ErrFeatureDisabled) {
				// ignore when themes disabled
			} else {
				logger.Error("duplicate template lookup failed", "error", err)
				return nil, err
			}
		}
	}

	allPages, err := s.pages.List(ctx)
	if err != nil {
		logger.Error("page list failed", "error", err)
		return nil, err
	}

	now := s.now()
	createdBy := selectActor(req.CreatedBy, source.CreatedBy)
	updatedBy := selectActor(req.UpdatedBy, createdBy)
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = string(domain.StatusDraft)
	}

	var parentPtr *uuid.UUID
	if parentID != nil {
		parentValue := *parentID
		parentPtr = &parentValue
	}

	cloned := &Page{
		ID:         newPageID,
		ContentID:  source.ContentID,
		ParentID:   parentPtr,
		TemplateID: source.TemplateID,
		Slug:       slug,
		Status:     status,
		CreatedBy:  createdBy,
		UpdatedBy:  updatedBy,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if len(source.Translations) == 0 && s.translationsRequired() {
		return nil, ErrNoPageTranslations
	}

	for _, tr := range source.Translations {
		if tr == nil {
			continue
		}
		path := deriveDuplicatePath(allPages, tr.LocaleID, tr.Path)
		clonedTranslation := &PageTranslation{
			ID:            s.id(),
			PageID:        newPageID,
			LocaleID:      tr.LocaleID,
			Title:         tr.Title,
			Path:          path,
			Summary:       cloneStringPtr(tr.Summary),
			MediaBindings: media.CloneBindingSet(tr.MediaBindings),
			CreatedAt:     now,
			UpdatedAt:     now,
			Locale:        tr.Locale,
		}
		cloned.Translations = append(cloned.Translations, clonedTranslation)
	}

	if s.translationsRequired() && len(cloned.Translations) == 0 {
		return nil, ErrNoPageTranslations
	}

	targetState := requestedWorkflowState(status)
	state, _, err := s.applyPageWorkflow(ctx, cloned, pageTransitionOptions{
		TargetState: targetState,
		ActorID:     updatedBy,
		Metadata: map[string]any{
			"operation": "duplicate",
		},
	})
	if err != nil {
		logger.Error("duplicate workflow transition failed", "error", err)
		return nil, err
	}
	cloned.Status = string(state)

	created, err := s.pages.Create(ctx, cloned)
	if err != nil {
		logger.Error("page repository create failed", "error", err)
		return nil, err
	}

	enriched, err := s.enrichPages(ctx, []*Page{created})
	if err != nil {
		logger.Error("page enrichment failed", "error", err)
		return nil, err
	}

	logger.Info("page duplicated")
	return enriched[0], nil
}

// Schedule registers publish/unpublish windows for a page and enqueues scheduler jobs.
func (s *pageService) Schedule(ctx context.Context, req SchedulePageRequest) (*Page, error) {
	if !s.schedulingEnabled {
		return nil, ErrSchedulingDisabled
	}
	if req.PageID == uuid.Nil {
		return nil, ErrPageRequired
	}
	if req.PublishAt != nil && req.UnpublishAt != nil && req.UnpublishAt.Before(*req.PublishAt) {
		return nil, ErrScheduleWindowInvalid
	}
	if req.PublishAt != nil && req.PublishAt.IsZero() {
		return nil, ErrScheduleTimestampInvalid
	}
	if req.UnpublishAt != nil && req.UnpublishAt.IsZero() {
		return nil, ErrScheduleTimestampInvalid
	}

	logger := s.opLogger(ctx, "pages.schedule", map[string]any{"page_id": req.PageID})

	record, err := s.pages.GetByID(ctx, req.PageID)
	if err != nil {
		logger.Error("page schedule lookup failed", "error", err)
		return nil, err
	}

	now := s.now()
	record.PublishAt = cloneTimePtr(req.PublishAt)
	record.UnpublishAt = cloneTimePtr(req.UnpublishAt)
	record.UpdatedAt = now
	if req.ScheduledBy != uuid.Nil {
		record.UpdatedBy = req.ScheduledBy
	}

	var targetState interfaces.WorkflowState
	switch {
	case record.PublishAt != nil:
		targetState = interfaces.WorkflowState(domain.WorkflowStateScheduled)
	case record.PublishedVersion != nil:
		targetState = interfaces.WorkflowState(domain.WorkflowStatePublished)
	default:
		targetState = interfaces.WorkflowState(domain.WorkflowStateDraft)
	}

	status, _, err := s.applyPageWorkflow(ctx, record, pageTransitionOptions{
		TargetState: targetState,
		ActorID:     req.ScheduledBy,
		Metadata: map[string]any{
			"operation": "schedule",
		},
	})
	if err != nil {
		logger.Error("page workflow transition failed", "error", err)
		return nil, err
	}
	record.Status = string(status)

	if s.scheduler != nil {
		if record.PublishAt != nil {
			payload := map[string]any{"page_id": record.ID.String()}
			if req.ScheduledBy != uuid.Nil {
				payload["scheduled_by"] = req.ScheduledBy.String()
			}
			if _, err := s.scheduler.Enqueue(ctx, interfaces.JobSpec{
				Key:     cmsscheduler.PagePublishJobKey(record.ID),
				Type:    cmsscheduler.JobTypePagePublish,
				RunAt:   *record.PublishAt,
				Payload: payload,
			}); err != nil {
				logger.Error("page publish job enqueue failed", "error", err)
				return nil, err
			}
			logger.Debug("page publish job enqueued", "job_key", cmsscheduler.PagePublishJobKey(record.ID))
		} else {
			cancelErr := s.scheduler.CancelByKey(ctx, cmsscheduler.PagePublishJobKey(record.ID))
			if cancelErr != nil && !errors.Is(cancelErr, interfaces.ErrJobNotFound) {
				logger.Error("page publish job cancel failed", "error", cancelErr)
				return nil, cancelErr
			}
			if cancelErr == nil {
				logger.Debug("page publish job cancelled", "job_key", cmsscheduler.PagePublishJobKey(record.ID))
			}
		}

		if record.UnpublishAt != nil {
			payload := map[string]any{"page_id": record.ID.String()}
			if req.ScheduledBy != uuid.Nil {
				payload["scheduled_by"] = req.ScheduledBy.String()
			}
			if _, err := s.scheduler.Enqueue(ctx, interfaces.JobSpec{
				Key:     cmsscheduler.PageUnpublishJobKey(record.ID),
				Type:    cmsscheduler.JobTypePageUnpublish,
				RunAt:   *record.UnpublishAt,
				Payload: payload,
			}); err != nil {
				logger.Error("page unpublish job enqueue failed", "error", err)
				return nil, err
			}
			logger.Debug("page unpublish job enqueued", "job_key", cmsscheduler.PageUnpublishJobKey(record.ID))
		} else {
			cancelErr := s.scheduler.CancelByKey(ctx, cmsscheduler.PageUnpublishJobKey(record.ID))
			if cancelErr != nil && !errors.Is(cancelErr, interfaces.ErrJobNotFound) {
				logger.Error("page unpublish job cancel failed", "error", cancelErr)
				return nil, cancelErr
			}
			if cancelErr == nil {
				logger.Debug("page unpublish job cancelled", "job_key", cmsscheduler.PageUnpublishJobKey(record.ID))
			}
		}
	}

	updated, err := s.pages.Update(ctx, record)
	if err != nil {
		logger.Error("page schedule update failed", "error", err)
		return nil, err
	}
	enriched, err := s.enrichPages(ctx, []*Page{updated})
	if err != nil {
		logger.Error("page enrichment failed", "error", err)
		return nil, err
	}
	logger.Info("page schedule updated",
		"publish_at", record.PublishAt,
		"unpublish_at", record.UnpublishAt,
		"status", record.Status,
	)
	return enriched[0], nil
}

func (s *pageService) CreateDraft(ctx context.Context, req CreatePageDraftRequest) (*PageVersion, error) {
	if !s.versioningEnabled {
		return nil, ErrVersioningDisabled
	}
	if req.PageID == uuid.Nil {
		return nil, ErrPageRequired
	}

	extra := map[string]any{
		"page_id": req.PageID,
	}
	if req.BaseVersion != nil {
		extra["base_version"] = *req.BaseVersion
	}
	logger := s.opLogger(ctx, "pages.version.create_draft", extra)

	page, err := s.pages.GetByID(ctx, req.PageID)
	if err != nil {
		logger.Error("page lookup failed", "error", err)
		return nil, err
	}

	existing, err := s.pages.ListVersions(ctx, req.PageID)
	if err != nil {
		logger.Error("page version list failed", "error", err)
		return nil, err
	}
	logger.Debug("page versions loaded", "count", len(existing))

	if s.versionRetentionLimit > 0 && len(existing) >= s.versionRetentionLimit {
		logger.Warn("page version retention limit reached", "limit", s.versionRetentionLimit)
		return nil, ErrVersionRetentionExceeded
	}

	next := nextVersionNumber(existing)
	if req.BaseVersion != nil && *req.BaseVersion != next-1 {
		logger.Warn("page version conflict detected", "expected_base", next-1, "provided_base", *req.BaseVersion)
		return nil, ErrVersionConflict
	}

	now := s.now()
	version := &PageVersion{
		ID:        s.id(),
		PageID:    req.PageID,
		Version:   next,
		Status:    domain.StatusDraft,
		Snapshot:  clonePageVersionSnapshot(req.Snapshot),
		CreatedBy: req.CreatedBy,
		CreatedAt: now,
	}

	created, err := s.pages.CreateVersion(ctx, version)
	if err != nil {
		logger.Error("page version create failed", "error", err)
		return nil, err
	}

	page.CurrentVersion = created.Version
	page.UpdatedAt = now
	switch {
	case req.UpdatedBy != uuid.Nil:
		page.UpdatedBy = req.UpdatedBy
	case req.CreatedBy != uuid.Nil:
		page.UpdatedBy = req.CreatedBy
	}
	if page.PublishedVersion == nil {
		status, _, err := s.applyPageWorkflow(ctx, page, pageTransitionOptions{
			TargetState: interfaces.WorkflowState(domain.WorkflowStateDraft),
			ActorID:     selectActor(req.UpdatedBy, req.CreatedBy),
			Metadata: map[string]any{
				"operation": "create_draft",
				"version":   created.Version,
			},
		})
		if err != nil {
			logger.Error("page workflow transition failed", "error", err)
			return nil, err
		}
		page.Status = string(status)
	}

	if _, err := s.pages.Update(ctx, page); err != nil {
		logger.Error("page record update failed", "error", err)
		return nil, err
	}

	logger = logging.WithFields(logger, map[string]any{"version": created.Version})
	logger.Info("page draft created")

	return clonePageVersion(created), nil
}

func (s *pageService) PublishDraft(ctx context.Context, req PublishPagePublishRequest) (*PageVersion, error) {
	if !s.versioningEnabled {
		return nil, ErrVersioningDisabled
	}
	if req.PageID == uuid.Nil {
		return nil, ErrPageRequired
	}
	if req.Version <= 0 {
		return nil, ErrPageVersionRequired
	}

	logger := s.opLogger(ctx, "pages.version.publish", map[string]any{
		"page_id": req.PageID,
		"version": req.Version,
	})

	page, err := s.pages.GetByID(ctx, req.PageID)
	if err != nil {
		logger.Error("page lookup failed", "error", err)
		return nil, err
	}

	version, err := s.pages.GetVersion(ctx, req.PageID, req.Version)
	if err != nil {
		logger.Error("page version lookup failed", "error", err)
		return nil, err
	}
	if version.Status == domain.StatusPublished {
		logger.Warn("page version already published")
		return nil, ErrVersionAlreadyPublished
	}

	publishedAt := s.now()
	if req.PublishedAt != nil && !req.PublishedAt.IsZero() {
		publishedAt = *req.PublishedAt
	}
	version.Status = domain.StatusPublished
	version.PublishedAt = &publishedAt
	if req.PublishedBy != uuid.Nil {
		version.PublishedBy = &req.PublishedBy
	}

	updatedVersion, err := s.pages.UpdateVersion(ctx, version)
	if err != nil {
		logger.Error("page version update failed", "error", err)
		return nil, err
	}

	if page.PublishedVersion != nil && *page.PublishedVersion != updatedVersion.Version {
		previous, prevErr := s.pages.GetVersion(ctx, req.PageID, *page.PublishedVersion)
		if prevErr == nil && previous.Status == domain.StatusPublished {
			previous.Status = domain.StatusArchived
			if _, archiveErr := s.pages.UpdateVersion(ctx, previous); archiveErr != nil {
				logger.Error("page previous version archive failed", "error", archiveErr, "previous_version", previous.Version)
				return nil, archiveErr
			}
			logger.Debug("page previous version archived", "previous_version", previous.Version)
		} else if prevErr != nil {
			logger.Error("page previous version lookup failed", "error", prevErr, "previous_version", *page.PublishedVersion)
		}
	}

	page.PublishedVersion = &updatedVersion.Version
	page.PublishedAt = &publishedAt
	if req.PublishedBy != uuid.Nil {
		page.PublishedBy = &req.PublishedBy
	}
	if updatedVersion.Version > page.CurrentVersion {
		page.CurrentVersion = updatedVersion.Version
	}
	page.UpdatedAt = s.now()
	if req.PublishedBy != uuid.Nil {
		page.UpdatedBy = req.PublishedBy
	}

	targetState := interfaces.WorkflowState(domain.WorkflowStatePublished)
	transitionName := "publish"
	if strings.EqualFold(page.Status, string(targetState)) {
		transitionName = ""
	}

	status, _, err := s.applyPageWorkflow(ctx, page, pageTransitionOptions{
		Transition:  transitionName,
		TargetState: targetState,
		ActorID:     req.PublishedBy,
		Metadata: map[string]any{
			"operation": "publish_draft",
			"version":   updatedVersion.Version,
		},
	})
	if err != nil {
		logger.Error("page workflow transition failed", "error", err)
		return nil, err
	}
	page.Status = string(status)

	if _, err := s.pages.Update(ctx, page); err != nil {
		logger.Error("page publish update failed", "error", err)
		return nil, err
	}

	logger.Info("page version published", "published_at", publishedAt)

	return clonePageVersion(updatedVersion), nil
}

func (s *pageService) ListVersions(ctx context.Context, pageID uuid.UUID) ([]*PageVersion, error) {
	if !s.versioningEnabled {
		return nil, ErrVersioningDisabled
	}
	if pageID == uuid.Nil {
		return nil, ErrPageRequired
	}

	logger := s.opLogger(ctx, "pages.version.list", map[string]any{"page_id": pageID})

	versions, err := s.pages.ListVersions(ctx, pageID)
	if err != nil {
		logger.Error("page version list failed", "error", err)
		return nil, err
	}
	results := clonePageVersions(versions)
	logger.Debug("page versions returned", "count", len(results))
	return results, nil
}

func (s *pageService) RestoreVersion(ctx context.Context, req RestorePageVersionRequest) (*PageVersion, error) {
	if !s.versioningEnabled {
		return nil, ErrVersioningDisabled
	}
	if req.PageID == uuid.Nil {
		return nil, ErrPageRequired
	}
	if req.Version <= 0 {
		return nil, ErrPageVersionRequired
	}

	logger := s.opLogger(ctx, "pages.version.restore", map[string]any{
		"page_id": req.PageID,
		"version": req.Version,
	})

	version, err := s.pages.GetVersion(ctx, req.PageID, req.Version)
	if err != nil {
		logger.Error("page version lookup failed", "error", err)
		return nil, err
	}

	return s.CreateDraft(ctx, CreatePageDraftRequest{
		PageID:    req.PageID,
		Snapshot:  clonePageVersionSnapshot(version.Snapshot),
		CreatedBy: req.RestoredBy,
		UpdatedBy: req.RestoredBy,
	})
}

func (s *pageService) enrichPages(ctx context.Context, pages []*Page) ([]*Page, error) {
	withBlocks, err := s.attachBlocks(ctx, pages)
	if err != nil {
		return nil, err
	}
	withWidgets, err := s.attachWidgets(ctx, withBlocks)
	if err != nil {
		return nil, err
	}
	withMedia, err := s.attachMedia(ctx, withWidgets)
	if err != nil {
		return nil, err
	}
	for _, page := range withMedia {
		s.decoratePage(page)
	}
	return withMedia, nil
}

func (s *pageService) attachBlocks(ctx context.Context, pages []*Page) ([]*Page, error) {
	if s.blocks == nil || len(pages) == 0 {
		return pages, nil
	}

	global := []*blocks.Instance{}
	if inst, err := s.blocks.ListGlobalInstances(ctx); err == nil {
		global = inst
	} else {
		var nf *blocks.NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
	}

	enriched := make([]*Page, 0, len(pages))
	for _, page := range pages {
		if page == nil {
			enriched = append(enriched, nil)
			continue
		}
		clone := *page
		pageBlocks, err := s.blocks.ListPageInstances(ctx, page.ID)
		if err != nil {
			var nf *blocks.NotFoundError
			if !errors.As(err, &nf) {
				return nil, err
			}
			pageBlocks = nil
		}

		combined := append(cloneBlockInstances(pageBlocks), cloneBlockInstances(global)...)
		clone.Blocks = combined
		enriched = append(enriched, &clone)
	}

	return enriched, nil
}

func (s *pageService) attachWidgets(ctx context.Context, pages []*Page) ([]*Page, error) {
	if s.widgets == nil || len(pages) == 0 {
		return pages, nil
	}

	definitions, err := s.widgets.ListAreaDefinitions(ctx)
	if err != nil {
		if errors.Is(err, widgets.ErrFeatureDisabled) || errors.Is(err, widgets.ErrAreaFeatureDisabled) {
			return pages, nil
		}
		return nil, err
	}
	if len(definitions) == 0 {
		return pages, nil
	}

	templateCache := make(map[uuid.UUID]*themes.Template)
	regionCache := make(map[uuid.UUID]map[string]struct{})

	now := s.now()
	enriched := make([]*Page, 0, len(pages))
	for _, page := range pages {
		if page == nil {
			enriched = append(enriched, nil)
			continue
		}

		clone := *page
		var areaWidgets map[string][]*widgets.ResolvedWidget

		var template *themes.Template
		allowedRegions := map[string]struct{}{}
		if s.themes != nil && page.TemplateID != uuid.Nil {
			if cached, ok := templateCache[page.TemplateID]; ok {
				template = cached
			} else {
				tpl, tplErr := s.themes.GetTemplate(ctx, page.TemplateID)
				if tplErr != nil {
					if errors.Is(tplErr, themes.ErrFeatureDisabled) || errors.Is(tplErr, themes.ErrTemplateNotFound) {
						templateCache[page.TemplateID] = nil
					} else {
						return nil, tplErr
					}
				} else {
					templateCache[page.TemplateID] = tpl
					template = tpl
				}
			}
			if template != nil {
				if cachedRegions, ok := regionCache[template.ID]; ok {
					if cachedRegions != nil {
						allowedRegions = cachedRegions
					}
				} else {
					infos, regionErr := s.themes.TemplateRegions(ctx, template.ID)
					if regionErr != nil {
						if errors.Is(regionErr, themes.ErrFeatureDisabled) || errors.Is(regionErr, themes.ErrTemplateNotFound) {
							regionCache[template.ID] = nil
						} else {
							return nil, regionErr
						}
					} else {
						regionMap := make(map[string]struct{})
						for _, info := range infos {
							if info.AcceptsWidgets {
								regionMap[info.Key] = struct{}{}
							}
						}
						regionCache[template.ID] = regionMap
						allowedRegions = regionMap
					}
				}
			}
		}
		for _, definition := range definitions {
			if definition == nil {
				continue
			}
			code := strings.TrimSpace(definition.Code)
			if code == "" {
				continue
			}
			if template != nil && !areaDefinitionApplies(definition, template) {
				continue
			}
			if len(allowedRegions) > 0 {
				if _, ok := allowedRegions[code]; !ok {
					continue
				}
			}

			resolved, err := s.widgets.ResolveArea(ctx, widgets.ResolveAreaInput{
				AreaCode: code,
				Now:      now,
			})
			if err != nil {
				if errors.Is(err, widgets.ErrFeatureDisabled) || errors.Is(err, widgets.ErrAreaFeatureDisabled) {
					areaWidgets = nil
					break
				}
				return nil, err
			}
			if len(resolved) == 0 {
				continue
			}
			if areaWidgets == nil {
				areaWidgets = make(map[string][]*widgets.ResolvedWidget)
			}
			areaWidgets[code] = cloneResolvedWidgetSlice(resolved)
		}

		if len(areaWidgets) > 0 {
			clone.Widgets = areaWidgets
		}
		enriched = append(enriched, &clone)
	}
	return enriched, nil
}

func (s *pageService) attachMedia(ctx context.Context, pages []*Page) ([]*Page, error) {
	if len(pages) == 0 {
		return pages, nil
	}
	enriched := make([]*Page, 0, len(pages))
	for _, page := range pages {
		if page == nil {
			enriched = append(enriched, nil)
			continue
		}
		clone := *page
		if len(page.Translations) > 0 {
			hydrated := make([]*PageTranslation, len(page.Translations))
			for i, tr := range page.Translations {
				translation, err := s.hydrateTranslation(ctx, tr)
				if err != nil {
					return nil, err
				}
				hydrated[i] = translation
			}
			clone.Translations = hydrated
		}
		enriched = append(enriched, &clone)
	}
	return enriched, nil
}

func validatePageMediaBindings(bindings media.BindingSet) error {
	for slot, entries := range bindings {
		for _, binding := range entries {
			reference := binding.Reference
			if strings.TrimSpace(reference.ID) == "" && strings.TrimSpace(reference.Path) == "" {
				return fmt.Errorf("%w: %s", ErrPageMediaReferenceRequired, slot)
			}
		}
	}
	return nil
}

func (s *pageService) hydrateTranslation(ctx context.Context, translation *PageTranslation) (*PageTranslation, error) {
	if translation == nil {
		return nil, nil
	}
	clone := *translation
	clone.MediaBindings = media.CloneBindingSet(translation.MediaBindings)
	if len(clone.MediaBindings) == 0 || s.media == nil {
		clone.ResolvedMedia = nil
		return &clone, nil
	}
	resolved, err := s.media.ResolveBindings(ctx, clone.MediaBindings, media.ResolveOptions{})
	if err != nil {
		return nil, err
	}
	clone.ResolvedMedia = resolved
	return &clone, nil
}

func cloneResolvedWidgetSlice(input []*widgets.ResolvedWidget) []*widgets.ResolvedWidget {
	if len(input) == 0 {
		return nil
	}
	cloned := make([]*widgets.ResolvedWidget, len(input))
	copy(cloned, input)
	return cloned
}

func areaDefinitionApplies(definition *widgets.AreaDefinition, template *themes.Template) bool {
	if definition == nil {
		return false
	}
	scope := definition.Scope
	switch scope {
	case widgets.AreaScopeTheme:
		if template == nil || definition.ThemeID == nil {
			return false
		}
		return template.ThemeID == *definition.ThemeID
	case widgets.AreaScopeTemplate:
		if template == nil || definition.TemplateID == nil {
			return false
		}
		return template.ID == *definition.TemplateID
	default:
		// Treat global/unspecified scopes as applicable everywhere.
		return true
	}
}

func (s *pageService) decoratePage(page *Page) *Page {
	if page == nil {
		return nil
	}
	status := effectivePageStatus(page, s.now())
	page.EffectiveStatus = status
	page.IsVisible = status == domain.StatusPublished
	return page
}

func effectivePageStatus(page *Page, now time.Time) domain.Status {
	if page == nil {
		return domain.StatusDraft
	}
	state := domain.WorkflowStateFromStatus(domain.Status(page.Status))
	status := domain.StatusFromWorkflowState(state)
	if strings.TrimSpace(string(status)) == "" {
		status = domain.Status(page.Status)
		if strings.TrimSpace(string(status)) == "" {
			status = domain.StatusDraft
		}
	}

	if page.UnpublishAt != nil && !page.UnpublishAt.After(now) {
		return domain.StatusArchived
	}

	if page.PublishAt != nil {
		if page.PublishAt.After(now) {
			return domain.StatusScheduled
		}
		return domain.StatusPublished
	}

	if page.PublishedAt != nil && !page.PublishedAt.After(now) {
		return domain.StatusPublished
	}

	return status
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

type pageTransitionOptions struct {
	Transition  string
	TargetState interfaces.WorkflowState
	ActorID     uuid.UUID
	Metadata    map[string]any
}

func (s *pageService) applyPageWorkflow(ctx context.Context, page *Page, opts pageTransitionOptions) (domain.Status, domain.WorkflowState, error) {
	if page == nil {
		return "", "", fmt.Errorf("pages: workflow transition requires page context")
	}

	currentState := domain.WorkflowStateFromStatus(domain.Status(page.Status))
	desiredState := currentState
	if strings.TrimSpace(string(opts.TargetState)) != "" {
		desiredState = domain.WorkflowState(domain.NormalizeWorkflowState(string(opts.TargetState)))
	}

	if s.workflow == nil {
		status := domain.StatusFromWorkflowState(desiredState)
		if strings.TrimSpace(string(status)) == "" {
			status = domain.Status(desiredState)
		}
		if strings.TrimSpace(string(status)) == "" {
			status = domain.StatusDraft
		}
		return status, desiredState, nil
	}

	context := buildPageContext(page)
	metadata := mergeMetadata(context.Metadata(), opts.Metadata)

	result, err := s.workflow.Transition(ctx, interfaces.TransitionInput{
		EntityID:     page.ID,
		EntityType:   workflow.EntityTypePage,
		CurrentState: interfaces.WorkflowState(context.WorkflowState),
		Transition:   opts.Transition,
		TargetState:  opts.TargetState,
		ActorID:      opts.ActorID,
		Metadata:     metadata,
	})
	if err != nil {
		return "", "", err
	}

	newState := domain.WorkflowState(result.ToState)
	status := domain.StatusFromWorkflowState(newState)
	if strings.TrimSpace(string(status)) == "" {
		status = domain.Status(newState)
		if strings.TrimSpace(string(status)) == "" {
			status = domain.StatusDraft
		}
	}
	s.recordWorkflowEvents(ctx, page, result, metadata)
	return status, newState, nil
}

func (s *pageService) recordWorkflowEvents(ctx context.Context, page *Page, result *interfaces.TransitionResult, metadata map[string]any) {
	if result == nil || len(result.Events) == 0 {
		return
	}

	logger := s.log(ctx)
	fields := map[string]any{
		"page_id":             result.EntityID,
		"workflow_from":       result.FromState,
		"workflow_to":         result.ToState,
		"workflow_transition": result.Transition,
	}
	if page != nil && strings.TrimSpace(page.Slug) != "" {
		fields["page_slug"] = page.Slug
	}
	if strings.TrimSpace(result.EntityType) != "" {
		fields["workflow_entity"] = result.EntityType
	}
	if result.ActorID != uuid.Nil {
		fields["actor_id"] = result.ActorID
	}
	if !result.CompletedAt.IsZero() {
		fields["workflow_completed_at"] = result.CompletedAt
	}
	if metadata != nil {
		if op, ok := metadata["operation"]; ok {
			fields["operation"] = op
		}
	}

	for _, event := range result.Events {
		eventFields := maps.Clone(fields)
		eventFields["workflow_event"] = event.Name
		if !event.Timestamp.IsZero() {
			eventFields["workflow_event_time"] = event.Timestamp
		}
		if len(event.Payload) > 0 {
			eventFields["workflow_event_payload"] = event.Payload
		}
		logging.WithFields(logger, eventFields).Info("workflow event emitted")
	}
}

func buildPageContext(page *Page) workflow.PageContext {
	return workflow.PageContext{
		ID:               page.ID,
		ContentID:        page.ContentID,
		TemplateID:       page.TemplateID,
		ParentID:         page.ParentID,
		Slug:             page.Slug,
		Status:           domain.Status(page.Status),
		WorkflowState:    domain.WorkflowStateFromStatus(domain.Status(page.Status)),
		CurrentVersion:   page.CurrentVersion,
		PublishedVersion: page.PublishedVersion,
		PublishAt:        page.PublishAt,
		UnpublishAt:      page.UnpublishAt,
		CreatedBy:        page.CreatedBy,
		UpdatedBy:        page.UpdatedBy,
	}
}

func mergeMetadata(base map[string]any, extra map[string]any) map[string]any {
	if len(extra) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]any, len(extra))
	}
	for key, value := range extra {
		base[key] = value
	}
	return base
}

func requestedWorkflowState(status string) interfaces.WorkflowState {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return ""
	}
	return interfaces.WorkflowState(domain.NormalizeWorkflowState(trimmed))
}

func selectActor(primary, fallback uuid.UUID) uuid.UUID {
	if primary != uuid.Nil {
		return primary
	}
	return fallback
}

func cloneBlockInstances(instances []*blocks.Instance) []*blocks.Instance {
	if len(instances) == 0 {
		return nil
	}

	cloned := make([]*blocks.Instance, len(instances))
	for i, inst := range instances {
		if inst == nil {
			continue
		}
		copyInst := *inst
		if inst.Configuration != nil {
			copyInst.Configuration = maps.Clone(inst.Configuration)
		}
		if len(inst.Translations) > 0 {
			copyInst.Translations = make([]*blocks.Translation, len(inst.Translations))
			for j, tr := range inst.Translations {
				if tr == nil {
					continue
				}
				copyTr := *tr
				if tr.Content != nil {
					copyTr.Content = maps.Clone(tr.Content)
				}
				if tr.AttributeOverride != nil {
					copyTr.AttributeOverride = maps.Clone(tr.AttributeOverride)
				}
				copyTr.MediaBindings = media.CloneBindingSet(tr.MediaBindings)
				copyTr.ResolvedMedia = media.CloneAttachments(tr.ResolvedMedia)
				copyInst.Translations[j] = &copyTr
			}
		}
		cloned[i] = &copyInst
	}
	return cloned
}

func sanitizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func pathExists(pages []*Page, localeID uuid.UUID, path string) bool {
	for _, p := range pages {
		for _, tr := range p.Translations {
			if tr == nil {
				continue
			}
			if tr.LocaleID == localeID && strings.EqualFold(tr.Path, path) {
				return true
			}
		}
	}
	return false
}

func pathExistsExcept(pages []*Page, localeID uuid.UUID, path string, ignorePageID uuid.UUID) bool {
	for _, p := range pages {
		if p == nil {
			continue
		}
		if ignorePageID != uuid.Nil && p.ID == ignorePageID {
			continue
		}
		for _, tr := range p.Translations {
			if tr == nil {
				continue
			}
			if tr.LocaleID == localeID && strings.EqualFold(tr.Path, path) {
				return true
			}
		}
	}
	return false
}

func isValidSlug(slug string) bool {
	const pattern = "^[a-z0-9\\-]+$"
	matched, _ := regexp.MatchString(pattern, slug)
	return matched
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func (s *pageService) ensureValidParent(ctx context.Context, pageID uuid.UUID, parentID *uuid.UUID) error {
	if parentID == nil || *parentID == uuid.Nil {
		return nil
	}
	if *parentID == pageID {
		return ErrPageParentCycle
	}

	parent, err := s.pages.GetByID(ctx, *parentID)
	if err != nil {
		var notFound *PageNotFoundError
		if errors.As(err, &notFound) {
			return ErrParentNotFound
		}
		return err
	}

	current := parent.ParentID
	for current != nil && *current != uuid.Nil {
		if *current == pageID {
			return ErrPageParentCycle
		}
		next, err := s.pages.GetByID(ctx, *current)
		if err != nil {
			return err
		}
		current = next.ParentID
	}

	return nil
}

func (s *pageService) generateDuplicateSlug(ctx context.Context, requested, fallback string) (string, error) {
	candidate := strings.TrimSpace(requested)
	if candidate != "" {
		if !isValidSlug(candidate) {
			return "", ErrSlugInvalid
		}
		if _, err := s.pages.GetBySlug(ctx, candidate); err == nil {
			return "", ErrSlugExists
		} else {
			var notFound *PageNotFoundError
			if !errors.As(err, &notFound) {
				return "", err
			}
		}
		return candidate, nil
	}

	base := strings.TrimSpace(fallback)
	if base == "" {
		base = fmt.Sprintf("page-%s", uuid.NewString())
	}

	for attempt := 0; attempt < 100; attempt++ {
		next := appendCopySuffix(base, attempt)
		if _, err := s.pages.GetBySlug(ctx, next); err != nil {
			var notFound *PageNotFoundError
			if errors.As(err, &notFound) {
				return next, nil
			}
			return "", err
		}
	}
	return "", ErrPageDuplicateSlug
}

func appendCopySuffix(base string, attempt int) string {
	suffix := "-copy"
	if attempt > 0 {
		suffix = fmt.Sprintf("-copy-%d", attempt+1)
	}
	return base + suffix
}

func deriveDuplicatePath(existing []*Page, localeID uuid.UUID, base string) string {
	base = sanitizePath(base)
	if base == "" || base == "/" {
		base = "/page"
	}
	attempt := 0
	for {
		candidate := appendPathCopySuffix(base, attempt)
		if !pathExists(existing, localeID, candidate) {
			return candidate
		}
		attempt++
	}
}

func appendPathCopySuffix(path string, attempt int) string {
	suffix := "-copy"
	if attempt > 0 {
		suffix = fmt.Sprintf("-copy-%d", attempt+1)
	}
	if path == "/" {
		return "/" + strings.TrimPrefix(suffix, "-")
	}
	segments := strings.Split(path, "/")
	last := len(segments) - 1
	if last < 0 {
		return "/" + strings.TrimPrefix(suffix, "-")
	}
	if segments[last] == "" {
		last--
	}
	if last < 0 {
		return "/" + strings.TrimPrefix(suffix, "-")
	}
	segments[last] = segments[last] + suffix
	result := strings.Join(segments, "/")
	if !strings.HasPrefix(result, "/") {
		result = "/" + result
	}
	return result
}

func (s *pageService) buildPageTranslations(ctx context.Context, pageID uuid.UUID, inputs []PageTranslationInput, existingPages []*Page, existing []*PageTranslation, now time.Time) ([]*PageTranslation, error) {
	byLocale := indexPageTranslationsByLocaleID(existing)
	seen := map[string]struct{}{}
	result := make([]*PageTranslation, 0, len(inputs))

	for _, input := range inputs {
		code := strings.TrimSpace(input.Locale)
		if code == "" {
			return nil, ErrUnknownLocale
		}
		lower := strings.ToLower(code)
		if _, ok := seen[lower]; ok {
			return nil, ErrDuplicateLocale
		}

		locale, err := s.locales.GetByCode(ctx, code)
		if err != nil {
			return nil, ErrUnknownLocale
		}

		path := sanitizePath(input.Path)
		if path == "" {
			return nil, ErrPathExists
		}
		if pathExistsExcept(existingPages, locale.ID, path, pageID) {
			return nil, ErrPathExists
		}

		if err := validatePageMediaBindings(input.MediaBindings); err != nil {
			return nil, err
		}

		translation := &PageTranslation{
			PageID:        pageID,
			LocaleID:      locale.ID,
			Title:         input.Title,
			Path:          path,
			Summary:       input.Summary,
			MediaBindings: media.CloneBindingSet(input.MediaBindings),
			UpdatedAt:     now,
		}

		if existingTranslation, ok := byLocale[locale.ID]; ok && existingTranslation != nil {
			translation.ID = existingTranslation.ID
			if !existingTranslation.CreatedAt.IsZero() {
				translation.CreatedAt = existingTranslation.CreatedAt
			} else {
				translation.CreatedAt = now
			}
		} else {
			translation.ID = s.id()
			translation.CreatedAt = now
		}

		result = append(result, translation)
		seen[lower] = struct{}{}
	}

	return result, nil
}

func indexPageTranslationsByLocaleID(translations []*PageTranslation) map[uuid.UUID]*PageTranslation {
	if len(translations) == 0 {
		return map[uuid.UUID]*PageTranslation{}
	}
	result := make(map[uuid.UUID]*PageTranslation, len(translations))
	for _, tr := range translations {
		if tr == nil {
			continue
		}
		result[tr.LocaleID] = tr
	}
	return result
}

func nextVersionNumber(records []*PageVersion) int {
	max := 0
	for _, version := range records {
		if version == nil {
			continue
		}
		if version.Version > max {
			max = version.Version
		}
	}

	return max + 1
}
