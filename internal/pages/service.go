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
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// Service describes page management capabilities.
type Service interface {
	Create(ctx context.Context, req CreatePageRequest) (*Page, error)
	Get(ctx context.Context, id uuid.UUID) (*Page, error)
	List(ctx context.Context) ([]*Page, error)
	Schedule(ctx context.Context, req SchedulePageRequest) (*Page, error)
	CreateDraft(ctx context.Context, req CreatePageDraftRequest) (*PageVersion, error)
	PublishDraft(ctx context.Context, req PublishPagePublishRequest) (*PageVersion, error)
	ListVersions(ctx context.Context, pageID uuid.UUID) ([]*PageVersion, error)
	RestoreVersion(ctx context.Context, req RestorePageVersionRequest) (*PageVersion, error)
}

// CreatePageRequest captures the payload required to create a page.
type CreatePageRequest struct {
	ContentID    uuid.UUID
	TemplateID   uuid.UUID
	ParentID     *uuid.UUID
	Slug         string
	Status       string
	CreatedBy    uuid.UUID
	UpdatedBy    uuid.UUID
	Translations []PageTranslationInput
}

// PageTranslationInput represents localized routing information.
type PageTranslationInput struct {
	Locale        string
	Title         string
	Path          string
	Summary       *string
	MediaBindings media.BindingSet
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
)

// PageRepository abstracts storage operations for pages.
type PageRepository interface {
	Create(ctx context.Context, record *Page) (*Page, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Page, error)
	GetBySlug(ctx context.Context, slug string) (*Page, error)
	List(ctx context.Context) ([]*Page, error)
	Update(ctx context.Context, record *Page) (*Page, error)
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

func WithMediaService(svc media.Service) ServiceOption {
	return func(ps *pageService) {
		if svc != nil {
			ps.media = svc
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
		pages:     pages,
		content:   contentRepo,
		locales:   locales,
		now:       time.Now,
		id:        uuid.New,
		media:     media.NewNoOpService(),
		scheduler: cmsscheduler.NewNoOp(),
		logger:    logging.PagesLogger(nil),
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

	if len(req.Translations) == 0 {
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
		Status:       chooseStatus(req.Status),
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

	existingPages, err := s.pages.List(ctx)
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

	if record.PublishAt != nil {
		record.Status = string(domain.StatusScheduled)
	} else if record.PublishedVersion != nil {
		record.Status = string(domain.StatusPublished)
	} else {
		record.Status = string(domain.StatusDraft)
	}

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
		page.Status = string(domain.StatusDraft)
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
	page.Status = string(domain.StatusPublished)
	if updatedVersion.Version > page.CurrentVersion {
		page.CurrentVersion = updatedVersion.Version
	}
	page.UpdatedAt = s.now()
	if req.PublishedBy != uuid.Nil {
		page.UpdatedBy = req.PublishedBy
	}

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
	status := domain.Status(page.Status)
	if status == "" {
		status = domain.StatusDraft
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

func chooseStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "draft"
	}
	return strings.ToLower(status)
}

func isValidSlug(slug string) bool {
	const pattern = "^[a-z0-9\\-]+$"
	matched, _ := regexp.MatchString(pattern, slug)
	return matched
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
