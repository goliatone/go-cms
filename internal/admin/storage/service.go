package storage

import (
	"context"
	"errors"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
	"github.com/goliatone/go-cms/internal/storageconfig"
	"github.com/goliatone/go-cms/pkg/storage"
)

// ErrRepositoryRequired indicates the service was constructed without a repository.
var ErrRepositoryRequired = errors.New("adminstorage: repository is required")

// ErrConfigInvalid indicates configuration validation failed.
var ErrConfigInvalid = errors.New("adminstorage: storage configuration is invalid")

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

// Service synchronises runtime storage profiles with the backing repository and emits audit events.
type Service struct {
	repo  storageconfig.Repository
	audit jobs.AuditRecorder

	clock func() time.Time

	mu      sync.RWMutex
	aliases map[string]string
}

// NewService constructs a storage admin service.
func NewService(repo storageconfig.Repository, recorder jobs.AuditRecorder, opts ...Option) *Service {
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

// ApplyConfig maps the supplied runtime storage configuration to repository writes.
func (s *Service) ApplyConfig(ctx context.Context, cfg runtimeconfig.StorageConfig) error {
	if s.repo == nil {
		return ErrRepositoryRequired
	}
	if err := cfg.ValidateProfiles(); err != nil {
		return errors.Join(ErrConfigInvalid, err)
	}

	existing, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	existingByName := make(map[string]storage.Profile, len(existing))
	for _, profile := range existing {
		existingByName[strings.TrimSpace(profile.Name)] = profile
	}

	for _, profile := range cfg.Profiles {
		name := strings.TrimSpace(profile.Name)
		if name == "" {
			return errors.Join(ErrConfigInvalid, storageconfig.ErrProfileNameRequired)
		}
		_, existed := existingByName[name]
		stored, err := s.repo.Upsert(ctx, profile)
		if err != nil {
			return err
		}
		action := "storage_profile_updated"
		if !existed {
			action = "storage_profile_created"
		}
		s.recordAudit(ctx, jobs.AuditEvent{
			EntityType: "storage_profile",
			EntityID:   stored.Name,
			Action:     action,
			OccurredAt: s.clock(),
			Metadata:   buildProfileMetadata(*stored),
		})
		delete(existingByName, name)
	}

	for name := range existingByName {
		if err := s.repo.Delete(ctx, name); err != nil && !errors.Is(err, storageconfig.ErrProfileNotFound) {
			return err
		}
		s.recordAudit(ctx, jobs.AuditEvent{
			EntityType: "storage_profile",
			EntityID:   name,
			Action:     "storage_profile_deleted",
			OccurredAt: s.clock(),
		})
	}

	s.applyAliases(ctx, cfg.Aliases)

	return nil
}

// Aliases returns a copy of the current alias mapping.
func (s *Service) Aliases() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return maps.Clone(s.aliases)
}

// ResolveAlias resolves an alias to its target profile name.
func (s *Service) ResolveAlias(alias string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	target, ok := s.aliases[alias]
	return target, ok
}

func (s *Service) applyAliases(ctx context.Context, aliases map[string]string) {
	s.mu.Lock()
	if maps.Equal(s.aliases, aliases) {
		s.mu.Unlock()
		return
	}
	cloned := maps.Clone(aliases)
	s.aliases = cloned
	s.mu.Unlock()

	if len(cloned) == 0 && s.audit == nil {
		return
	}

	s.recordAudit(ctx, jobs.AuditEvent{
		EntityType: "storage_profile_aliases",
		EntityID:   "aliases",
		Action:     "storage_profile_aliases_updated",
		OccurredAt: s.clock(),
		Metadata: map[string]any{
			"aliases": cloned,
		},
	})
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

func buildProfileMetadata(profile storage.Profile) map[string]any {
	meta := map[string]any{
		"provider": profile.Provider,
		"default":  profile.Default,
	}
	if len(profile.Fallbacks) > 0 {
		meta["fallbacks"] = append([]string(nil), profile.Fallbacks...)
	}
	if len(profile.Labels) > 0 {
		meta["labels"] = maps.Clone(profile.Labels)
	}
	meta["config"] = map[string]any{
		"name":      profile.Config.Name,
		"driver":    profile.Config.Driver,
		"read_only": profile.Config.ReadOnly,
		"options":   len(profile.Config.Options),
	}
	return meta
}
