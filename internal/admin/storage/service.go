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

// ErrPreviewUnsupported indicates the service was constructed without a preview helper.
var ErrPreviewUnsupported = errors.New("adminstorage: preview not supported")

// Validator validates runtime configuration payloads before they are persisted.
type Validator func(runtimeconfig.StorageConfig) error

// PreviewFunc attempts to initialise a storage profile without mutating runtime state.
type PreviewFunc func(ctx context.Context, profile storage.Profile) (PreviewResult, error)

// PreviewResult reports metadata gathered during preview initialisation.
type PreviewResult struct {
	Profile      storage.Profile      `json:"profile"`
	Capabilities storage.Capabilities `json:"capabilities"`
	Diagnostics  map[string]any       `json:"diagnostics,omitempty"`
	Warnings     []string             `json:"warnings,omitempty"`
}

// Schemas exposes JSON schema helpers for admin surfaces.
type Schemas struct {
	Config  string
	Profile string
}

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

// WithValidator overrides the configuration validator used by the service.
func WithValidator(validator Validator) Option {
	return func(s *Service) {
		if validator != nil {
			s.validate = validator
		}
	}
}

// WithPreviewer wires a preview helper used to verify storage profiles before activation.
func WithPreviewer(preview PreviewFunc) Option {
	return func(s *Service) {
		s.preview = preview
	}
}

// Service synchronises runtime storage profiles with the backing repository and emits audit events.
type Service struct {
	repo  storageconfig.Repository
	audit jobs.AuditRecorder

	clock func() time.Time

	validate Validator
	preview  PreviewFunc

	mu      sync.RWMutex
	aliases map[string]string
}

// NewService constructs a storage admin service.
func NewService(repo storageconfig.Repository, recorder jobs.AuditRecorder, opts ...Option) *Service {
	svc := &Service{
		repo:  repo,
		audit: recorder,
		clock: time.Now,
		validate: func(cfg runtimeconfig.StorageConfig) error {
			return cfg.ValidateProfiles()
		},
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
	if err := s.ValidateConfig(cfg); err != nil {
		return err
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

// ListProfiles returns the stored profiles ordered by name.
func (s *Service) ListProfiles(ctx context.Context) ([]storage.Profile, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]storage.Profile, 0, len(list))
	for _, profile := range list {
		out = append(out, cloneProfile(profile))
	}
	return out, nil
}

// GetProfile returns a single profile by name.
func (s *Service) GetProfile(ctx context.Context, name string) (*storage.Profile, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}
	profile, err := s.repo.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	cloned := cloneProfile(*profile)
	return &cloned, nil
}

// ValidateConfig ensures profile collections and aliases are well formed.
func (s *Service) ValidateConfig(cfg runtimeconfig.StorageConfig) error {
	if s.validate == nil {
		if err := cfg.ValidateProfiles(); err != nil {
			return errors.Join(ErrConfigInvalid, err)
		}
		return nil
	}
	if err := s.validate(cfg); err != nil {
		return errors.Join(ErrConfigInvalid, err)
	}
	return nil
}

// ValidateProfile validates a single profile with optional alias mappings.
func (s *Service) ValidateProfile(profile storage.Profile, aliases map[string]string) error {
	cfg := runtimeconfig.StorageConfig{
		Profiles: []storage.Profile{profile},
		Aliases:  aliases,
	}
	return s.ValidateConfig(cfg)
}

// PreviewProfile initialises the supplied profile using the configured previewer.
func (s *Service) PreviewProfile(ctx context.Context, profile storage.Profile) (PreviewResult, error) {
	if s.preview == nil {
		return PreviewResult{}, ErrPreviewUnsupported
	}
	if err := s.ValidateProfile(profile, nil); err != nil {
		return PreviewResult{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := s.preview(ctx, cloneProfile(profile))
	if err != nil {
		return PreviewResult{}, err
	}
	return result, nil
}

// Schemas returns JSON schema helpers for admin integrations.
func (s *Service) Schemas() Schemas {
	return Schemas{
		Config:  storage.ConfigJSONSchema,
		Profile: storage.ProfileJSONSchema,
	}
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

func cloneProfile(profile storage.Profile) storage.Profile {
	cloned := profile
	if profile.Fallbacks != nil {
		cloned.Fallbacks = append([]string(nil), profile.Fallbacks...)
	}
	if profile.Labels != nil {
		cloned.Labels = maps.Clone(profile.Labels)
	}
	if profile.Config.Options != nil {
		cloned.Config.Options = maps.Clone(profile.Config.Options)
	}
	return cloned
}
