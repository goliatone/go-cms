package content

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service exposes content management use-cases.
type Service interface {
	Create(ctx context.Context, req CreateContentRequest) (*Content, error)
	Get(ctx context.Context, id uuid.UUID) (*Content, error)
	List(ctx context.Context) ([]*Content, error)
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

var (
	ErrContentTypeRequired = errors.New("content: content type does not exist")
	ErrSlugRequired        = errors.New("content: slug is required")
	ErrSlugInvalid         = errors.New("content: slug contains invalid characters")
	ErrSlugExists          = errors.New("content: slug already exists")
	ErrNoTranslations      = errors.New("content: at least one translation is required")
	ErrDuplicateLocale     = errors.New("content: duplicate locale provided")
	ErrUnknownLocale       = errors.New("content: unknown locale")
)

// ContentRepository abstracts storage operations for content entities.
type ContentRepository interface {
	Create(ctx context.Context, record *Content) (*Content, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Content, error)
	GetBySlug(ctx context.Context, slug string) (*Content, error)
	List(ctx context.Context) ([]*Content, error)
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

// service implements Service.
type service struct {
	contents     ContentRepository
	contentTypes ContentTypeRepository
	locales      LocaleRepository
	now          func() time.Time
}

// NewService constructs a content service with the required dependencies.
func NewService(contents ContentRepository, types ContentTypeRepository, locales LocaleRepository, opts ...ServiceOption) Service {
	s := &service{
		contents:     contents,
		contentTypes: types,
		locales:      locales,
		now:          time.Now,
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
	} else if err != nil && !errors.As(err, &NotFoundError{}) {
		return nil, err
	}

	seenLocales := map[string]struct{}{}
	now := s.now()

	record := &Content{
		ID:            uuid.New(),
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
			ID:        uuid.New(),
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

	return created, nil
}

// Get fetches content by identifier.
func (s *service) Get(ctx context.Context, id uuid.UUID) (*Content, error) {
	return s.contents.GetByID(ctx, id)
}

// List returns all content entries.
func (s *service) List(ctx context.Context) ([]*Content, error) {
	return s.contents.List(ctx)
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

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	maps.Copy(out, src)
	return out
}
