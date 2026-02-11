package content

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/domain"
	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/logging"
	cmsscheduler "github.com/goliatone/go-cms/internal/scheduler"
	cmsschema "github.com/goliatone/go-cms/internal/schema"
	"github.com/goliatone/go-cms/internal/translationconfig"
	"github.com/goliatone/go-cms/internal/validation"
	"github.com/goliatone/go-cms/pkg/activity"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-slug"
	"github.com/google/uuid"
)

// ContentRepository abstracts storage operations for content entities.
type ContentRepository interface {
	Create(ctx context.Context, record *Content) (*Content, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Content, error)
	GetBySlug(ctx context.Context, slug string, contentTypeID uuid.UUID, env ...string) (*Content, error)
	List(ctx context.Context, env ...ContentListOption) ([]*Content, error)
	Update(ctx context.Context, record *Content) (*Content, error)
	ReplaceTranslations(ctx context.Context, contentID uuid.UUID, translations []*ContentTranslation) error
	Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error
	CreateVersion(ctx context.Context, version *ContentVersion) (*ContentVersion, error)
	ListVersions(ctx context.Context, contentID uuid.UUID) ([]*ContentVersion, error)
	GetVersion(ctx context.Context, contentID uuid.UUID, number int) (*ContentVersion, error)
	GetLatestVersion(ctx context.Context, contentID uuid.UUID) (*ContentVersion, error)
	UpdateVersion(ctx context.Context, version *ContentVersion) (*ContentVersion, error)
}

// ContentTypeRepository resolves content types.
type ContentTypeRepository interface {
	Create(ctx context.Context, record *ContentType) (*ContentType, error)
	GetByID(ctx context.Context, id uuid.UUID) (*ContentType, error)
	GetBySlug(ctx context.Context, slug string, env ...string) (*ContentType, error)
	List(ctx context.Context, env ...string) ([]*ContentType, error)
	Search(ctx context.Context, query string, env ...string) ([]*ContentType, error)
	Update(ctx context.Context, record *ContentType) (*ContentType, error)
	Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error
}

// LocaleRepository resolves locales by code.
type LocaleRepository interface {
	GetByCode(ctx context.Context, code string) (*Locale, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Locale, error)
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

// IDGenerator returns the identifier used for newly created records.
type IDGenerator func() uuid.UUID

// WithIDGenerator overrides the generator used to create identifiers.
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

// WithSlugNormalizer overrides the slug normalizer used by the service.
func WithSlugNormalizer(normalizer slug.Normalizer) ServiceOption {
	return func(s *service) {
		if normalizer != nil {
			s.slugger = normalizer
		}
	}
}

// WithSchemaMigrator configures schema migrations for publish-time upgrades.
func WithSchemaMigrator(migrator *cmsschema.Migrator) ServiceOption {
	return func(s *service) {
		if migrator != nil {
			s.schemaMigrator = migrator
		}
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

// WithLogger assigns the logger used by the service. When omitted, a no-op logger is used.
func WithLogger(logger interfaces.Logger) ServiceOption {
	return func(svc *service) {
		if logger != nil {
			svc.logger = logger
		}
	}
}

// WithRequireTranslations controls whether translations are mandatory.
func WithRequireTranslations(required bool) ServiceOption {
	return func(svc *service) {
		svc.requireTranslations = required
	}
}

// WithTranslationsEnabled toggles translation handling.
func WithTranslationsEnabled(enabled bool) ServiceOption {
	return func(svc *service) {
		svc.translationsEnabled = enabled
	}
}

// WithTranslationState wires a shared, runtime-configurable translation state.
func WithTranslationState(state *translationconfig.State) ServiceOption {
	return func(svc *service) {
		svc.translationState = state
	}
}

// WithDefaultLocale sets the locale required for default fallback handling.
func WithDefaultLocale(locale string, required bool) ServiceOption {
	return func(svc *service) {
		svc.defaultLocale = strings.TrimSpace(locale)
		svc.defaultLocaleRequired = required
	}
}

// WithActivityEmitter wires the activity emitter used for activity records.
func WithActivityEmitter(emitter *activity.Emitter) ServiceOption {
	return func(svc *service) {
		if emitter != nil {
			svc.activity = emitter
		}
	}
}

// WithEnvironmentService wires the environment service for env resolution.
func WithEnvironmentService(envSvc cmsenv.Service) ServiceOption {
	return func(svc *service) {
		if envSvc != nil {
			svc.envSvc = envSvc
		}
	}
}

// WithDefaultEnvironmentKey overrides the default environment key.
func WithDefaultEnvironmentKey(key string) ServiceOption {
	return func(svc *service) {
		if strings.TrimSpace(key) != "" {
			svc.defaultEnvKey = key
		}
	}
}

// WithRequireExplicitEnvironment enforces explicit environment selection.
func WithRequireExplicitEnvironment(required bool) ServiceOption {
	return func(svc *service) {
		svc.requireExplicitEnv = required
	}
}

// WithRequireActiveEnvironment blocks operations on inactive environments.
func WithRequireActiveEnvironment(required bool) ServiceOption {
	return func(svc *service) {
		svc.requireActiveEnv = required
	}
}

// WithEmbeddedBlocksResolver wires the embedded blocks bridge (dual-write + fallback).
func WithEmbeddedBlocksResolver(resolver EmbeddedBlocksResolver) ServiceOption {
	return func(svc *service) {
		if resolver != nil {
			svc.embeddedBlocks = resolver
		}
	}
}

// WithProjectionTranslationMode configures projection behavior when translations
// are not explicitly requested for reads.
func WithProjectionTranslationMode(mode ProjectionTranslationMode) ServiceOption {
	return func(svc *service) {
		if normalized, ok := normalizeProjectionTranslationMode(mode); ok {
			svc.projectionTranslationMode = normalized
		}
	}
}

// service implements Service.
type service struct {
	contents                  ContentRepository
	contentTypes              ContentTypeRepository
	locales                   LocaleRepository
	now                       func() time.Time
	id                        IDGenerator
	slugger                   slug.Normalizer
	versioningEnabled         bool
	versionRetentionLimit     int
	scheduler                 interfaces.Scheduler
	schedulingEnabled         bool
	logger                    interfaces.Logger
	requireTranslations       bool
	translationsEnabled       bool
	translationState          *translationconfig.State
	defaultLocale             string
	defaultLocaleRequired     bool
	activity                  *activity.Emitter
	schemaMigrator            *cmsschema.Migrator
	embeddedBlocks            EmbeddedBlocksResolver
	envSvc                    cmsenv.Service
	defaultEnvKey             string
	requireExplicitEnv        bool
	requireActiveEnv          bool
	projectionTranslationMode ProjectionTranslationMode
}

// NewService constructs a content service with the required dependencies.
func NewService(contents ContentRepository, types ContentTypeRepository, locales LocaleRepository, opts ...ServiceOption) Service {
	s := &service{
		contents:                  contents,
		contentTypes:              types,
		locales:                   locales,
		now:                       time.Now,
		id:                        uuid.New,
		slugger:                   slug.Default(),
		scheduler:                 cmsscheduler.NewNoOp(),
		logger:                    logging.ContentLogger(nil),
		requireTranslations:       true,
		translationsEnabled:       true,
		activity:                  activity.NewEmitter(nil, activity.Config{}),
		defaultEnvKey:             cmsenv.DefaultKey,
		projectionTranslationMode: ProjectionTranslationModeAutoLoad,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *service) log(ctx context.Context) interfaces.Logger {
	if ctx == nil {
		return s.logger
	}
	return s.logger.WithContext(ctx)
}

func (s *service) opLogger(ctx context.Context, operation string, extra map[string]any) interfaces.Logger {
	fields := map[string]any{"operation": operation}
	for key, value := range extra {
		fields[key] = value
	}
	return logging.WithFields(s.log(ctx), fields)
}

func (s *service) translationsRequired() bool {
	enabled := s.translationsEnabled
	required := s.requireTranslations
	if s.translationState != nil {
		enabled = s.translationState.Enabled()
		required = s.translationState.RequireTranslations()
	}
	return enabled && required
}

func (s *service) translationsEnabledFlag() bool {
	if s.translationState != nil {
		return s.translationState.Enabled()
	}
	return s.translationsEnabled
}

func (s *service) defaultLocaleKey() string {
	return strings.ToLower(strings.TrimSpace(s.defaultLocale))
}

func (s *service) requiresDefaultLocale() bool {
	return s.defaultLocaleRequired && s.translationsRequired() && s.defaultLocaleKey() != ""
}

func (s *service) hasDefaultLocale(inputs []ContentTranslationInput) bool {
	target := s.defaultLocaleKey()
	for _, input := range inputs {
		if strings.ToLower(strings.TrimSpace(input.Locale)) == target {
			return true
		}
	}
	return false
}

func (s *service) isDefaultLocale(code string) bool {
	return strings.ToLower(strings.TrimSpace(code)) == s.defaultLocaleKey()
}

func (s *service) resolveEnvironment(ctx context.Context, key string) (uuid.UUID, string, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" && s.requireExplicitEnv {
		return uuid.Nil, "", cmsenv.ErrEnvironmentKeyRequired
	}
	if trimmed != "" {
		if parsed, err := uuid.Parse(trimmed); err == nil {
			return parsed, trimmed, nil
		}
	}
	normalized, err := cmsenv.ResolveKey(trimmed, s.defaultEnvKey, s.requireExplicitEnv)
	if err != nil {
		return uuid.Nil, "", err
	}
	if s.envSvc == nil {
		return cmsenv.IDForKey(normalized), normalized, nil
	}
	env, err := s.envSvc.GetEnvironmentByKey(ctx, normalized)
	if err != nil {
		return uuid.Nil, "", err
	}
	if s.requireActiveEnv && !env.IsActive {
		return uuid.Nil, "", cmsenv.ErrEnvironmentNotFound
	}
	return env.ID, env.Key, nil
}

func (s *service) ensureEnvironmentActive(ctx context.Context, envID uuid.UUID) error {
	if !s.requireActiveEnv || s.envSvc == nil || envID == uuid.Nil {
		return nil
	}
	env, err := s.envSvc.GetEnvironment(ctx, envID)
	if err != nil {
		return err
	}
	if !env.IsActive {
		return cmsenv.ErrEnvironmentNotFound
	}
	return nil
}

func (s *service) environmentKeyForID(ctx context.Context, envID uuid.UUID) string {
	if envID == uuid.Nil {
		return ""
	}
	if s.envSvc != nil {
		env, err := s.envSvc.GetEnvironment(ctx, envID)
		if err == nil && env != nil {
			return env.Key
		}
	}
	defaultKey := strings.TrimSpace(s.defaultEnvKey)
	if defaultKey == "" {
		defaultKey = cmsenv.DefaultKey
	}
	if envID == cmsenv.IDForKey(defaultKey) {
		return cmsenv.NormalizeKey(defaultKey)
	}
	if envID == cmsenv.IDForKey(cmsenv.DefaultKey) {
		return cmsenv.DefaultKey
	}
	return ""
}

func (s *service) emitActivity(ctx context.Context, actor uuid.UUID, verb, objectType string, objectID uuid.UUID, meta map[string]any) {
	if s.activity == nil || !s.activity.Enabled() || objectID == uuid.Nil {
		return
	}
	if meta != nil {
		if _, ok := meta["environment_key"]; !ok {
			if raw, ok := meta["environment_id"].(string); ok && strings.TrimSpace(raw) != "" {
				if parsed, err := uuid.Parse(raw); err == nil {
					if key := s.environmentKeyForID(ctx, parsed); key != "" {
						meta["environment_key"] = key
					}
				}
			}
		}
	}
	event := activity.Event{
		Verb:       verb,
		ActorID:    actor.String(),
		ObjectType: objectType,
		ObjectID:   objectID.String(),
		Metadata:   meta,
	}
	if err := s.activity.Emit(ctx, event); err != nil {
		s.log(ctx).Warn("content.activity_emit_failed", "error", err)
	}
}

// Create orchestrates creation of a new content entry with translations.
func (s *service) Create(ctx context.Context, req CreateContentRequest) (*Content, error) {
	if (req.ContentTypeID == uuid.UUID{}) {
		return nil, ErrContentTypeRequired
	}

	rawSlug := strings.TrimSpace(req.Slug)
	logger := s.opLogger(ctx, "content.create", map[string]any{
		"content_type_id": req.ContentTypeID,
		"slug":            rawSlug,
	})

	if rawSlug == "" {
		return nil, ErrSlugRequired
	}
	slugValue, err := s.slugger.Normalize(rawSlug)
	if err != nil || !slug.IsValid(slugValue) {
		return nil, ErrSlugInvalid
	}

	if s.translationsRequired() && len(req.Translations) == 0 && !req.AllowMissingTranslations {
		return nil, ErrNoTranslations
	}
	if s.requiresDefaultLocale() && !req.AllowMissingTranslations && len(req.Translations) > 0 && !s.hasDefaultLocale(req.Translations) {
		return nil, ErrDefaultLocaleRequired
	}

	contentType, err := s.contentTypes.GetByID(ctx, req.ContentTypeID)
	if err != nil {
		logger.Debug("content type lookup failed", "error", err)
		return nil, ErrContentTypeRequired
	}
	if err := s.ensureEnvironmentActive(ctx, contentType.EnvironmentID); err != nil {
		return nil, err
	}
	var resolvedEnvID uuid.UUID
	if strings.TrimSpace(req.EnvironmentKey) != "" || s.requireExplicitEnv {
		envID, _, err := s.resolveEnvironment(ctx, req.EnvironmentKey)
		if err != nil {
			return nil, err
		}
		resolvedEnvID = envID
		if contentType.EnvironmentID != uuid.Nil && contentType.EnvironmentID != envID {
			return nil, cmsenv.ErrEnvironmentNotFound
		}
	}
	envID := contentType.EnvironmentID
	if envID == uuid.Nil {
		if resolvedEnvID != uuid.Nil {
			envID = resolvedEnvID
		} else {
			resolvedID, _, err := s.resolveEnvironment(ctx, "")
			if err != nil {
				logger.Error("environment lookup failed", "error", err)
				return nil, err
			}
			envID = resolvedID
		}
	}
	if err := s.ensureEnvironmentActive(ctx, envID); err != nil {
		return nil, err
	}
	if err := validation.ValidateSchema(contentType.Schema); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}
	version, err := resolveContentSchemaVersion(contentType.Schema, contentType.Slug)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}
	entryMetadata, err := normalizeEntryMetadata(req.Metadata)
	if err != nil {
		return nil, err
	}

	if existing, err := s.contents.GetBySlug(ctx, slugValue, req.ContentTypeID, envID.String()); err == nil && existing != nil {
		return nil, ErrSlugExists
	} else if err != nil {
		var notFound *NotFoundError
		if !errors.As(err, &notFound) {
			logger.Error("content slug lookup failed", "error", err)
			return nil, err
		}
	}

	now := s.now()

	record := &Content{
		ID:            s.id(),
		ContentTypeID: req.ContentTypeID,
		EnvironmentID: envID,
		Status:        chooseStatus(req.Status),
		Slug:          slugValue,
		Metadata:      entryMetadata,
		CreatedBy:     req.CreatedBy,
		UpdatedBy:     req.UpdatedBy,
		CreatedAt:     now,
		UpdatedAt:     now,
		Translations:  []*ContentTranslation{},
		Type:          contentType,
	}

	primaryLocale := ""
	if len(req.Translations) > 0 {
		groupID := record.ID
		seenLocales := map[string]struct{}{}
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
			if primaryLocale == "" {
				primaryLocale = loc.Code
			}
			payload := mergeBlocksContent(tr.Content, tr.Blocks)
			cleanContent := stripSchemaVersion(payload)
			if blocks, ok := ExtractEmbeddedBlocks(cleanContent); ok {
				if err := s.validateBlockAvailability(ctx, contentType.Slug, contentType.Schema, blocks); err != nil {
					return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
				}
				if err := s.validateEmbeddedBlocks(ctx, code, blocks, EmbeddedBlockValidationStrict); err != nil {
					return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
				}
			}
			if err := validation.ValidatePayload(contentType.Schema, SanitizeEmbeddedBlocks(cleanContent)); err != nil {
				return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
			}

			translation := &ContentTranslation{
				ID:        s.id(),
				ContentID: record.ID,
				LocaleID:  loc.ID,
				TranslationGroupID: func() *uuid.UUID {
					return &groupID
				}(),
				Title:     tr.Title,
				Summary:   tr.Summary,
				Content:   applySchemaVersion(cleanContent, version),
				Locale:    loc,
				CreatedAt: now,
				UpdatedAt: now,
			}

			record.Translations = append(record.Translations, translation)
			seenLocales[code] = struct{}{}
		}
	}
	if primaryLocale != "" {
		record.PrimaryLocale = primaryLocale
	}

	created, err := s.contents.Create(ctx, record)
	if err != nil {
		logger.Error("content repository create failed", "error", err)
		return nil, err
	}
	if created != nil && created.Type == nil {
		created.Type = contentType
	}

	logger = logging.WithFields(logger, map[string]any{
		"content_id": created.ID,
	})
	logger.Info("content created")
	meta := map[string]any{
		"slug":            created.Slug,
		"status":          created.Status,
		"locales":         collectContentLocalesFromInputs(req.Translations),
		"content_type_id": created.ContentTypeID.String(),
	}
	if created.EnvironmentID != uuid.Nil {
		meta["environment_id"] = created.EnvironmentID.String()
	}
	s.emitActivity(ctx, pickActor(req.CreatedBy, req.UpdatedBy), "create", "content", created.ID, meta)
	if err := s.syncEmbeddedBlocks(ctx, created.ID, req.Translations, pickActor(req.CreatedBy, req.UpdatedBy)); err != nil {
		logger.Error("embedded block sync failed", "error", err)
		return nil, err
	}

	return s.decorateContent(created), nil
}

// Get fetches content by identifier. Use WithTranslations to preload translations.
func (s *service) Get(ctx context.Context, id uuid.UUID, opts ...ContentGetOption) (*Content, error) {
	logger := s.opLogger(ctx, "content.get", map[string]any{
		"content_id": id,
	})
	record, err := s.contents.GetByID(ctx, id)
	if err != nil {
		logger.Error("content lookup failed", "error", err)
		return nil, err
	}
	if err := s.ensureEnvironmentActive(ctx, record.EnvironmentID); err != nil {
		return nil, err
	}
	s.attachContentType(ctx, record)
	readOpts := parseContentListOptions(opts...)
	mode, err := s.resolveProjectionMode(readOpts)
	if err != nil {
		return nil, err
	}
	translationsRequested := s.shouldLoadTranslations(readOpts, mode)
	translationsLoaded := translationsRequested && s.translationsEnabledFlag()
	if translationsLoaded {
		if err := s.loadTranslations(ctx, record); err != nil {
			logger.Error("content translation lookup failed", "error", err)
			return nil, err
		}
	} else {
		record.Translations = nil
	}
	s.mergeLegacyBlocks(ctx, record)
	if err := s.applyProjection(record, readOpts, mode, translationsLoaded); err != nil {
		logger.Error("content projection failed", "error", err)
		return nil, err
	}
	logger.Debug("content retrieved")
	return s.decorateContent(record), nil
}

// List returns content entries. Use WithTranslations to preload translations for list views.
func (s *service) List(ctx context.Context, env ...ContentListOption) ([]*Content, error) {
	logger := s.opLogger(ctx, "content.list", nil)
	opts := parseContentListOptions(env...)
	envID, _, err := s.resolveEnvironment(ctx, opts.envKey)
	if err != nil {
		logger.Error("content list environment lookup failed", "error", err)
		return nil, err
	}
	mode, err := s.resolveProjectionMode(opts)
	if err != nil {
		return nil, err
	}
	listArgs := []ContentListOption{ContentListOption(envID.String())}
	translationsRequested := s.shouldLoadTranslations(opts, mode)
	translationsLoaded := translationsRequested && s.translationsEnabledFlag()
	if translationsLoaded {
		listArgs = append(listArgs, WithTranslations())
	}
	records, err := s.contents.List(ctx, listArgs...)
	if err != nil {
		logger.Error("content list failed", "error", err)
		return nil, err
	}
	for _, record := range records {
		s.attachContentType(ctx, record)
		if !translationsLoaded {
			record.Translations = nil
		}
		s.mergeLegacyBlocks(ctx, record)
		if err := s.applyProjection(record, opts, mode, translationsLoaded); err != nil {
			logger.Error("content projection failed", "error", err, "content_id", record.ID)
			return nil, err
		}
		s.decorateContent(record)
	}
	logger.Debug("content list returned records", "count", len(records))
	return records, nil
}

func (s *service) Update(ctx context.Context, req UpdateContentRequest) (*Content, error) {
	if req.ID == uuid.Nil {
		return nil, ErrContentIDRequired
	}
	if s.translationsRequired() && len(req.Translations) == 0 && !req.AllowMissingTranslations {
		return nil, ErrNoTranslations
	}
	if s.requiresDefaultLocale() && !req.AllowMissingTranslations && len(req.Translations) > 0 && !s.hasDefaultLocale(req.Translations) {
		return nil, ErrDefaultLocaleRequired
	}

	logger := s.opLogger(ctx, "content.update", map[string]any{
		"content_id": req.ID,
	})

	existing, err := s.contents.GetByID(ctx, req.ID)
	if err != nil {
		logger.Error("content lookup failed", "error", err)
		return nil, err
	}
	if err := s.ensureEnvironmentActive(ctx, existing.EnvironmentID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.EnvironmentKey) != "" || s.requireExplicitEnv {
		envID, _, err := s.resolveEnvironment(ctx, req.EnvironmentKey)
		if err != nil {
			return nil, err
		}
		if existing.EnvironmentID != uuid.Nil && existing.EnvironmentID != envID {
			return nil, cmsenv.ErrEnvironmentNotFound
		}
	}
	s.attachContentType(ctx, existing)
	contentType := existing.Type
	if contentType == nil {
		contentType, err = s.contentTypes.GetByID(ctx, existing.ContentTypeID)
		if err != nil {
			return nil, ErrContentTypeRequired
		}
	}
	if err := validation.ValidateSchema(contentType.Schema); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}
	version, err := resolveContentSchemaVersion(contentType.Schema, contentType.Slug)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}

	now := s.now()

	replaceTranslations := len(req.Translations) > 0
	var translations []*ContentTranslation
	if replaceTranslations {
		existingLocales := indexTranslationsByLocaleID(existing.Translations)

		var err error
		translations, err = s.buildTranslations(ctx, existing.ID, req.Translations, existingLocales, now, contentType.Slug, contentType.Schema, version)
		if err != nil {
			logger.Error("content translations build failed", "error", err)
			return nil, err
		}
	}
	if replaceTranslations && strings.TrimSpace(existing.PrimaryLocale) == "" {
		if primary := primaryLocaleFromContentInputs(ctx, s.locales, req.Translations); primary != "" {
			existing.PrimaryLocale = primary
		}
	}

	existing.Status = chooseStatus(req.Status)
	if req.UpdatedBy != uuid.Nil {
		existing.UpdatedBy = req.UpdatedBy
	}
	existing.UpdatedAt = now
	if replaceTranslations {
		existing.Translations = translations
	}

	if replaceTranslations {
		if err := s.contents.ReplaceTranslations(ctx, existing.ID, translations); err != nil {
			logger.Error("content translations replace failed", "error", err)
			return nil, err
		}
	}

	if req.Metadata != nil {
		entryMetadata, err := normalizeEntryMetadata(req.Metadata)
		if err != nil {
			return nil, err
		}
		existing.Metadata = entryMetadata
	}

	updated, err := s.contents.Update(ctx, existing)
	if err != nil {
		logger.Error("content repository update failed", "error", err)
		return nil, err
	}

	logger.Info("content updated")
	meta := map[string]any{
		"slug":    existing.Slug,
		"status":  existing.Status,
		"locales": collectContentLocalesFromTranslations(existing.Translations),
	}
	if existing.EnvironmentID != uuid.Nil {
		meta["environment_id"] = existing.EnvironmentID.String()
	}
	if replaceTranslations {
		meta["locales"] = collectContentLocalesFromInputs(req.Translations)
	}
	s.emitActivity(ctx, req.UpdatedBy, "update", "content", existing.ID, meta)
	s.attachContentType(ctx, updated)
	if replaceTranslations {
		if err := s.syncEmbeddedBlocks(ctx, updated.ID, req.Translations, req.UpdatedBy); err != nil {
			logger.Error("embedded block sync failed", "error", err)
			return nil, err
		}
	}
	return s.decorateContent(updated), nil
}

func (s *service) Delete(ctx context.Context, req DeleteContentRequest) error {
	if req.ID == uuid.Nil {
		return ErrContentIDRequired
	}
	if !req.HardDelete {
		return ErrContentSoftDeleteUnsupported
	}

	logger := s.opLogger(ctx, "content.delete", map[string]any{
		"content_id": req.ID,
	})

	record, err := s.contents.GetByID(ctx, req.ID)
	if err != nil {
		logger.Error("content lookup failed", "error", err)
		return err
	}
	if err := s.ensureEnvironmentActive(ctx, record.EnvironmentID); err != nil {
		return err
	}

	if s.scheduler != nil {
		if err := s.scheduler.CancelByKey(ctx, cmsscheduler.ContentPublishJobKey(req.ID)); err != nil && !errors.Is(err, interfaces.ErrJobNotFound) {
			logger.Warn("content publish job cancel failed", "error", err)
		}
		if err := s.scheduler.CancelByKey(ctx, cmsscheduler.ContentUnpublishJobKey(req.ID)); err != nil && !errors.Is(err, interfaces.ErrJobNotFound) {
			logger.Warn("content unpublish job cancel failed", "error", err)
		}
	}

	if err := s.contents.Delete(ctx, req.ID, true); err != nil {
		logger.Error("content repository delete failed", "error", err)
		return err
	}

	logger.Info("content deleted")
	meta := map[string]any{
		"slug":    record.Slug,
		"status":  record.Status,
		"locales": collectContentLocalesFromTranslations(record.Translations),
	}
	if record.EnvironmentID != uuid.Nil {
		meta["environment_id"] = record.EnvironmentID.String()
	}
	s.emitActivity(ctx, pickActor(req.DeletedBy, record.UpdatedBy, record.CreatedBy), "delete", "content", record.ID, meta)
	return nil
}

// UpdateTranslation mutates a single translation without replacing the entire set.
func (s *service) UpdateTranslation(ctx context.Context, req UpdateContentTranslationRequest) (*ContentTranslation, error) {
	if !s.translationsEnabledFlag() {
		return nil, ErrContentTranslationsDisabled
	}
	if req.ContentID == uuid.Nil {
		return nil, ErrContentIDRequired
	}
	localeCode := strings.TrimSpace(req.Locale)
	if localeCode == "" {
		return nil, ErrUnknownLocale
	}
	if s.requiresDefaultLocale() && s.isDefaultLocale(localeCode) {
		return nil, ErrDefaultLocaleRequired
	}

	logger := s.opLogger(ctx, "content.translation.update", map[string]any{
		"content_id": req.ContentID,
		"locale":     localeCode,
	})

	record, err := s.contents.GetByID(ctx, req.ContentID)
	if err != nil {
		logger.Error("content lookup failed", "error", err)
		return nil, err
	}
	contentType, err := s.contentTypes.GetByID(ctx, record.ContentTypeID)
	if err != nil {
		logger.Error("content type lookup failed", "error", err)
		return nil, ErrContentTypeRequired
	}
	if err := validation.ValidateSchema(contentType.Schema); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}
	version, err := resolveContentSchemaVersion(contentType.Schema, contentType.Slug)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}

	loc, err := s.locales.GetByCode(ctx, localeCode)
	if err != nil {
		logger.Error("locale lookup failed", "error", err)
		return nil, ErrUnknownLocale
	}

	if strings.TrimSpace(record.PrimaryLocale) == "" {
		record.PrimaryLocale = loc.Code
	}

	var target *ContentTranslation
	targetIdx := -1
	for idx, tr := range record.Translations {
		if tr == nil {
			continue
		}
		if tr.LocaleID == loc.ID {
			target = tr
			targetIdx = idx
			break
		}
	}
	if target == nil {
		return nil, ErrContentTranslationNotFound
	}

	if req.Content == nil {
		req.Content = map[string]any{}
	}
	payload := mergeBlocksContent(req.Content, req.Blocks)
	cleanContent := stripSchemaVersion(payload)
	if blocks, ok := ExtractEmbeddedBlocks(cleanContent); ok {
		if err := s.validateBlockAvailability(ctx, contentType.Slug, contentType.Schema, blocks); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
		}
		if err := s.validateEmbeddedBlocks(ctx, localeCode, blocks, EmbeddedBlockValidationStrict); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
		}
	}
	if err := validation.ValidatePayload(contentType.Schema, SanitizeEmbeddedBlocks(cleanContent)); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}

	now := s.now()
	updatedTranslation := &ContentTranslation{
		ID:        target.ID,
		ContentID: req.ContentID,
		LocaleID:  loc.ID,
		TranslationGroupID: func() *uuid.UUID {
			if target.TranslationGroupID != nil {
				return target.TranslationGroupID
			}
			groupID := req.ContentID
			return &groupID
		}(),
		Title:     req.Title,
		Summary:   cloneString(req.Summary),
		Content:   applySchemaVersion(cleanContent, version),
		CreatedAt: target.CreatedAt,
		UpdatedAt: now,
		Locale:    loc,
	}

	translations := make([]*ContentTranslation, len(record.Translations))
	for i, tr := range record.Translations {
		if i == targetIdx {
			translations[i] = updatedTranslation
			continue
		}
		translations[i] = tr
	}

	if err := s.contents.ReplaceTranslations(ctx, req.ContentID, translations); err != nil {
		logger.Error("content translation replace failed", "error", err)
		return nil, err
	}

	record.Translations = translations
	record.UpdatedAt = now
	if req.UpdatedBy != uuid.Nil {
		record.UpdatedBy = req.UpdatedBy
	}
	if _, err := s.contents.Update(ctx, record); err != nil {
		logger.Error("content update failed after translation mutate", "error", err)
		return nil, err
	}

	logger.Info("content translation updated")
	meta := map[string]any{
		"content_id": req.ContentID.String(),
		"locale":     loc.Code,
		"title":      req.Title,
	}
	if record.EnvironmentID != uuid.Nil {
		meta["environment_id"] = record.EnvironmentID.String()
	}
	s.emitActivity(ctx, req.UpdatedBy, "update", "content_translation", updatedTranslation.ID, meta)
	if err := s.syncEmbeddedBlocks(ctx, req.ContentID, []ContentTranslationInput{{
		Locale:  loc.Code,
		Title:   req.Title,
		Summary: req.Summary,
		Content: req.Content,
		Blocks:  req.Blocks,
	}}, req.UpdatedBy); err != nil {
		logger.Error("embedded block sync failed", "error", err)
		return nil, err
	}
	return updatedTranslation, nil
}

// DeleteTranslation removes a locale from the translation set.
func (s *service) DeleteTranslation(ctx context.Context, req DeleteContentTranslationRequest) error {
	if !s.translationsEnabledFlag() {
		return ErrContentTranslationsDisabled
	}
	if req.ContentID == uuid.Nil {
		return ErrContentIDRequired
	}
	localeCode := strings.TrimSpace(req.Locale)
	if localeCode == "" {
		return ErrUnknownLocale
	}

	logger := s.opLogger(ctx, "content.translation.delete", map[string]any{
		"content_id": req.ContentID,
		"locale":     localeCode,
	})

	record, err := s.contents.GetByID(ctx, req.ContentID)
	if err != nil {
		logger.Error("content lookup failed", "error", err)
		return err
	}

	if len(record.Translations) == 0 {
		return ErrContentTranslationNotFound
	}

	loc, err := s.locales.GetByCode(ctx, localeCode)
	if err != nil {
		logger.Error("locale lookup failed", "error", err)
		return ErrUnknownLocale
	}

	var removed bool
	var removedTranslationID uuid.UUID
	translations := make([]*ContentTranslation, 0, len(record.Translations))
	for _, tr := range record.Translations {
		if tr == nil {
			continue
		}
		if tr.LocaleID == loc.ID {
			removed = true
			removedTranslationID = tr.ID
			continue
		}
		translations = append(translations, tr)
	}
	if !removed {
		return ErrContentTranslationNotFound
	}

	if s.translationsRequired() && len(translations) == 0 {
		return ErrNoTranslations
	}

	if err := s.contents.ReplaceTranslations(ctx, req.ContentID, translations); err != nil {
		logger.Error("content translation replace failed", "error", err)
		return err
	}

	record.Translations = translations
	record.UpdatedAt = s.now()
	if req.DeletedBy != uuid.Nil {
		record.UpdatedBy = req.DeletedBy
	}
	if _, err := s.contents.Update(ctx, record); err != nil {
		logger.Error("content update failed after translation delete", "error", err)
		return err
	}

	logger.Info("content translation deleted")
	targetID := removedTranslationID
	if targetID == uuid.Nil {
		targetID = req.ContentID
	}
	meta := map[string]any{
		"content_id": req.ContentID.String(),
		"locale":     loc.Code,
	}
	if record.EnvironmentID != uuid.Nil {
		meta["environment_id"] = record.EnvironmentID.String()
	}
	s.emitActivity(ctx, req.DeletedBy, "delete", "content_translation", targetID, meta)
	return nil
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

	logger := s.opLogger(ctx, "content.schedule", map[string]any{
		"content_id": req.ContentID,
	})

	record, err := s.contents.GetByID(ctx, req.ContentID)
	if err != nil {
		logger.Error("content schedule lookup failed", "error", err)
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
				logger.Error("content publish job enqueue failed", "error", err)
				return nil, err
			}
			logger.Debug("content publish job enqueued", "job_key", cmsscheduler.ContentPublishJobKey(record.ID))
		} else {
			cancelErr := s.scheduler.CancelByKey(ctx, cmsscheduler.ContentPublishJobKey(record.ID))
			if cancelErr != nil && !errors.Is(cancelErr, interfaces.ErrJobNotFound) {
				logger.Error("content publish job cancel failed", "error", cancelErr)
				return nil, cancelErr
			}
			if cancelErr == nil {
				logger.Debug("content publish job cancelled", "job_key", cmsscheduler.ContentPublishJobKey(record.ID))
			}
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
				logger.Error("content unpublish job enqueue failed", "error", err)
				return nil, err
			}
			logger.Debug("content unpublish job enqueued", "job_key", cmsscheduler.ContentUnpublishJobKey(record.ID))
		} else {
			cancelErr := s.scheduler.CancelByKey(ctx, cmsscheduler.ContentUnpublishJobKey(record.ID))
			if cancelErr != nil && !errors.Is(cancelErr, interfaces.ErrJobNotFound) {
				logger.Error("content unpublish job cancel failed", "error", cancelErr)
				return nil, cancelErr
			}
			if cancelErr == nil {
				logger.Debug("content unpublish job cancelled", "job_key", cmsscheduler.ContentUnpublishJobKey(record.ID))
			}
		}
	}

	updated, err := s.contents.Update(ctx, record)
	if err != nil {
		logger.Error("content schedule update failed", "error", err)
		return nil, err
	}

	logger.Info("content schedule updated",
		"publish_at", record.PublishAt,
		"unpublish_at", record.UnpublishAt,
		"status", record.Status,
	)
	meta := map[string]any{
		"status":       record.Status,
		"publish_at":   record.PublishAt,
		"unpublish_at": record.UnpublishAt,
	}
	if record.EnvironmentID != uuid.Nil {
		meta["environment_id"] = record.EnvironmentID.String()
	}
	s.emitActivity(ctx, req.ScheduledBy, "schedule", "content", record.ID, meta)

	return s.decorateContent(updated), nil
}

func (s *service) CreateDraft(ctx context.Context, req CreateContentDraftRequest) (*ContentVersion, error) {
	if !s.versioningEnabled {
		return nil, ErrVersioningDisabled
	}
	if req.ContentID == uuid.Nil {
		return nil, ErrContentIDRequired
	}

	extra := map[string]any{
		"content_id": req.ContentID,
	}
	if req.BaseVersion != nil {
		extra["base_version"] = *req.BaseVersion
	}
	logger := s.opLogger(ctx, "content.version.create_draft", extra)

	contentRecord, err := s.contents.GetByID(ctx, req.ContentID)
	if err != nil {
		logger.Error("content lookup failed", "error", err)
		return nil, err
	}

	contentType, err := s.contentTypes.GetByID(ctx, contentRecord.ContentTypeID)
	if err != nil {
		logger.Error("content type lookup failed", "error", err)
		return nil, ErrContentTypeRequired
	}
	if err := validation.ValidateSchema(contentType.Schema); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}

	snapshot := cloneContentVersionSnapshot(req.Snapshot)
	if snapshot.Metadata == nil {
		snapshot.Metadata = cloneMap(contentRecord.Metadata)
	}
	entryMetadata, err := normalizeEntryMetadata(snapshot.Metadata)
	if err != nil {
		return nil, err
	}
	snapshot.Metadata = entryMetadata
	if err := s.validateSnapshot(ctx, contentType.Schema, snapshot, EmbeddedBlockValidationDraft); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}

	versions, err := s.contents.ListVersions(ctx, req.ContentID)
	if err != nil {
		logger.Error("content version list failed", "error", err)
		return nil, err
	}
	logger.Debug("content versions loaded", "count", len(versions))

	if s.versionRetentionLimit > 0 && len(versions) >= s.versionRetentionLimit {
		logger.Warn("content version retention limit reached", "limit", s.versionRetentionLimit)
		return nil, ErrContentVersionRetentionExceeded
	}

	next := nextContentVersionNumber(versions)
	if req.BaseVersion != nil && *req.BaseVersion != next-1 {
		logger.Warn("content version conflict detected", "expected_base", next-1, "provided_base", *req.BaseVersion)
		return nil, ErrContentVersionConflict
	}

	now := s.now()
	version := &ContentVersion{
		ID:        s.id(),
		ContentID: req.ContentID,
		Version:   next,
		Status:    domain.StatusDraft,
		Snapshot:  snapshot,
		CreatedBy: req.CreatedBy,
		CreatedAt: now,
	}

	created, err := s.contents.CreateVersion(ctx, version)
	if err != nil {
		logger.Error("content version create failed", "error", err)
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
		logger.Error("content record update failed", "error", err)
		return nil, err
	}

	logger = logging.WithFields(logger, map[string]any{
		"version": created.Version,
	})
	logger.Info("content draft created")

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

	logger := s.opLogger(ctx, "content.version.publish", map[string]any{
		"content_id": req.ContentID,
		"version":    req.Version,
	})

	contentRecord, err := s.contents.GetByID(ctx, req.ContentID)
	if err != nil {
		logger.Error("content lookup failed", "error", err)
		return nil, err
	}

	version, err := s.contents.GetVersion(ctx, req.ContentID, req.Version)
	if err != nil {
		logger.Error("content version lookup failed", "error", err)
		return nil, err
	}
	if version.Status == domain.StatusPublished {
		logger.Warn("content version already published")
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

	contentType, err := s.contentTypes.GetByID(ctx, contentRecord.ContentTypeID)
	if err != nil {
		logger.Error("content type lookup failed", "error", err)
		return nil, ErrContentTypeRequired
	}
	if err := validation.ValidateSchema(contentType.Schema); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}
	migratedSnapshot, err := s.migrateContentSnapshot(contentType, version.Snapshot, true)
	if err != nil {
		logger.Error("content schema migration failed", "error", err)
		return nil, err
	}
	migratedSnapshot, err = s.migrateEmbeddedBlocksSnapshot(ctx, migratedSnapshot)
	if err != nil {
		logger.Error("embedded block migration failed", "error", err)
		var embeddedErr *EmbeddedBlockValidationError
		if errors.As(err, &embeddedErr) {
			return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
		}
		return nil, err
	}
	entryMetadata, err := normalizeEntryMetadata(migratedSnapshot.Metadata)
	if err != nil {
		return nil, err
	}
	migratedSnapshot.Metadata = entryMetadata
	if err := s.validateSnapshot(ctx, contentType.Schema, migratedSnapshot, EmbeddedBlockValidationStrict); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}
	version.Snapshot = migratedSnapshot

	updatedVersion, err := s.contents.UpdateVersion(ctx, version)
	if err != nil {
		logger.Error("content version update failed", "error", err)
		return nil, err
	}

	if contentRecord.PublishedVersion != nil && *contentRecord.PublishedVersion != updatedVersion.Version {
		prev, prevErr := s.contents.GetVersion(ctx, req.ContentID, *contentRecord.PublishedVersion)
		if prevErr == nil && prev.Status == domain.StatusPublished {
			prev.Status = domain.StatusArchived
			if _, archiveErr := s.contents.UpdateVersion(ctx, prev); archiveErr != nil {
				logger.Error("content previous version archive failed", "error", archiveErr, "previous_version", prev.Version)
				return nil, archiveErr
			}
			logger.Debug("content previous version archived", "previous_version", prev.Version)
		} else if prevErr != nil {
			logger.Error("content previous version lookup failed", "error", prevErr, "previous_version", *contentRecord.PublishedVersion)
		}
	}

	contentRecord.PublishedVersion = &updatedVersion.Version
	contentRecord.PublishedAt = &publishedAt
	if req.PublishedBy != uuid.Nil {
		contentRecord.PublishedBy = &req.PublishedBy
	}
	contentRecord.Status = string(domain.StatusPublished)
	if updatedVersion.Snapshot.Metadata != nil {
		contentRecord.Metadata = cloneMap(updatedVersion.Snapshot.Metadata)
	}
	if updatedVersion.Version > contentRecord.CurrentVersion {
		contentRecord.CurrentVersion = updatedVersion.Version
	}

	contentRecord.UpdatedAt = s.now()
	if req.PublishedBy != uuid.Nil {
		contentRecord.UpdatedBy = req.PublishedBy
	}

	if _, err := s.contents.Update(ctx, contentRecord); err != nil {
		logger.Error("content record publish update failed", "error", err)
		return nil, err
	}

	logger.Info("content version published", "published_at", publishedAt)
	meta := map[string]any{
		"version":      updatedVersion.Version,
		"status":       contentRecord.Status,
		"published_at": publishedAt,
	}
	if contentRecord.EnvironmentID != uuid.Nil {
		meta["environment_id"] = contentRecord.EnvironmentID.String()
	}
	s.emitActivity(ctx, req.PublishedBy, "publish", "content", contentRecord.ID, meta)

	return cloneContentVersion(updatedVersion), nil
}

func (s *service) ListVersions(ctx context.Context, contentID uuid.UUID) ([]*ContentVersion, error) {
	if !s.versioningEnabled {
		return nil, ErrVersioningDisabled
	}
	if contentID == uuid.Nil {
		return nil, ErrContentIDRequired
	}

	logger := s.opLogger(ctx, "content.version.list", map[string]any{
		"content_id": contentID,
	})

	versions, err := s.contents.ListVersions(ctx, contentID)
	if err != nil {
		logger.Error("content version list failed", "error", err)
		return nil, err
	}
	results := cloneContentVersions(versions)
	logger.Debug("content versions returned", "count", len(results))
	return results, nil
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

	logger := s.opLogger(ctx, "content.version.restore", map[string]any{
		"content_id": req.ContentID,
		"version":    req.Version,
	})

	version, err := s.contents.GetVersion(ctx, req.ContentID, req.Version)
	if err != nil {
		logger.Error("content version lookup failed", "error", err)
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

// PreviewDraft returns a migrated draft snapshot without persisting changes.
func (s *service) PreviewDraft(ctx context.Context, req PreviewContentDraftRequest) (*ContentPreview, error) {
	if !s.versioningEnabled {
		return nil, ErrVersioningDisabled
	}
	if req.ContentID == uuid.Nil {
		return nil, ErrContentIDRequired
	}
	if req.Version <= 0 {
		return nil, ErrContentVersionRequired
	}

	logger := s.opLogger(ctx, "content.version.preview", map[string]any{
		"content_id": req.ContentID,
		"version":    req.Version,
	})

	record, err := s.contents.GetByID(ctx, req.ContentID)
	if err != nil {
		logger.Error("content lookup failed", "error", err)
		return nil, err
	}

	version, err := s.contents.GetVersion(ctx, req.ContentID, req.Version)
	if err != nil {
		logger.Error("content version lookup failed", "error", err)
		return nil, err
	}

	contentType, err := s.contentTypes.GetByID(ctx, record.ContentTypeID)
	if err != nil {
		logger.Error("content type lookup failed", "error", err)
		return nil, ErrContentTypeRequired
	}
	if err := validation.ValidateSchema(contentType.Schema); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}

	previewVersion := cloneContentVersion(version)
	migratedSnapshot, err := s.migrateContentSnapshot(contentType, previewVersion.Snapshot, false)
	if err != nil {
		logger.Error("content schema migration failed", "error", err)
		return nil, err
	}
	migratedSnapshot, err = s.migrateEmbeddedBlocksSnapshot(ctx, migratedSnapshot)
	if err != nil {
		logger.Error("embedded block migration failed", "error", err)
		var embeddedErr *EmbeddedBlockValidationError
		if errors.As(err, &embeddedErr) {
			return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
		}
		return nil, err
	}
	if err := s.validateSnapshot(ctx, contentType.Schema, migratedSnapshot, EmbeddedBlockValidationDraft); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
	}
	previewVersion.Snapshot = migratedSnapshot

	previewRecord := cloneContent(record)
	if previewRecord == nil {
		return nil, ErrContentIDRequired
	}
	previewRecord.Status = string(previewVersion.Status)
	previewRecord.CurrentVersion = previewVersion.Version
	previewRecord.Type = contentType
	if previewVersion.Snapshot.Metadata == nil {
		previewVersion.Snapshot.Metadata = cloneMap(record.Metadata)
	}
	previewRecord.Metadata = cloneMap(previewVersion.Snapshot.Metadata)

	translations, err := s.previewTranslations(ctx, previewRecord.ID, migratedSnapshot)
	if err != nil {
		logger.Error("content preview translations failed", "error", err)
		return nil, err
	}
	previewRecord.Translations = translations
	s.decorateContent(previewRecord)

	logger.Info("content preview built")
	return &ContentPreview{
		Content: previewRecord,
		Version: previewVersion,
	}, nil
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

func (s *service) attachContentType(ctx context.Context, record *Content) {
	if record == nil || record.Type != nil || record.ContentTypeID == uuid.Nil || s.contentTypes == nil {
		return
	}
	ct, err := s.contentTypes.GetByID(ctx, record.ContentTypeID)
	if err != nil {
		return
	}
	record.Type = ct
}

func (s *service) migrateContentSnapshot(contentType *ContentType, snapshot ContentVersionSnapshot, strict bool) (ContentVersionSnapshot, error) {
	if contentType == nil {
		return snapshot, ErrContentTypeRequired
	}
	targetVersion, err := resolveContentSchemaVersion(contentType.Schema, contentType.Slug)
	if err != nil {
		return snapshot, err
	}
	if len(snapshot.Translations) == 0 {
		return snapshot, nil
	}
	updated := cloneContentVersionSnapshot(snapshot)
	for idx, tr := range updated.Translations {
		if tr.Content == nil {
			tr.Content = map[string]any{}
		}
		migrated, _, err := s.migratePayload(contentType.Slug, contentType.Schema, targetVersion, tr.Content, strict)
		if err != nil {
			return snapshot, err
		}
		updated.Translations[idx].Content = migrated
	}
	return updated, nil
}

func (s *service) migratePayload(slug string, schema map[string]any, target cmsschema.Version, payload map[string]any, strict bool) (map[string]any, bool, error) {
	current, ok := cmsschema.RootSchemaVersion(payload)
	if !ok || current.String() == target.String() {
		return applySchemaVersion(stripSchemaVersion(payload), target), false, nil
	}
	if s.schemaMigrator == nil {
		return nil, false, ErrContentSchemaMigrationRequired
	}
	if current.Slug != "" && target.Slug != "" && current.Slug != target.Slug {
		return nil, false, fmt.Errorf("%w: schema slug mismatch", ErrContentSchemaMigrationRequired)
	}
	trimmed := stripSchemaVersion(payload)
	migrated, err := s.schemaMigrator.Migrate(slug, current.String(), target.String(), trimmed)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %v", ErrContentSchemaMigrationRequired, err)
	}
	clean := stripSchemaVersion(migrated)
	if strict && schema != nil {
		if err := validation.ValidateMigrationPayload(schema, SanitizeEmbeddedBlocks(clean)); err != nil {
			return nil, false, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
		}
	}
	return applySchemaVersion(clean, target), true, nil
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

func (s *service) buildTranslations(ctx context.Context, contentID uuid.UUID, inputs []ContentTranslationInput, existing map[uuid.UUID]*ContentTranslation, now time.Time, contentType string, schema map[string]any, version cmsschema.Version) ([]*ContentTranslation, error) {
	seen := map[string]struct{}{}
	result := make([]*ContentTranslation, 0, len(inputs))

	for _, input := range inputs {
		code := strings.TrimSpace(input.Locale)
		if code == "" {
			return nil, ErrUnknownLocale
		}
		lower := strings.ToLower(code)
		if _, ok := seen[lower]; ok {
			return nil, ErrDuplicateLocale
		}

		loc, err := s.locales.GetByCode(ctx, code)
		if err != nil {
			return nil, ErrUnknownLocale
		}

		var summary *string
		if input.Summary != nil {
			value := *input.Summary
			summary = &value
		}
		payload := mergeBlocksContent(input.Content, input.Blocks)
		cleanContent := stripSchemaVersion(payload)
		if blocks, ok := ExtractEmbeddedBlocks(cleanContent); ok {
			if err := s.validateBlockAvailability(ctx, contentType, schema, blocks); err != nil {
				return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
			}
			if err := s.validateEmbeddedBlocks(ctx, code, blocks, EmbeddedBlockValidationStrict); err != nil {
				return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
			}
		}
		if err := validation.ValidatePayload(schema, SanitizeEmbeddedBlocks(cleanContent)); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrContentSchemaInvalid, err)
		}

		translation := &ContentTranslation{
			ContentID: contentID,
			LocaleID:  loc.ID,
			TranslationGroupID: func() *uuid.UUID {
				if existingTranslation, ok := existing[loc.ID]; ok && existingTranslation != nil && existingTranslation.TranslationGroupID != nil {
					return existingTranslation.TranslationGroupID
				}
				groupID := contentID
				return &groupID
			}(),
			Title:     input.Title,
			Summary:   summary,
			Content:   applySchemaVersion(cleanContent, version),
			Locale:    loc,
			UpdatedAt: now,
		}

		if existingTranslation, ok := existing[loc.ID]; ok && existingTranslation != nil {
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

func (s *service) previewTranslations(ctx context.Context, contentID uuid.UUID, snapshot ContentVersionSnapshot) ([]*ContentTranslation, error) {
	if len(snapshot.Translations) == 0 {
		return nil, nil
	}
	now := s.now()
	groupID := contentID
	out := make([]*ContentTranslation, 0, len(snapshot.Translations))
	for _, tr := range snapshot.Translations {
		code := strings.TrimSpace(tr.Locale)
		if code == "" {
			return nil, ErrUnknownLocale
		}
		locale, err := s.locales.GetByCode(ctx, code)
		if err != nil {
			return nil, ErrUnknownLocale
		}
		entry := &ContentTranslation{
			ID:                 s.id(),
			ContentID:          contentID,
			LocaleID:           locale.ID,
			TranslationGroupID: &groupID,
			Title:              tr.Title,
			Summary:            cloneString(tr.Summary),
			Content:            cloneMap(tr.Content),
			Locale:             locale,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		out = append(out, entry)
	}
	return out, nil
}

func indexTranslationsByLocaleID(translations []*ContentTranslation) map[uuid.UUID]*ContentTranslation {
	if len(translations) == 0 {
		return map[uuid.UUID]*ContentTranslation{}
	}
	indexed := make(map[uuid.UUID]*ContentTranslation, len(translations))
	for _, tr := range translations {
		if tr == nil {
			continue
		}
		indexed[tr.LocaleID] = tr
	}
	return indexed
}

func pickActor(ids ...uuid.UUID) uuid.UUID {
	for _, id := range ids {
		if id != uuid.Nil {
			return id
		}
	}
	return uuid.Nil
}

func collectContentLocalesFromInputs(inputs []ContentTranslationInput) []string {
	if len(inputs) == 0 {
		return nil
	}
	locales := make([]string, 0, len(inputs))
	for _, input := range inputs {
		code := strings.TrimSpace(input.Locale)
		if code == "" {
			continue
		}
		locales = append(locales, code)
	}
	return locales
}

func collectContentLocalesFromTranslations(translations []*ContentTranslation) []string {
	if len(translations) == 0 {
		return nil
	}
	locales := make([]string, 0, len(translations))
	for _, tr := range translations {
		if tr == nil {
			continue
		}
		if tr.Locale != nil && strings.TrimSpace(tr.Locale.Code) != "" {
			locales = append(locales, strings.TrimSpace(tr.Locale.Code))
			continue
		}
		locales = append(locales, tr.LocaleID.String())
	}
	return locales
}

func resolveContentSchemaVersion(schema map[string]any, slug string) (cmsschema.Version, error) {
	if len(schema) == 0 {
		if strings.TrimSpace(slug) == "" {
			return cmsschema.Version{}, cmsschema.ErrInvalidSchemaVersion
		}
		return cmsschema.DefaultVersion(slug), nil
	}
	_, version, err := cmsschema.EnsureSchemaVersion(schema, slug)
	return version, err
}

func stripSchemaVersion(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	clean := cloneMap(payload)
	delete(clean, cmsschema.RootSchemaKey)
	return clean
}

func (s *service) validateBlockAvailability(ctx context.Context, contentType string, schema map[string]any, blocks []map[string]any) error {
	if len(blocks) == 0 {
		return nil
	}
	availability := cmsschema.ExtractMetadata(schema).BlockAvailability
	if availability.Empty() {
		return nil
	}
	if s == nil || s.embeddedBlocks == nil {
		return ErrEmbeddedBlocksResolverMissing
	}
	return s.embeddedBlocks.ValidateBlockAvailability(ctx, contentType, availability, blocks)
}

func (s *service) validateEmbeddedBlocks(ctx context.Context, locale string, blocks []map[string]any, mode EmbeddedBlockValidationMode) error {
	if len(blocks) == 0 {
		return nil
	}
	if s == nil || s.embeddedBlocks == nil {
		return ErrEmbeddedBlocksResolverMissing
	}
	return s.embeddedBlocks.ValidateEmbeddedBlocks(ctx, locale, blocks, mode)
}

func (s *service) migrateEmbeddedBlocks(ctx context.Context, locale string, blocks []map[string]any) ([]map[string]any, error) {
	if len(blocks) == 0 {
		return blocks, nil
	}
	if s == nil || s.embeddedBlocks == nil {
		return nil, ErrEmbeddedBlocksResolverMissing
	}
	return s.embeddedBlocks.MigrateEmbeddedBlocks(ctx, locale, blocks)
}

func (s *service) migrateEmbeddedBlocksSnapshot(ctx context.Context, snapshot ContentVersionSnapshot) (ContentVersionSnapshot, error) {
	if len(snapshot.Translations) == 0 {
		return snapshot, nil
	}
	updated := cloneContentVersionSnapshot(snapshot)
	for idx, tr := range updated.Translations {
		if tr.Content == nil {
			tr.Content = map[string]any{}
		}
		blocks, ok := ExtractEmbeddedBlocks(tr.Content)
		if !ok || len(blocks) == 0 {
			updated.Translations[idx].Content = tr.Content
			continue
		}
		migrated, err := s.migrateEmbeddedBlocks(ctx, tr.Locale, blocks)
		if err != nil {
			return snapshot, err
		}
		updated.Translations[idx].Content = MergeEmbeddedBlocks(tr.Content, migrated)
	}
	return updated, nil
}

func (s *service) validateSnapshot(ctx context.Context, schema map[string]any, snapshot ContentVersionSnapshot, mode EmbeddedBlockValidationMode) error {
	if len(snapshot.Translations) == 0 {
		return nil
	}
	contentType := cmsschema.ExtractMetadata(schema).Slug
	for _, tr := range snapshot.Translations {
		contentPayload := tr.Content
		if contentPayload == nil {
			contentPayload = map[string]any{}
		}
		cleanContent := stripSchemaVersion(contentPayload)
		if blocks, ok := ExtractEmbeddedBlocks(cleanContent); ok {
			if err := s.validateBlockAvailability(ctx, contentType, schema, blocks); err != nil {
				return err
			}
			if err := s.validateEmbeddedBlocks(ctx, tr.Locale, blocks, mode); err != nil {
				return err
			}
		}
		switch mode {
		case EmbeddedBlockValidationDraft:
			if err := validation.ValidatePartialPayload(schema, SanitizeEmbeddedBlocks(cleanContent)); err != nil {
				return err
			}
		default:
			if err := validation.ValidatePayload(schema, SanitizeEmbeddedBlocks(cleanContent)); err != nil {
				return err
			}
		}
	}
	return nil
}

func mergeBlocksContent(content map[string]any, blocks []map[string]any) map[string]any {
	if len(blocks) == 0 {
		return content
	}
	return MergeEmbeddedBlocks(content, blocks)
}

func (s *service) syncEmbeddedBlocks(ctx context.Context, contentID uuid.UUID, translations []ContentTranslationInput, actor uuid.UUID) error {
	if s == nil || s.embeddedBlocks == nil {
		return nil
	}
	if contentID == uuid.Nil {
		return nil
	}
	return s.embeddedBlocks.SyncEmbeddedBlocks(ctx, contentID, translations, actor)
}

func (s *service) mergeLegacyBlocks(ctx context.Context, record *Content) {
	if s == nil || s.embeddedBlocks == nil || record == nil {
		return
	}
	if err := s.embeddedBlocks.MergeLegacyBlocks(ctx, record); err != nil {
		s.log(ctx).Warn("content.embedded_blocks.merge_failed", "error", err, "content_id", record.ID)
	}
}

func (s *service) loadTranslations(ctx context.Context, record *Content) error {
	if s == nil || record == nil || record.ID == uuid.Nil {
		return nil
	}
	if len(record.Translations) > 0 {
		return nil
	}
	reader, ok := s.contents.(ContentTranslationReader)
	if !ok {
		return nil
	}
	translations, err := reader.ListTranslations(ctx, record.ID)
	if err != nil {
		if errors.Is(err, ErrContentTranslationLookupUnsupported) {
			return nil
		}
		return err
	}
	record.Translations = translations
	return nil
}

func (s *service) resolveProjectionMode(opts contentListOptions) (ProjectionTranslationMode, error) {
	mode := s.projectionTranslationMode
	if mode == "" {
		mode = ProjectionTranslationModeAutoLoad
	}
	if !opts.projectionModeSet {
		return mode, nil
	}
	normalized, ok := normalizeProjectionTranslationMode(opts.projectionMode)
	if !ok {
		return "", ErrContentProjectionModeInvalid
	}
	return normalized, nil
}

func normalizeProjectionTranslationMode(mode ProjectionTranslationMode) (ProjectionTranslationMode, bool) {
	normalized := strings.ToLower(strings.TrimSpace(string(mode)))
	switch ProjectionTranslationMode(normalized) {
	case ProjectionTranslationModeAutoLoad, ProjectionTranslationModeNoop, ProjectionTranslationModeError:
		return ProjectionTranslationMode(normalized), true
	default:
		return "", false
	}
}

func (s *service) shouldLoadTranslations(opts contentListOptions, mode ProjectionTranslationMode) bool {
	if opts.includeTranslations {
		return true
	}
	if strings.TrimSpace(opts.projection) == "" {
		return false
	}
	return mode == ProjectionTranslationModeAutoLoad
}

func (s *service) applyProjection(record *Content, opts contentListOptions, mode ProjectionTranslationMode, translationsLoaded bool) error {
	if record == nil {
		return nil
	}
	projection := canonicalProjectionName(opts.projection)
	if projection == "" {
		return nil
	}
	if !translationsLoaded {
		if mode == ProjectionTranslationModeNoop {
			return nil
		}
		return ErrContentProjectionRequiresTranslations
	}

	switch projection {
	case ContentProjectionAdmin, ContentProjectionDerivedFields:
		for _, tr := range record.Translations {
			if tr == nil {
				continue
			}
			tr.Content = projectDerivedContentFields(tr.Content)
		}
		return nil
	default:
		return ErrContentProjectionUnsupported
	}
}

func canonicalProjectionName(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "":
		return ""
	case "derived", "derived-fields":
		return ContentProjectionDerivedFields
	default:
		return normalized
	}
}

func projectDerivedContentFields(payload map[string]any) map[string]any {
	result := cloneMap(payload)
	if result == nil {
		result = map[string]any{}
	}

	setDerivedFieldIfEmpty(result, "content", nestedLookup(result, "markdown", "body"))
	setDerivedFieldIfEmpty(result, "summary",
		result["excerpt"],
		sourceFieldValue(result, "summary"),
		sourceFieldValue(result, "excerpt"),
	)
	setDerivedFieldIfEmpty(result, "excerpt",
		result["summary"],
		sourceFieldValue(result, "excerpt"),
		sourceFieldValue(result, "summary"),
	)

	for _, field := range []string{
		"path",
		"published_at",
		"featured_image",
		"meta",
		"tags",
		"template_id",
		"parent_id",
		"blocks",
		"seo",
	} {
		setDerivedFieldIfEmpty(result, field, sourceFieldValue(result, field))
	}

	setDerivedFieldIfEmpty(result, "meta_title",
		sourceFieldValue(result, "meta_title"),
		seoFieldValue(result["seo"], "title"),
	)
	setDerivedFieldIfEmpty(result, "meta_description",
		sourceFieldValue(result, "meta_description"),
		seoFieldValue(result["seo"], "description"),
	)

	return result
}

func setDerivedFieldIfEmpty(target map[string]any, key string, candidates ...any) {
	if target == nil || strings.TrimSpace(key) == "" {
		return
	}
	if isNonEmptyValue(target[key]) {
		return
	}
	for _, candidate := range candidates {
		if isNonEmptyValue(candidate) {
			target[key] = candidate
			return
		}
	}
}

func sourceFieldValue(payload map[string]any, field string) any {
	if strings.TrimSpace(field) == "" {
		return nil
	}
	for _, path := range [][]string{
		{"markdown", "custom", field},
		{"markdown", "frontmatter", field},
		{"markdown", "custom", "markdown", "frontmatter", field},
	} {
		value := nestedLookup(payload, path...)
		if isNonEmptyValue(value) {
			return value
		}
	}
	return nil
}

func nestedLookup(payload map[string]any, path ...string) any {
	if payload == nil || len(path) == 0 {
		return nil
	}
	var current any = payload
	for _, part := range path {
		record, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		value, ok := record[part]
		if !ok {
			return nil
		}
		current = value
	}
	return current
}

func seoFieldValue(value any, key string) any {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	switch seo := value.(type) {
	case map[string]any:
		return seo[key]
	case map[string]string:
		return seo[key]
	default:
		return nil
	}
}

func isNonEmptyValue(value any) bool {
	if value == nil {
		return false
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case map[string]any:
		return len(typed) > 0
	case []string:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Interface, reflect.Pointer:
		if rv.IsNil() {
			return false
		}
		return isNonEmptyValue(rv.Elem().Interface())
	case reflect.Map, reflect.Slice, reflect.Array:
		return rv.Len() > 0
	default:
		return true
	}
}

func applySchemaVersion(payload map[string]any, version cmsschema.Version) map[string]any {
	result := cloneMap(payload)
	if result == nil {
		result = map[string]any{}
	}
	result[cmsschema.RootSchemaKey] = version.String()
	return result
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

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
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

func primaryLocaleFromContentInputs(ctx context.Context, locales LocaleRepository, inputs []ContentTranslationInput) string {
	if locales == nil {
		return ""
	}
	for _, input := range inputs {
		code := strings.TrimSpace(input.Locale)
		if code == "" {
			continue
		}
		loc, err := locales.GetByCode(ctx, code)
		if err == nil && loc != nil {
			return strings.TrimSpace(loc.Code)
		}
	}
	return ""
}
