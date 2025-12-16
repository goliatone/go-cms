package translations

import (
	"context"
	"errors"
	"time"

	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/translationconfig"
)

// ErrRepositoryRequired indicates the service was constructed without a repository.
var ErrRepositoryRequired = errors.New("admintranslations: repository is required")

// Option mutates the service configuration.
type Option func(*Service)

// WithClock overrides the clock used for audit timestamps.
func WithClock(clock func() time.Time) Option {
	return func(s *Service) {
		if clock != nil {
			s.clock = clock
		}
	}
}

// WithAuditRecorder overrides the audit recorder dependency.
func WithAuditRecorder(recorder jobs.AuditRecorder) Option {
	return func(s *Service) {
		s.audit = recorder
	}
}

// Service persists translation settings and emits audit records.
type Service struct {
	repo  translationconfig.Repository
	audit jobs.AuditRecorder
	clock func() time.Time
}

// NewService constructs a translations admin service.
func NewService(repo translationconfig.Repository, recorder jobs.AuditRecorder, opts ...Option) *Service {
	svc := &Service{
		repo:  repo,
		audit: recorder,
		clock: time.Now,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// GetSettings returns the stored translation settings.
func (s *Service) GetSettings(ctx context.Context) (translationconfig.Settings, error) {
	if s.repo == nil {
		return translationconfig.Settings{}, ErrRepositoryRequired
	}
	return s.repo.Get(ctx)
}

// ApplySettings stores translation settings and records an audit entry.
func (s *Service) ApplySettings(ctx context.Context, settings translationconfig.Settings) error {
	if s.repo == nil {
		return ErrRepositoryRequired
	}
	if ctx == nil {
		ctx = context.Background()
	}

	action := "translation_settings_updated"
	if _, err := s.repo.Get(ctx); err != nil {
		if errors.Is(err, translationconfig.ErrSettingsNotFound) {
			action = "translation_settings_created"
		} else {
			return err
		}
	}

	stored, err := s.repo.Upsert(ctx, settings)
	if err != nil {
		return err
	}

	s.recordAudit(ctx, jobs.AuditEvent{
		EntityType: "translation_settings",
		EntityID:   "global",
		Action:     action,
		OccurredAt: s.clock(),
		Metadata: map[string]any{
			"translations_enabled": stored.TranslationsEnabled,
			"require_translations": stored.RequireTranslations,
		},
	})
	return nil
}

// Reset clears translation settings from the repository.
func (s *Service) Reset(ctx context.Context) error {
	if s.repo == nil {
		return ErrRepositoryRequired
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := s.repo.Delete(ctx); err != nil {
		return err
	}

	s.recordAudit(ctx, jobs.AuditEvent{
		EntityType: "translation_settings",
		EntityID:   "global",
		Action:     "translation_settings_deleted",
		OccurredAt: s.clock(),
	})
	return nil
}

func (s *Service) recordAudit(ctx context.Context, event jobs.AuditEvent) {
	if s.audit == nil {
		return
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = s.clock()
	}
	_ = s.audit.Record(ctx, event)
}
