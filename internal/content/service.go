package content

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/domain"
	cmsscheduler "github.com/goliatone/go-cms/internal/scheduler"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// Service exposes content management use-cases.
type Service interface {
	Create(ctx context.Context, req CreateContentRequest) (*Content, error)
	Get(ctx context.Context, id uuid.UUID) (*Content, error)
	List(ctx context.Context) ([]*Content, error)
	Schedule(ctx context.Context, req ScheduleContentRequest) (*Content, error)
	CreateDraft(ctx context.Context, req CreateContentDraftRequest) (*ContentVersion, error)
	PublishDraft(ctx context.Context, req PublishContentDraftRequest) (*ContentVersion, error)
	ListVersions(ctx context.Context, contentID uuid.UUID) ([]*ContentVersion, error)
	RestoreVersion(ctx context.Context, req RestoreContentVersionRequest) (*ContentVersion, error)
}

// CreateContentRequest captures the information required to create content.
type CreateContentRequest struct {
	ContentTypeID uuid.UUID
	Slug          string
	Status        string
	CreatedBy     uuid.UUID
	UpdatedBy     uuid.UUID
	Translations  []ContentTranslationInput
}

// ContentTranslationInput represents localized fields supplied during create.
type ContentTranslationInput struct {
	Locale  string
	Title   string
	Summary *string
	Content map[string]any
}

// CreateContentDraftRequest captures the payload needed to record a draft snapshot.
type CreateContentDraftRequest struct {
	ContentID   uuid.UUID
	Snapshot    ContentVersionSnapshot
	CreatedBy   uuid.UUID
	UpdatedBy   uuid.UUID
	BaseVersion *int
}

// PublishContentDraftRequest captures the information required to publish a content draft.
type PublishContentDraftRequest struct {
	ContentID   uuid.UUID
	Version     int
	PublishedBy uuid.UUID
	PublishedAt *time.Time
}

// RestoreContentVersionRequest captures the request to restore a prior content version.
type RestoreContentVersionRequest struct {
	ContentID  uuid.UUID
	Version    int
	RestoredBy uuid.UUID
}

// ScheduleContentRequest captures details to schedule publish/unpublish events.
type ScheduleContentRequest struct {
	ContentID   uuid.UUID
	PublishAt   *time.Time
	UnpublishAt *time.Time
	ScheduledBy uuid.UUID
}

var (
	ErrContentTypeRequired             = errors.New("content: content type does not exist")
	ErrSlugRequired                    = errors.New("content: slug is required")
	ErrSlugInvalid                     = errors.New("content: slug contains invalid characters")
	ErrSlugExists                      = errors.New("content: slug already exists")
	ErrNoTranslations                  = errors.New("content: at least one translation is required")
	ErrDuplicateLocale                 = errors.New("content: duplicate locale provided")
	ErrUnknownLocale                   = errors.New("content: unknown locale")
	ErrContentIDRequired               = errors.New("content: content id required")
	ErrVersioningDisabled              = errors.New("content: versioning feature disabled")
	ErrContentVersionRequired          = errors.New("content: version identifier required")
	ErrContentVersionConflict          = errors.New("content: base version mismatch")
	ErrContentVersionAlreadyPublished  = errors.New("content: version already published")
	ErrContentVersionRetentionExceeded = errors.New("content: version retention limit reached")
	ErrSchedulingDisabled              = errors.New("content: scheduling feature disabled")
	ErrScheduleWindowInvalid           = errors.New("content: publish_at must be before unpublish_at")
	ErrScheduleTimestampInvalid        = errors.New("content: schedule timestamp is invalid")
)

// ContentRepository abstracts storage operations for content entities.
type ContentRepository interface {
	Create(ctx context.Context, record *Content) (*Content, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Content, error)
	GetBySlug(ctx context.Context, slug string) (*Content, error)
	List(ctx context.Context) ([]*Content, error)
	Update(ctx context.Context, record *Content) (*Content, error)
	CreateVersion(ctx context.Context, version *ContentVersion) (*ContentVersion, error)
	ListVersions(ctx context.Context, contentID uuid.UUID) ([]*ContentVersion, error)
	GetVersion(ctx context.Context, contentID uuid.UUID, number int) (*ContentVersion, error)
	GetLatestVersion(ctx context.Context, contentID uuid.UUID) (*ContentVersion, error)
	UpdateVersion(ctx context.Context, version *ContentVersion) (*ContentVersion, error)
}

// ContentTypeRepository resolves content types.
type ContentTypeRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*ContentType, error)
}

// LocaleRepository resolves locales by code.
type LocaleRepository interface {
	GetByCode(ctx context.Context, code string) (*Locale, error)
}

// NotFoundError represents missing records from repository lookups.
type NotFoundError struct {
	Resource string
	Key      string
}

func (e *NotFoundError) Error() string {
	if e.Key == "" {
		return fmt.Sprintf("%s not found", e.Resource)
	}
	return fmt.Sprintf("%s %q not found", e.Resource, e.Key)
}

// ServiceOption configures the service at construction time.
type ServiceOption func(*service)

// WithClock overrides the clock used to stamp records.
func WithClock(clock func() time.Time) ServiceOption {
	return func(s *service) {
		s.now = clock
	}
}

type IDGenerator func() uuid.UUID

func WithIDGenerator(generator IDGenerator) ServiceOption {
	return func(s *service) {
		if generator != nil {
			s.id = generator
		}
	}
}

// WithVersioningEnabled toggles the versioning workflow for the service.
func WithVersioningEnabled(enabled bool) ServiceOption {
	return func(s *service) {
		s.versioningEnabled = enabled
	}
}

// WithVersionRetentionLimit constrains how many versions are retained per content entity.
func WithVersionRetentionLimit(limit int) ServiceOption {
	return func(s *service) {
		if limit < 0 {
			limit = 0
		}
		s.versionRetentionLimit = limit
	}
}

// WithScheduler overrides the scheduler used to register publish/unpublish jobs.
func WithScheduler(scheduler interfaces.Scheduler) ServiceOption {
	return func(svc *service) {
		if scheduler != nil {
			svc.scheduler = scheduler
		}
	}
}

// WithSchedulingEnabled toggles scheduling-related workflows.
func WithSchedulingEnabled(enabled bool) ServiceOption {
	return func(svc *service) {
		svc.schedulingEnabled = enabled
	}
}

// service implements Service.
type service struct {
	contents              ContentRepository
	contentTypes          ContentTypeRepository
	locales               LocaleRepository
	now                   func() time.Time
	id                    IDGenerator
	versioningEnabled     bool
	versionRetentionLimit int
	scheduler             interfaces.Scheduler
	schedulingEnabled     bool
}

// NewService constructs a content service with the required dependencies.
func NewService(contents ContentRepository, types ContentTypeRepository, locales LocaleRepository, opts ...ServiceOption) Service {
	s := &service{
		contents:     contents,
		contentTypes: types,
		locales:      locales,
		now:          time.Now,
		id:           uuid.New,
		scheduler:    cmsscheduler.NewNoOp(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Create orchestrates creation of a new content entry with translations.
func (s *service) Create(ctx context.Context, req CreateContentRequest) (*Content, error) {
	if (req.ContentTypeID == uuid.UUID{}) {
		return nil, ErrContentTypeRequired
	}

	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		return nil, ErrSlugRequired
	}
	if !isValidSlug(slug) {
		return nil, ErrSlugInvalid
	}

	if len(req.Translations) == 0 {
		return nil, ErrNoTranslations
	}

	if _, err := s.contentTypes.GetByID(ctx, req.ContentTypeID); err != nil {
		return nil, ErrContentTypeRequired
	}

	if existing, err := s.contents.GetBySlug(ctx, slug); err == nil && existing != nil {
		return nil, ErrSlugExists
	} else if err != nil {
		var notFound *NotFoundError
		if !errors.As(err, &notFound) {
			return nil, err
		}
	}

	seenLocales := map[string]struct{}{}
	now := s.now()

	record := &Content{
		ID:            s.id(),
		ContentTypeID: req.ContentTypeID,
		Status:        chooseStatus(req.Status),
		Slug:          slug,
		CreatedBy:     req.CreatedBy,
		UpdatedBy:     req.UpdatedBy,
		CreatedAt:     now,
		UpdatedAt:     now,
		Translations:  []*ContentTranslation{},
	}

	for _, tr := range req.Translations {
		code := strings.TrimSpace(tr.Locale)
		if code == "" {
			return nil, ErrUnknownLocale
		}

		if _, ok := seenLocales[code]; ok {
			return nil, ErrDuplicateLocale
		}

		loc, err := s.locales.GetByCode(ctx, code)
		if err != nil {
			return nil, ErrUnknownLocale
		}

		translation := &ContentTranslation{
			ID:        s.id(),
			ContentID: record.ID,
			LocaleID:  loc.ID,
			Title:     tr.Title,
			Summary:   tr.Summary,
			Content:   cloneMap(tr.Content),
			CreatedAt: now,
			UpdatedAt: now,
		}

		record.Translations = append(record.Translations, translation)
		seenLocales[code] = struct{}{}
	}

	created, err := s.contents.Create(ctx, record)
	if err != nil {
		return nil, err
	}

	return s.decorateContent(created), nil
}

// Get fetches content by identifier.
func (s *service) Get(ctx context.Context, id uuid.UUID) (*Content, error) {
	record, err := s.contents.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.decorateContent(record), nil
}

// List returns all content entries.
func (s *service) List(ctx context.Context) ([]*Content, error) {
	records, err := s.contents.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		s.decorateContent(record)
	}
	return records, nil
}

// Schedule registers publish and unpublish windows for a content entry and dispatches scheduler jobs.
func (s *service) Schedule(ctx context.Context, req ScheduleContentRequest) (*Content, error) {
	if !s.schedulingEnabled {
		return nil, ErrSchedulingDisabled
	}
	if req.ContentID == uuid.Nil {
		return nil, ErrContentIDRequired
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

	record, err := s.contents.GetByID(ctx, req.ContentID)
	if err != nil {
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
			payload := map[string]any{"content_id": record.ID.String()}
			if req.ScheduledBy != uuid.Nil {
				payload["scheduled_by"] = req.ScheduledBy.String()
			}
			if _, err := s.scheduler.Enqueue(ctx, interfaces.JobSpec{
				Key:     cmsscheduler.ContentPublishJobKey(record.ID),
				Type:    cmsscheduler.JobTypeContentPublish,
				RunAt:   *record.PublishAt,
				Payload: payload,
			}); err != nil {
				return nil, err
			}
		} else if cancelErr := s.scheduler.CancelByKey(ctx, cmsscheduler.ContentPublishJobKey(record.ID)); cancelErr != nil && !errors.Is(cancelErr, interfaces.ErrJobNotFound) {
			return nil, cancelErr
		}

		if record.UnpublishAt != nil {
			payload := map[string]any{"content_id": record.ID.String()}
			if req.ScheduledBy != uuid.Nil {
				payload["scheduled_by"] = req.ScheduledBy.String()
			}
			if _, err := s.scheduler.Enqueue(ctx, interfaces.JobSpec{
				Key:     cmsscheduler.ContentUnpublishJobKey(record.ID),
				Type:    cmsscheduler.JobTypeContentUnpublish,
				RunAt:   *record.UnpublishAt,
				Payload: payload,
			}); err != nil {
				return nil, err
			}
		} else if cancelErr := s.scheduler.CancelByKey(ctx, cmsscheduler.ContentUnpublishJobKey(record.ID)); cancelErr != nil && !errors.Is(cancelErr, interfaces.ErrJobNotFound) {
			return nil, cancelErr
		}
	}

	updated, err := s.contents.Update(ctx, record)
	if err != nil {
		return nil, err
	}

	return s.decorateContent(updated), nil
}

func (s *service) CreateDraft(ctx context.Context, req CreateContentDraftRequest) (*ContentVersion, error) {
	if !s.versioningEnabled {
		return nil, ErrVersioningDisabled
	}
	if req.ContentID == uuid.Nil {
		return nil, ErrContentIDRequired
	}

	contentRecord, err := s.contents.GetByID(ctx, req.ContentID)
	if err != nil {
		return nil, err
	}

	versions, err := s.contents.ListVersions(ctx, req.ContentID)
	if err != nil {
		return nil, err
	}

	if s.versionRetentionLimit > 0 && len(versions) >= s.versionRetentionLimit {
		return nil, ErrContentVersionRetentionExceeded
	}

	next := nextContentVersionNumber(versions)
	if req.BaseVersion != nil && *req.BaseVersion != next-1 {
		return nil, ErrContentVersionConflict
	}

	now := s.now()
	version := &ContentVersion{
		ID:        s.id(),
		ContentID: req.ContentID,
		Version:   next,
		Status:    domain.StatusDraft,
		Snapshot:  cloneContentVersionSnapshot(req.Snapshot),
		CreatedBy: req.CreatedBy,
		CreatedAt: now,
	}

	created, err := s.contents.CreateVersion(ctx, version)
	if err != nil {
		return nil, err
	}

	contentRecord.CurrentVersion = created.Version
	contentRecord.UpdatedAt = now
	switch {
	case req.UpdatedBy != uuid.Nil:
		contentRecord.UpdatedBy = req.UpdatedBy
	case req.CreatedBy != uuid.Nil:
		contentRecord.UpdatedBy = req.CreatedBy
	}
	if contentRecord.PublishedVersion == nil {
		contentRecord.Status = string(domain.StatusDraft)
	}

	if _, err := s.contents.Update(ctx, contentRecord); err != nil {
		return nil, err
	}

	return cloneContentVersion(created), nil
}

func (s *service) PublishDraft(ctx context.Context, req PublishContentDraftRequest) (*ContentVersion, error) {
	if !s.versioningEnabled {
		return nil, ErrVersioningDisabled
	}
	if req.ContentID == uuid.Nil {
		return nil, ErrContentIDRequired
	}
	if req.Version <= 0 {
		return nil, ErrContentVersionRequired
	}

	contentRecord, err := s.contents.GetByID(ctx, req.ContentID)
	if err != nil {
		return nil, err
	}

	version, err := s.contents.GetVersion(ctx, req.ContentID, req.Version)
	if err != nil {
		return nil, err
	}
	if version.Status == domain.StatusPublished {
		return nil, ErrContentVersionAlreadyPublished
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

	updatedVersion, err := s.contents.UpdateVersion(ctx, version)
	if err != nil {
		return nil, err
	}

	if contentRecord.PublishedVersion != nil && *contentRecord.PublishedVersion != updatedVersion.Version {
		prev, prevErr := s.contents.GetVersion(ctx, req.ContentID, *contentRecord.PublishedVersion)
		if prevErr == nil && prev.Status == domain.StatusPublished {
			prev.Status = domain.StatusArchived
			if _, archiveErr := s.contents.UpdateVersion(ctx, prev); archiveErr != nil {
				return nil, archiveErr
			}
		}
	}

	contentRecord.PublishedVersion = &updatedVersion.Version
	contentRecord.PublishedAt = &publishedAt
	if req.PublishedBy != uuid.Nil {
		contentRecord.PublishedBy = &req.PublishedBy
	}
	contentRecord.Status = string(domain.StatusPublished)
	if updatedVersion.Version > contentRecord.CurrentVersion {
		contentRecord.CurrentVersion = updatedVersion.Version
	}

	contentRecord.UpdatedAt = s.now()
	if req.PublishedBy != uuid.Nil {
		contentRecord.UpdatedBy = req.PublishedBy
	}

	if _, err := s.contents.Update(ctx, contentRecord); err != nil {
		return nil, err
	}

	return cloneContentVersion(updatedVersion), nil
}

func (s *service) ListVersions(ctx context.Context, contentID uuid.UUID) ([]*ContentVersion, error) {
	if !s.versioningEnabled {
		return nil, ErrVersioningDisabled
	}
	if contentID == uuid.Nil {
		return nil, ErrContentIDRequired
	}

	versions, err := s.contents.ListVersions(ctx, contentID)
	if err != nil {
		return nil, err
	}
	return cloneContentVersions(versions), nil
}

func (s *service) RestoreVersion(ctx context.Context, req RestoreContentVersionRequest) (*ContentVersion, error) {
	if !s.versioningEnabled {
		return nil, ErrVersioningDisabled
	}
	if req.ContentID == uuid.Nil {
		return nil, ErrContentIDRequired
	}
	if req.Version <= 0 {
		return nil, ErrContentVersionRequired
	}

	version, err := s.contents.GetVersion(ctx, req.ContentID, req.Version)
	if err != nil {
		return nil, err
	}

	return s.CreateDraft(ctx, CreateContentDraftRequest{
		ContentID:   req.ContentID,
		Snapshot:    cloneContentVersionSnapshot(version.Snapshot),
		CreatedBy:   req.RestoredBy,
		UpdatedBy:   req.RestoredBy,
		BaseVersion: nil,
	})
}

func isValidSlug(slug string) bool {
	const pattern = "^[a-z0-9\\-]+$"
	matched, _ := regexp.MatchString(pattern, slug)
	return matched
}

func chooseStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "draft"
	}
	return strings.ToLower(status)
}

func (s *service) decorateContent(record *Content) *Content {
	if record == nil {
		return nil
	}
	status := effectiveContentStatus(record, s.now())
	record.EffectiveStatus = status
	record.IsVisible = status == domain.StatusPublished
	return record
}

func effectiveContentStatus(record *Content, now time.Time) domain.Status {
	if record == nil {
		return domain.StatusDraft
	}
	status := domain.Status(record.Status)
	if status == "" {
		status = domain.StatusDraft
	}
	if record.UnpublishAt != nil && !record.UnpublishAt.After(now) {
		return domain.StatusArchived
	}
	if record.PublishAt != nil {
		if record.PublishAt.After(now) {
			return domain.StatusScheduled
		}
		return domain.StatusPublished
	}
	if record.PublishedAt != nil && !record.PublishedAt.After(now) {
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

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func nextContentVersionNumber(records []*ContentVersion) int {
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
