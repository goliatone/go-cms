package pages

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/google/uuid"
)

// Service describes page management capabilities.
type Service interface {
	Create(ctx context.Context, req CreatePageRequest) (*Page, error)
	Get(ctx context.Context, id uuid.UUID) (*Page, error)
	List(ctx context.Context) ([]*Page, error)
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
	Locale  string
	Title   string
	Path    string
	Summary *string
}

var (
	ErrContentRequired    = errors.New("pages: content does not exist")
	ErrTemplateRequired   = errors.New("pages: template is required")
	ErrSlugRequired       = errors.New("pages: slug is required")
	ErrSlugInvalid        = errors.New("pages: slug contains invalid characters")
	ErrSlugExists         = errors.New("pages: slug already exists")
	ErrPathExists         = errors.New("pages: translation path already exists")
	ErrUnknownLocale      = errors.New("pages: unknown locale")
	ErrDuplicateLocale    = errors.New("pages: duplicate locale provided")
	ErrParentNotFound     = errors.New("pages: parent page not found")
	ErrNoPageTranslations = errors.New("pages: at least one translation is required")
)

// PageRepository abstracts storage operations for pages.
type PageRepository interface {
	Create(ctx context.Context, record *Page) (*Page, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Page, error)
	GetBySlug(ctx context.Context, slug string) (*Page, error)
	List(ctx context.Context) ([]*Page, error)
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
	pages   PageRepository
	content ContentRepository
	locales LocaleRepository
	now     func() time.Time
	id      IDGenerator
}

// NewService constructs a page service with the required dependencies.
func NewService(pages PageRepository, contentRepo ContentRepository, locales LocaleRepository, opts ...ServiceOption) Service {
	s := &pageService{
		pages:   pages,
		content: contentRepo,
		locales: locales,
		now:     time.Now,
		id:      uuid.New,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
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
		return nil, ErrContentRequired
	}

	if existing, err := s.pages.GetBySlug(ctx, slug); err == nil && existing != nil {
		return nil, ErrSlugExists
	} else if err != nil {
		var notFound *PageNotFoundError
		if !errors.As(err, &notFound) {
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
			return nil, ErrParentNotFound
		}
	}

	existingPages, err := s.pages.List(ctx)
	if err != nil {
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
			return nil, ErrUnknownLocale
		}

		path := sanitizePath(tr.Path)
		if path == "" {
			return nil, ErrPathExists
		}
		if pathExists(existingPages, locale.ID, path) {
			return nil, ErrPathExists
		}

		translation := &PageTranslation{
			ID:        s.id(),
			PageID:    page.ID,
			LocaleID:  locale.ID,
			Title:     tr.Title,
			Path:      path,
			Summary:   tr.Summary,
			CreatedAt: now,
			UpdatedAt: now,
		}

		page.Translations = append(page.Translations, translation)
		seenLocales[code] = struct{}{}
	}

	created, err := s.pages.Create(ctx, page)
	if err != nil {
		return nil, err
	}

	return created, nil
}

// Get fetches a page by identifier.
func (s *pageService) Get(ctx context.Context, id uuid.UUID) (*Page, error) {
	return s.pages.GetByID(ctx, id)
}

// List returns all pages.
func (s *pageService) List(ctx context.Context) ([]*Page, error) {
	return s.pages.List(ctx)
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
