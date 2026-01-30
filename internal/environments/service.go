package environments

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service describes environment management capabilities.
type Service interface {
	CreateEnvironment(ctx context.Context, input CreateEnvironmentInput) (*Environment, error)
	UpdateEnvironment(ctx context.Context, input UpdateEnvironmentInput) (*Environment, error)
	DeleteEnvironment(ctx context.Context, id uuid.UUID) error
	GetEnvironment(ctx context.Context, id uuid.UUID) (*Environment, error)
	GetEnvironmentByKey(ctx context.Context, key string) (*Environment, error)
	ListEnvironments(ctx context.Context) ([]*Environment, error)
	ListActiveEnvironments(ctx context.Context) ([]*Environment, error)
	GetDefaultEnvironment(ctx context.Context) (*Environment, error)
}

// CreateEnvironmentInput captures the information required to register an environment.
type CreateEnvironmentInput struct {
	Key         string
	Name        string
	Description *string
	IsActive    *bool
	IsDefault   bool
}

// UpdateEnvironmentInput captures mutable environment fields.
type UpdateEnvironmentInput struct {
	ID          uuid.UUID
	Name        *string
	Description *string
	IsActive    *bool
	IsDefault   *bool
}

var (
	ErrEnvironmentRepositoryRequired = errors.New("environments: repository required")
	ErrEnvironmentKeyRequired        = errors.New("environments: key is required")
	ErrEnvironmentKeyInvalid         = errors.New("environments: key is invalid")
	ErrEnvironmentKeyExists          = errors.New("environments: key already exists")
	ErrEnvironmentNameRequired       = errors.New("environments: name is required")
	ErrEnvironmentNotFound           = errors.New("environments: environment not found")
)

// IDDeriver produces deterministic environment IDs from keys.
type IDDeriver func(key string) uuid.UUID

// ServiceOption configures service behaviour.
type ServiceOption func(*service)

// WithIDDeriver overrides environment ID derivation.
func WithIDDeriver(deriver IDDeriver) ServiceOption {
	return func(s *service) {
		if deriver != nil {
			s.id = deriver
		}
	}
}

// WithNow overrides the time source (primarily for tests).
func WithNow(now func() time.Time) ServiceOption {
	return func(s *service) {
		if now != nil {
			s.now = now
		}
	}
}

type service struct {
	repo EnvironmentRepository
	id   IDDeriver
	now  func() time.Time
}

// NewService constructs an environment service instance.
func NewService(repo EnvironmentRepository, opts ...ServiceOption) Service {
	if repo == nil {
		panic(ErrEnvironmentRepositoryRequired)
	}

	s := &service{
		repo: repo,
		id:   IDForKey,
		now:  time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *service) CreateEnvironment(ctx context.Context, input CreateEnvironmentInput) (*Environment, error) {
	key := normalizeEnvironmentKey(input.Key)
	if key == "" {
		return nil, ErrEnvironmentKeyRequired
	}
	if !environmentKeyPattern.MatchString(key) {
		return nil, ErrEnvironmentKeyInvalid
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = deriveEnvironmentName(key)
	}
	if name == "" {
		return nil, ErrEnvironmentNameRequired
	}

	if existing, err := s.repo.GetByKey(ctx, key); err == nil && existing != nil {
		return nil, ErrEnvironmentKeyExists
	} else if err != nil {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
	}

	now := s.now().UTC()
	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	record := &Environment{
		ID:          s.id(key),
		Key:         key,
		Name:        name,
		Description: cloneString(input.Description),
		IsActive:    isActive,
		IsDefault:   input.IsDefault,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if input.IsDefault {
		record.IsDefault = false
	}

	created, err := s.repo.Create(ctx, record)
	if err != nil {
		return nil, err
	}

	if input.IsDefault {
		if err := s.setDefault(ctx, created.ID); err != nil {
			return nil, err
		}
		created, err = s.repo.GetByID(ctx, created.ID)
		if err != nil {
			return nil, translateRepoError(err, ErrEnvironmentNotFound)
		}
	}

	return cloneEnvironment(created), nil
}

func (s *service) UpdateEnvironment(ctx context.Context, input UpdateEnvironmentInput) (*Environment, error) {
	if input.ID == uuid.Nil {
		return nil, ErrEnvironmentNotFound
	}
	env, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, translateRepoError(err, ErrEnvironmentNotFound)
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, ErrEnvironmentNameRequired
		}
		env.Name = name
	}
	if input.Description != nil {
		desc := strings.TrimSpace(*input.Description)
		if desc == "" {
			env.Description = nil
		} else {
			env.Description = &desc
		}
	}
	if input.IsActive != nil {
		env.IsActive = *input.IsActive
	}
	if input.IsDefault != nil {
		env.IsDefault = *input.IsDefault
	}

	env.UpdatedAt = s.now().UTC()

	if input.IsDefault != nil && *input.IsDefault {
		if err := s.setDefault(ctx, env.ID); err != nil {
			return nil, err
		}
		refreshed, err := s.repo.GetByID(ctx, env.ID)
		if err != nil {
			return nil, translateRepoError(err, ErrEnvironmentNotFound)
		}
		return cloneEnvironment(refreshed), nil
	}

	updated, err := s.repo.Update(ctx, env)
	if err != nil {
		return nil, err
	}
	return cloneEnvironment(updated), nil
}

func (s *service) DeleteEnvironment(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrEnvironmentNotFound
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return translateRepoError(err, ErrEnvironmentNotFound)
	}
	return nil
}

func (s *service) GetEnvironment(ctx context.Context, id uuid.UUID) (*Environment, error) {
	if id == uuid.Nil {
		return nil, ErrEnvironmentNotFound
	}
	env, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, translateRepoError(err, ErrEnvironmentNotFound)
	}
	return cloneEnvironment(env), nil
}

func (s *service) GetEnvironmentByKey(ctx context.Context, key string) (*Environment, error) {
	key = normalizeEnvironmentKey(key)
	if key == "" {
		return nil, ErrEnvironmentNotFound
	}
	env, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return nil, translateRepoError(err, ErrEnvironmentNotFound)
	}
	return cloneEnvironment(env), nil
}

func (s *service) ListEnvironments(ctx context.Context) ([]*Environment, error) {
	records, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	return cloneEnvironmentSlice(records), nil
}

func (s *service) ListActiveEnvironments(ctx context.Context) ([]*Environment, error) {
	records, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	return cloneEnvironmentSlice(records), nil
}

func (s *service) GetDefaultEnvironment(ctx context.Context) (*Environment, error) {
	env, err := s.repo.GetDefault(ctx)
	if err != nil {
		return nil, translateRepoError(err, ErrEnvironmentNotFound)
	}
	return cloneEnvironment(env), nil
}

func (s *service) setDefault(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrEnvironmentNotFound
	}
	current, err := s.repo.GetDefault(ctx)
	if err != nil {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return err
		}
	}
	if current != nil && current.ID == id {
		return nil
	}
	if current != nil {
		current.IsDefault = false
		current.UpdatedAt = s.now().UTC()
		if _, err := s.repo.Update(ctx, current); err != nil {
			return err
		}
	}
	env, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return translateRepoError(err, ErrEnvironmentNotFound)
	}
	env.IsDefault = true
	env.UpdatedAt = s.now().UTC()
	if _, err := s.repo.Update(ctx, env); err != nil {
		return err
	}
	return nil
}

func translateRepoError(err error, fallback error) error {
	if err == nil {
		return nil
	}
	var nf *NotFoundError
	if errors.As(err, &nf) {
		return fallback
	}
	return err
}

func environmentIDForKey(key string) uuid.UUID {
	return IDForKey(key)
}
