package content

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/schema"
	"github.com/goliatone/go-slug"
	"github.com/google/uuid"
)

// ContentTypeService provides CRUD operations for content types.
type ContentTypeService interface {
	Create(ctx context.Context, req CreateContentTypeRequest) (*ContentType, error)
	Update(ctx context.Context, req UpdateContentTypeRequest) (*ContentType, error)
	Delete(ctx context.Context, req DeleteContentTypeRequest) error
	Get(ctx context.Context, id uuid.UUID) (*ContentType, error)
	GetBySlug(ctx context.Context, slug string) (*ContentType, error)
	List(ctx context.Context) ([]*ContentType, error)
	Search(ctx context.Context, query string) ([]*ContentType, error)
}

// CreateContentTypeRequest captures required fields to create a content type.
type CreateContentTypeRequest struct {
	Name         string
	Slug         string
	Description  *string
	Schema       map[string]any
	Capabilities map[string]any
	Icon         *string
}

// UpdateContentTypeRequest captures mutable fields for a content type.
type UpdateContentTypeRequest struct {
	ID           uuid.UUID
	Name         *string
	Slug         *string
	Description  *string
	Schema       map[string]any
	Capabilities map[string]any
	Icon         *string
}

// DeleteContentTypeRequest captures details required to delete a content type.
type DeleteContentTypeRequest struct {
	ID         uuid.UUID
	HardDelete bool
}

var (
	ErrContentTypeNameRequired   = errors.New("content type: name is required")
	ErrContentTypeSchemaRequired = errors.New("content type: schema is required")
	ErrContentTypeIDRequired     = errors.New("content type: id required")
	ErrContentTypeSlugInvalid    = errors.New("content type: slug contains invalid characters")
	ErrContentTypeSchemaVersion  = errors.New("content type: schema version invalid")
)

// ContentTypeOption mutates the content type service.
type ContentTypeOption func(*contentTypeService)

// WithContentTypeClock overrides the clock used by the service.
func WithContentTypeClock(clock func() time.Time) ContentTypeOption {
	return func(s *contentTypeService) {
		if clock != nil {
			s.now = clock
		}
	}
}

// WithContentTypeIDGenerator overrides the ID generator used by the service.
func WithContentTypeIDGenerator(generator IDGenerator) ContentTypeOption {
	return func(s *contentTypeService) {
		if generator != nil {
			s.id = generator
		}
	}
}

// WithContentTypeSlugNormalizer overrides the slug normalizer used by the service.
func WithContentTypeSlugNormalizer(normalizer slug.Normalizer) ContentTypeOption {
	return func(s *contentTypeService) {
		if normalizer != nil {
			s.slugger = normalizer
		}
	}
}

// NewContentTypeService constructs a content type service.
func NewContentTypeService(repo ContentTypeRepository, opts ...ContentTypeOption) ContentTypeService {
	svc := &contentTypeService{
		repo:    repo,
		now:     func() time.Time { return time.Now().UTC() },
		id:      uuid.New,
		slugger: slug.Default(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

type contentTypeService struct {
	repo    ContentTypeRepository
	now     func() time.Time
	id      IDGenerator
	slugger slug.Normalizer
}

func (s *contentTypeService) Create(ctx context.Context, req CreateContentTypeRequest) (*ContentType, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("content type service unavailable")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrContentTypeNameRequired
	}
	if req.Schema == nil {
		return nil, ErrContentTypeSchemaRequired
	}

	record := &ContentType{
		ID:           s.id(),
		Name:         name,
		Slug:         strings.TrimSpace(req.Slug),
		Description:  req.Description,
		Schema:       cloneMap(req.Schema),
		Capabilities: cloneMap(req.Capabilities),
		Icon:         req.Icon,
		CreatedAt:    s.now(),
		UpdatedAt:    s.now(),
	}

	var err error
	record.Slug, err = s.normalizeSlug(record)
	if err != nil {
		return nil, err
	}

	if err := s.ensureSlugAvailable(ctx, record.Slug, record.ID); err != nil {
		return nil, err
	}

	normalizedSchema, _, err := schema.EnsureSchemaVersion(record.Schema, record.Slug)
	if err != nil {
		return nil, ErrContentTypeSchemaVersion
	}
	record.Schema = normalizedSchema

	created, err := s.repo.Create(ctx, record)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *contentTypeService) Update(ctx context.Context, req UpdateContentTypeRequest) (*ContentType, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("content type service unavailable")
	}
	if req.ID == uuid.Nil {
		return nil, ErrContentTypeIDRequired
	}

	record, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		record.Name = strings.TrimSpace(*req.Name)
	}
	if req.Slug != nil {
		record.Slug = strings.TrimSpace(*req.Slug)
	}
	if req.Description != nil {
		record.Description = req.Description
	}
	if req.Schema != nil {
		record.Schema = cloneMap(req.Schema)
	}
	if req.Capabilities != nil {
		record.Capabilities = cloneMap(req.Capabilities)
	}
	if req.Icon != nil {
		record.Icon = req.Icon
	}

	record.Name = strings.TrimSpace(record.Name)
	if record.Name == "" {
		return nil, ErrContentTypeNameRequired
	}
	if record.Schema == nil {
		return nil, ErrContentTypeSchemaRequired
	}

	record.Slug, err = s.normalizeSlug(record)
	if err != nil {
		return nil, err
	}

	if err := s.ensureSlugAvailable(ctx, record.Slug, record.ID); err != nil {
		return nil, err
	}

	normalizedSchema, _, err := schema.EnsureSchemaVersion(record.Schema, record.Slug)
	if err != nil {
		return nil, ErrContentTypeSchemaVersion
	}
	record.Schema = normalizedSchema

	record.UpdatedAt = s.now()
	updated, err := s.repo.Update(ctx, record)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *contentTypeService) Delete(ctx context.Context, req DeleteContentTypeRequest) error {
	if s == nil || s.repo == nil {
		return errors.New("content type service unavailable")
	}
	if req.ID == uuid.Nil {
		return ErrContentTypeIDRequired
	}
	return s.repo.Delete(ctx, req.ID, req.HardDelete)
}

func (s *contentTypeService) Get(ctx context.Context, id uuid.UUID) (*ContentType, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("content type service unavailable")
	}
	if id == uuid.Nil {
		return nil, ErrContentTypeIDRequired
	}
	return s.repo.GetByID(ctx, id)
}

func (s *contentTypeService) GetBySlug(ctx context.Context, rawSlug string) (*ContentType, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("content type service unavailable")
	}
	slugValue := strings.TrimSpace(rawSlug)
	if slugValue == "" {
		return nil, ErrContentTypeSlugRequired
	}
	if s.slugger == nil {
		s.slugger = slug.Default()
	}
	normalized, err := s.slugger.Normalize(slugValue)
	if err != nil || normalized == "" {
		return nil, ErrContentTypeSlugInvalid
	}
	slugValue = normalized
	return s.repo.GetBySlug(ctx, slugValue)
}

func (s *contentTypeService) List(ctx context.Context) ([]*ContentType, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("content type service unavailable")
	}
	return s.repo.List(ctx)
}

func (s *contentTypeService) Search(ctx context.Context, query string) ([]*ContentType, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("content type service unavailable")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return s.repo.List(ctx)
	}
	return s.repo.Search(ctx, query)
}

func (s *contentTypeService) normalizeSlug(ct *ContentType) (string, error) {
	if ct == nil {
		return "", ErrContentTypeSlugRequired
	}
	if s.slugger == nil {
		s.slugger = slug.Default()
	}
	candidate := strings.TrimSpace(ct.Slug)
	if candidate == "" {
		candidate = strings.TrimSpace(extractSchemaSlug(ct.Schema))
	}
	if candidate == "" {
		candidate = strings.TrimSpace(ct.Name)
	}
	if candidate == "" {
		return "", ErrContentTypeSlugRequired
	}
	normalized, err := s.slugger.Normalize(candidate)
	if err != nil || normalized == "" {
		return "", ErrContentTypeSlugRequired
	}
	if !slug.IsValid(normalized) {
		return "", ErrContentTypeSlugInvalid
	}
	return normalized, nil
}

func (s *contentTypeService) ensureSlugAvailable(ctx context.Context, slug string, currentID uuid.UUID) error {
	existing, err := s.repo.GetBySlug(ctx, slug)
	if err == nil && existing != nil {
		if existing.ID != currentID {
			return ErrContentTypeSlugExists
		}
		return nil
	}
	var notFound *NotFoundError
	if err != nil && !errors.As(err, &notFound) {
		return err
	}
	return nil
}
