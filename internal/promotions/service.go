package promotions

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/schema"
	"github.com/goliatone/go-cms/internal/validation"
	"github.com/goliatone/go-cms/pkg/activity"
	"github.com/google/uuid"
)

// ServiceOption configures the promotion service.
type ServiceOption func(*service)

// WithBlockService wires the block service used for definition promotion.
func WithBlockService(svc blocks.Service) ServiceOption {
	return func(s *service) {
		if svc != nil {
			s.blocks = svc
		}
	}
}

// WithSchemaMigrator wires the schema migrator used for content promotions.
func WithSchemaMigrator(migrator *schema.Migrator) ServiceOption {
	return func(s *service) {
		if migrator != nil {
			s.schemaMigrator = migrator
		}
	}
}

// WithEmbeddedBlocksResolver wires the embedded blocks resolver used for content promotions.
func WithEmbeddedBlocksResolver(resolver content.EmbeddedBlocksResolver) ServiceOption {
	return func(s *service) {
		if resolver != nil {
			s.embeddedBlocks = resolver
		}
	}
}

// WithActivityEmitter wires the activity emitter used for promotion events.
func WithActivityEmitter(emitter *activity.Emitter) ServiceOption {
	return func(s *service) {
		if emitter != nil {
			s.activity = emitter
		}
	}
}

// WithClock overrides the clock used for timestamps.
func WithClock(clock func() time.Time) ServiceOption {
	return func(s *service) {
		if clock != nil {
			s.now = clock
		}
	}
}

// WithIDGenerator overrides the ID generator used for new records.
func WithIDGenerator(generator func() uuid.UUID) ServiceOption {
	return func(s *service) {
		if generator != nil {
			s.id = generator
		}
	}
}

// WithDefaultEnvironmentKey overrides the default environment key.
func WithDefaultEnvironmentKey(key string) ServiceOption {
	return func(s *service) {
		if strings.TrimSpace(key) != "" {
			s.defaultEnvKey = key
		}
	}
}

// NewService constructs the promotion service.
func NewService(envSvc cmsenv.Service, contentTypes content.ContentTypeRepository, contents content.ContentRepository, locales content.LocaleRepository, opts ...ServiceOption) Service {
	if envSvc == nil {
		panic("promotions: environment service required")
	}
	if contentTypes == nil || contents == nil || locales == nil {
		panic("promotions: content repositories required")
	}
	svc := &service{
		envs:           envSvc,
		contentTypes:   contentTypes,
		contents:       contents,
		locales:        locales,
		now:            func() time.Time { return time.Now().UTC() },
		id:             uuid.New,
		activity:       activity.NewEmitter(nil, activity.Config{}),
		defaultEnvKey:  cmsenv.DefaultKey,
		schemaMigrator: nil,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

type service struct {
	envs           cmsenv.Service
	contentTypes   content.ContentTypeRepository
	contents       content.ContentRepository
	locales        content.LocaleRepository
	blocks         blocks.Service
	schemaMigrator *schema.Migrator
	embeddedBlocks content.EmbeddedBlocksResolver
	activity       *activity.Emitter
	now            func() time.Time
	id             func() uuid.UUID
	defaultEnvKey  string
}

func (s *service) PromoteEnvironment(ctx context.Context, req PromoteEnvironmentRequest) (*PromoteEnvironmentResult, error) {
	if s == nil {
		return nil, errors.New("promotions: service unavailable")
	}
	source, err := s.resolveEnvironment(ctx, req.SourceEnvironment, nil)
	if err != nil {
		return nil, err
	}
	target, err := s.resolveEnvironment(ctx, req.TargetEnvironment, nil)
	if err != nil {
		return nil, err
	}
	scope := req.Scope
	if scope == "" {
		scope = ScopeAll
	}

	result := &PromoteEnvironmentResult{
		SourceEnv: EnvironmentRef{ID: source.ID, Key: source.Key},
		TargetEnv: EnvironmentRef{ID: target.ID, Key: target.Key},
		Summary:   PromoteSummary{},
	}

	if scope == ScopeContentTypes || scope == ScopeAll {
		ids, err := s.collectContentTypeIDs(ctx, source.ID.String(), req.ContentTypeIDs, req.ContentTypeSlugs)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			item, err := s.PromoteContentType(ctx, PromoteContentTypeRequest{
				ContentTypeID:     id,
				TargetEnvironment: target.Key,
				Options:           req.Options,
			})
			if err != nil {
				result.Errors = append(result.Errors, PromoteError{Kind: "content_type", SourceID: id, Error: err.Error()})
				result.Summary.ContentTypes.Failed++
				continue
			}
			if item != nil {
				result.Items = append(result.Items, *item)
				updateSummary(&result.Summary.ContentTypes, item.Status)
			}
		}
	}

	if scope == ScopeContentEntries || scope == ScopeAll {
		filter, err := s.resolveContentEntryTypeFilter(ctx, source, req)
		if err != nil {
			return nil, err
		}
		ids, err := s.collectContentEntryIDs(ctx, source.ID.String(), req.ContentIDs, req.ContentSlugs, filter)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			item, err := s.PromoteContentEntry(ctx, PromoteContentEntryRequest{
				ContentID:         id,
				TargetEnvironment: target.Key,
				Options:           req.Options,
			})
			if err != nil {
				result.Errors = append(result.Errors, PromoteError{Kind: "content_entry", SourceID: id, Error: err.Error()})
				result.Summary.ContentEntries.Failed++
				continue
			}
			if item != nil {
				result.Items = append(result.Items, *item)
				updateSummary(&result.Summary.ContentEntries, item.Status)
			}
		}
	}

	return result, nil
}

func (s *service) PromoteContentType(ctx context.Context, req PromoteContentTypeRequest) (*PromoteItem, error) {
	if s == nil || s.contentTypes == nil {
		return nil, errors.New("promotions: content type promotion unavailable")
	}
	if req.ContentTypeID == uuid.Nil {
		return nil, content.ErrContentTypeIDRequired
	}
	opts := normalizeOptions(req.Options)

	source, err := s.contentTypes.GetByID(ctx, req.ContentTypeID)
	if err != nil {
		return nil, err
	}
	sourceEnv, err := s.resolveEnvironmentByID(ctx, source.EnvironmentID)
	if err != nil {
		return nil, err
	}
	targetEnv, err := s.resolveEnvironment(ctx, req.TargetEnvironment, req.TargetEnvironmentID)
	if err != nil {
		return nil, err
	}
	if sourceEnv.ID == targetEnv.ID {
		return &PromoteItem{Kind: "content_type", SourceID: source.ID, TargetID: source.ID, Status: "skipped", Message: "source and target environments match"}, nil
	}

	if strings.ToLower(strings.TrimSpace(source.Status)) != strings.ToLower(content.ContentTypeStatusActive) && !opts.AllowDraft {
		return nil, fmt.Errorf("promotion: content type %s is not active", source.Slug)
	}

	sourceSchema, sourceVersion, err := normalizeContentTypeSchema(source)
	if err != nil {
		return nil, err
	}

	if err := s.promoteBlockDefinitions(ctx, sourceSchema, sourceEnv, targetEnv, opts.DryRun); err != nil {
		return nil, err
	}

	target, err := s.contentTypes.GetBySlug(ctx, source.Slug, targetEnv.ID.String())
	if err != nil {
		var nf *content.NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
		now := s.now()
		record := &content.ContentType{
			ID:            s.id(),
			Name:          source.Name,
			Slug:          source.Slug,
			Description:   cloneString(source.Description),
			Schema:        cloneMap(sourceSchema),
			UISchema:      cloneMap(source.UISchema),
			Capabilities:  cloneMap(source.Capabilities),
			Icon:          cloneString(source.Icon),
			SchemaVersion: sourceVersion,
			SchemaHistory: appendSchemaHistory(cloneSchemaHistory(source.SchemaHistory), newSchemaSnapshot(source, sourceVersion, sourceSchema, now)),
			Status:        chooseContentTypeStatus(opts.PromoteAsActive),
			EnvironmentID: targetEnv.ID,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if opts.DryRun {
			return buildContentTypeItem("created", source.ID, record.ID, sourceVersion, true), nil
		}
		created, err := s.contentTypes.Create(ctx, record)
		if err != nil {
			return nil, err
		}
		s.emitPromotionActivity(ctx, "content_type", "promote", source.ID, created.ID, sourceEnv, targetEnv, opts)
		return buildContentTypeItem("created", source.ID, created.ID, sourceVersion, false), nil
	}

	if compareSchemaVersions(target.SchemaVersion, sourceVersion) > 0 && !opts.Force {
		return nil, fmt.Errorf("promotion: target schema version ahead of source")
	}

	compatibility := schema.CheckSchemaCompatibility(target.Schema, sourceSchema)
	if len(compatibility.BreakingChanges) > 0 && !opts.AllowBreakingChanges {
		return nil, content.ErrContentTypeSchemaBreaking
	}

	updated := *target
	updated.Name = source.Name
	updated.Slug = source.Slug
	updated.Description = cloneString(source.Description)
	updated.Schema = cloneMap(sourceSchema)
	updated.UISchema = cloneMap(source.UISchema)
	updated.Capabilities = cloneMap(source.Capabilities)
	updated.Icon = cloneString(source.Icon)
	updated.SchemaVersion = sourceVersion
	updated.SchemaHistory = appendSchemaHistory(cloneSchemaHistory(target.SchemaHistory), newSchemaSnapshot(source, sourceVersion, sourceSchema, s.now()))
	updated.Status = chooseContentTypeStatus(opts.PromoteAsActive)
	updated.UpdatedAt = s.now()
	updated.EnvironmentID = targetEnv.ID

	if opts.DryRun {
		return buildContentTypeItem("updated", source.ID, target.ID, sourceVersion, true), nil
	}

	saved, err := s.contentTypes.Update(ctx, &updated)
	if err != nil {
		return nil, err
	}
	s.emitPromotionActivity(ctx, "content_type", "promote", source.ID, saved.ID, sourceEnv, targetEnv, opts)
	return buildContentTypeItem("updated", source.ID, saved.ID, sourceVersion, false), nil
}

func (s *service) PromoteContentEntry(ctx context.Context, req PromoteContentEntryRequest) (*PromoteItem, error) {
	if s == nil || s.contents == nil || s.contentTypes == nil {
		return nil, errors.New("promotions: content promotion unavailable")
	}
	if req.ContentID == uuid.Nil {
		return nil, content.ErrContentIDRequired
	}
	opts := normalizeOptions(req.Options)

	sourceContent, err := s.contents.GetByID(ctx, req.ContentID)
	if err != nil {
		return nil, err
	}
	sourceType, err := s.contentTypes.GetByID(ctx, sourceContent.ContentTypeID)
	if err != nil {
		return nil, err
	}
	sourceEnv, err := s.resolveEnvironmentByID(ctx, sourceContent.EnvironmentID)
	if err != nil {
		return nil, err
	}
	targetEnv, err := s.resolveEnvironment(ctx, req.TargetEnvironment, req.TargetEnvironmentID)
	if err != nil {
		return nil, err
	}
	if sourceEnv.ID == targetEnv.ID {
		return &PromoteItem{Kind: "content_entry", SourceID: sourceContent.ID, TargetID: sourceContent.ID, Status: "skipped", Message: "source and target environments match"}, nil
	}

	targetType, err := s.contentTypes.GetBySlug(ctx, sourceType.Slug, targetEnv.ID.String())
	if err != nil {
		var nf *content.NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
		if !opts.AutoPromoteType {
			return nil, content.ErrContentTypeRequired
		}
		if opts.DryRun {
			targetType = cloneContentTypeForEnv(sourceType, targetEnv.ID)
		} else {
			if _, err := s.PromoteContentType(ctx, PromoteContentTypeRequest{
				ContentTypeID:     sourceType.ID,
				TargetEnvironment: targetEnv.Key,
				Options:           opts,
			}); err != nil {
				return nil, err
			}
			targetType, err = s.contentTypes.GetBySlug(ctx, sourceType.Slug, targetEnv.ID.String())
			if err != nil {
				return nil, err
			}
		}
	}

	sourceVersion, err := s.selectSourceVersion(ctx, sourceContent, opts)
	if err != nil {
		return nil, err
	}

	targetSchema, targetVersion, err := normalizeSchemaVersion(targetType)
	if err != nil {
		return nil, err
	}

	snapshot := cloneContentVersionSnapshot(sourceVersion.Snapshot)
	snapshot, err = s.migrateSnapshot(ctx, snapshot, sourceType, targetSchema, targetVersion, opts)
	if err != nil {
		return nil, err
	}

	targetContent, err := s.contents.GetBySlug(ctx, sourceContent.Slug, targetType.ID, targetEnv.ID.String())
	if err != nil {
		var nf *content.NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
		targetContent = nil
	}

	if targetContent != nil && opts.Mode == ModeStrict {
		return nil, content.ErrSlugExists
	}

	actor := pickActor(sourceContent.UpdatedBy, sourceContent.CreatedBy)
	now := s.now()
	created := false
	if targetContent == nil {
		created = true
		targetContent = &content.Content{
			ID:            s.id(),
			ContentTypeID: targetType.ID,
			EnvironmentID: targetEnv.ID,
			Status:        string(domain.StatusDraft),
			Slug:          sourceContent.Slug,
			CreatedBy:     actor,
			UpdatedBy:     actor,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
	}

	translations, err := s.buildTranslations(ctx, targetContent.ID, snapshot)
	if err != nil {
		return nil, err
	}

	if opts.DryRun {
		return s.finalizeContentPromotion(ctx, sourceContent, targetContent, sourceVersion, snapshot, opts, created, sourceEnv, targetEnv, true)
	}

	if created {
		targetContent.Translations = translations
		createdRecord, err := s.contents.Create(ctx, targetContent)
		if err != nil {
			return nil, err
		}
		targetContent = createdRecord
	} else {
		if err := s.contents.ReplaceTranslations(ctx, targetContent.ID, translations); err != nil {
			return nil, err
		}
		targetContent.Translations = translations
		targetContent.UpdatedAt = now
		if actor != uuid.Nil {
			targetContent.UpdatedBy = actor
		}
		if _, err := s.contents.Update(ctx, targetContent); err != nil {
			return nil, err
		}
	}

	return s.finalizeContentPromotion(ctx, sourceContent, targetContent, sourceVersion, snapshot, opts, created, sourceEnv, targetEnv, false)
}

func (s *service) finalizeContentPromotion(ctx context.Context, source *content.Content, target *content.Content, sourceVersion *content.ContentVersion, snapshot content.ContentVersionSnapshot, opts PromoteOptions, created bool, sourceEnv *cmsenv.Environment, targetEnv *cmsenv.Environment, dryRun bool) (*PromoteItem, error) {
	if target == nil {
		return nil, errors.New("promotions: target content missing")
	}
	versions, err := s.contents.ListVersions(ctx, target.ID)
	if err != nil && !dryRun {
		return nil, err
	}
	next := nextContentVersionNumber(versions)
	actor := pickActor(source.UpdatedBy, source.CreatedBy)

	if opts.IncludeVersions {
		sourceVersions, err := s.contents.ListVersions(ctx, source.ID)
		if err != nil {
			return nil, err
		}
		sort.Slice(sourceVersions, func(i, j int) bool {
			return sourceVersions[i].Version < sourceVersions[j].Version
		})
		for _, src := range sourceVersions {
			if src == nil {
				continue
			}
			if sourceVersion != nil && src.Version == sourceVersion.Version {
				continue
			}
			copied := &content.ContentVersion{
				ID:        s.id(),
				ContentID: target.ID,
				Version:   next,
				Status:    domain.StatusDraft,
				Snapshot:  cloneContentVersionSnapshot(src.Snapshot),
				CreatedBy: actor,
				CreatedAt: s.now(),
			}
			next++
			if dryRun {
				continue
			}
			if _, err := s.contents.CreateVersion(ctx, copied); err != nil {
				return nil, err
			}
		}
	}

	promoted := &content.ContentVersion{
		ID:        s.id(),
		ContentID: target.ID,
		Version:   next,
		Status:    domain.StatusDraft,
		Snapshot:  cloneContentVersionSnapshot(snapshot),
		CreatedBy: actor,
		CreatedAt: s.now(),
	}
	if opts.PromoteAsPublished {
		promoted.Status = domain.StatusPublished
		publishedAt := s.now()
		promoted.PublishedAt = &publishedAt
		if actor != uuid.Nil {
			promoted.PublishedBy = &actor
		}
	}
	if !dryRun {
		if _, err := s.contents.CreateVersion(ctx, promoted); err != nil {
			return nil, err
		}
		updated, err := s.updateContentRecord(ctx, target, promoted, opts, actor)
		if err != nil {
			return nil, err
		}
		target = updated
		s.emitPromotionActivity(ctx, "content_entry", "promote", source.ID, target.ID, sourceEnv, targetEnv, opts)
	}

	status := "updated"
	if created {
		status = "created"
	}
	return buildContentEntryItem(status, source.ID, target.ID, targetSchemaVersion(snapshot), dryRun), nil
}

func (s *service) updateContentRecord(ctx context.Context, record *content.Content, version *content.ContentVersion, opts PromoteOptions, actor uuid.UUID) (*content.Content, error) {
	if record == nil || version == nil {
		return record, nil
	}
	updated := *record
	updated.CurrentVersion = maxInt(updated.CurrentVersion, version.Version)
	updated.UpdatedAt = s.now()
	if actor != uuid.Nil {
		updated.UpdatedBy = actor
	}
	if opts.PromoteAsPublished {
		updated.Status = string(domain.StatusPublished)
		updated.PublishedVersion = &version.Version
		updated.PublishedAt = cloneTimePtr(version.PublishedAt)
		updated.PublishedBy = cloneUUIDPtr(version.PublishedBy)
		if record.PublishedVersion != nil && *record.PublishedVersion != version.Version {
			if previous, err := s.contents.GetVersion(ctx, record.ID, *record.PublishedVersion); err == nil && previous != nil {
				previous.Status = domain.StatusArchived
				_, _ = s.contents.UpdateVersion(ctx, previous)
			}
		}
	} else if updated.PublishedVersion == nil {
		updated.Status = string(domain.StatusDraft)
	}
	if version.Snapshot.Metadata != nil {
		updated.Metadata = cloneMap(version.Snapshot.Metadata)
	}
	saved, err := s.contents.Update(ctx, &updated)
	if err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *service) selectSourceVersion(ctx context.Context, contentRecord *content.Content, opts PromoteOptions) (*content.ContentVersion, error) {
	if contentRecord == nil {
		return nil, content.ErrContentIDRequired
	}
	preferPublished := boolValue(opts.PreferPublished, true)
	if contentRecord.PublishedVersion != nil && (preferPublished || !opts.AllowDraft) {
		return s.contents.GetVersion(ctx, contentRecord.ID, *contentRecord.PublishedVersion)
	}
	if !opts.AllowDraft {
		return nil, content.ErrContentVersionRequired
	}
	if !preferPublished {
		return s.contents.GetLatestVersion(ctx, contentRecord.ID)
	}
	if contentRecord.PublishedVersion != nil {
		return s.contents.GetVersion(ctx, contentRecord.ID, *contentRecord.PublishedVersion)
	}
	return s.contents.GetLatestVersion(ctx, contentRecord.ID)
}

func (s *service) migrateSnapshot(ctx context.Context, snapshot content.ContentVersionSnapshot, sourceType *content.ContentType, targetSchema map[string]any, targetVersion schema.Version, opts PromoteOptions) (content.ContentVersionSnapshot, error) {
	current, ok := snapshotSchemaVersion(snapshot)
	if !ok {
		_, version, err := normalizeSchemaVersion(sourceType)
		if err == nil {
			current = version
			ok = true
		}
	}
	if ok && current.String() == targetVersion.String() {
		return applySnapshotSchemaVersion(snapshot, targetVersion), nil
	}
	if !boolValue(opts.MigrateOnPromote, true) {
		return snapshot, content.ErrContentSchemaMigrationRequired
	}
	if s.schemaMigrator == nil {
		return snapshot, content.ErrContentSchemaMigrationRequired
	}
	if !ok {
		return snapshot, content.ErrContentSchemaMigrationRequired
	}
	migrated := cloneContentVersionSnapshot(snapshot)
	for idx, tr := range migrated.Translations {
		payload := tr.Content
		if payload == nil {
			payload = map[string]any{}
		}
		trimmed := stripSchemaVersion(payload)
		updated, err := s.schemaMigrator.Migrate(sourceType.Slug, current.String(), targetVersion.String(), trimmed)
		if err != nil {
			return snapshot, fmt.Errorf("%w: %v", content.ErrContentSchemaMigrationRequired, err)
		}
		clean := stripSchemaVersion(updated)
		if err := validation.ValidateMigrationPayload(targetSchema, content.SanitizeEmbeddedBlocks(clean)); err != nil {
			return snapshot, fmt.Errorf("%w: %s", content.ErrContentSchemaInvalid, err)
		}
		migrated.Translations[idx].Content = applySchemaVersion(clean, targetVersion)
	}
	if s.embeddedBlocks != nil {
		migratedBlocks, err := s.migrateEmbeddedBlocksSnapshot(ctx, migrated)
		if err != nil {
			return snapshot, err
		}
		migrated = migratedBlocks
	}
	return migrated, nil
}

func (s *service) migrateEmbeddedBlocksSnapshot(ctx context.Context, snapshot content.ContentVersionSnapshot) (content.ContentVersionSnapshot, error) {
	if s.embeddedBlocks == nil || len(snapshot.Translations) == 0 {
		return snapshot, nil
	}
	updated := cloneContentVersionSnapshot(snapshot)
	for idx, tr := range updated.Translations {
		blocksPayload, ok := content.ExtractEmbeddedBlocks(tr.Content)
		if !ok || len(blocksPayload) == 0 {
			continue
		}
		migrated, err := s.embeddedBlocks.MigrateEmbeddedBlocks(ctx, tr.Locale, blocksPayload)
		if err != nil {
			return snapshot, err
		}
		updated.Translations[idx].Content = content.MergeEmbeddedBlocks(tr.Content, migrated)
	}
	return updated, nil
}

func (s *service) buildTranslations(ctx context.Context, contentID uuid.UUID, snapshot content.ContentVersionSnapshot) ([]*content.ContentTranslation, error) {
	if len(snapshot.Translations) == 0 {
		return nil, nil
	}
	now := s.now()
	groupID := contentID
	out := make([]*content.ContentTranslation, 0, len(snapshot.Translations))
	for _, tr := range snapshot.Translations {
		code := strings.TrimSpace(tr.Locale)
		if code == "" {
			return nil, content.ErrUnknownLocale
		}
		locale, err := s.locales.GetByCode(ctx, code)
		if err != nil {
			return nil, content.ErrUnknownLocale
		}
		entry := &content.ContentTranslation{
			ID:                 s.id(),
			ContentID:          contentID,
			LocaleID:           locale.ID,
			TranslationGroupID: &groupID,
			Title:              tr.Title,
			Summary:            cloneString(tr.Summary),
			Content:            cloneMap(tr.Content),
			CreatedAt:          now,
			UpdatedAt:          now,
			Locale:             locale,
		}
		out = append(out, entry)
	}
	return out, nil
}

func (s *service) promoteBlockDefinitions(ctx context.Context, schemaPayload map[string]any, source *cmsenv.Environment, target *cmsenv.Environment, dryRun bool) error {
	slugs := extractBlockSlugs(schemaPayload)
	if len(slugs) == 0 {
		return nil
	}
	if s.blocks == nil {
		return fmt.Errorf("promotions: block service required")
	}
	sourceDefs, err := s.blocks.ListDefinitions(ctx, source.Key)
	if err != nil {
		return err
	}
	targetDefs, err := s.blocks.ListDefinitions(ctx, target.Key)
	if err != nil {
		return err
	}
	sourceIndex := map[string]*blocks.Definition{}
	for _, def := range sourceDefs {
		if def == nil {
			continue
		}
		sourceIndex[strings.ToLower(strings.TrimSpace(def.Slug))] = def
	}
	targetIndex := map[string]*blocks.Definition{}
	for _, def := range targetDefs {
		if def == nil {
			continue
		}
		targetIndex[strings.ToLower(strings.TrimSpace(def.Slug))] = def
	}
	for _, slug := range slugs {
		key := strings.ToLower(strings.TrimSpace(slug))
		if key == "" {
			continue
		}
		sourceDef := sourceIndex[key]
		if sourceDef == nil {
			return fmt.Errorf("promotion: block definition %s not found", slug)
		}
		preparedSchema, _, err := normalizeDefinitionSchema(sourceDef)
		if err != nil {
			return err
		}
		if existing := targetIndex[key]; existing != nil {
			if dryRun {
				continue
			}
			name := sourceDef.Name
			slugValue := sourceDef.Slug
			desc := cloneString(sourceDef.Description)
			icon := cloneString(sourceDef.Icon)
			cat := cloneString(sourceDef.Category)
			status := sourceDef.Status
			uiSchema := cloneMap(sourceDef.UISchema)
			defaults := cloneMap(sourceDef.Defaults)
			editorURL := cloneString(sourceDef.EditorStyleURL)
			frontendURL := cloneString(sourceDef.FrontendStyleURL)
			_, err := s.blocks.UpdateDefinition(ctx, blocks.UpdateDefinitionInput{
				ID:               existing.ID,
				Name:             &name,
				Slug:             &slugValue,
				Description:      desc,
				Icon:             icon,
				Category:         cat,
				Status:           &status,
				Schema:           preparedSchema,
				UISchema:         uiSchema,
				Defaults:         defaults,
				EditorStyleURL:   editorURL,
				FrontendStyleURL: frontendURL,
			})
			if err != nil {
				return err
			}
			continue
		}
		if dryRun {
			continue
		}
		_, err = s.blocks.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
			Name:             sourceDef.Name,
			Slug:             sourceDef.Slug,
			Description:      cloneString(sourceDef.Description),
			Icon:             cloneString(sourceDef.Icon),
			Category:         cloneString(sourceDef.Category),
			Status:           sourceDef.Status,
			Schema:           preparedSchema,
			UISchema:         cloneMap(sourceDef.UISchema),
			Defaults:         cloneMap(sourceDef.Defaults),
			EditorStyleURL:   cloneString(sourceDef.EditorStyleURL),
			FrontendStyleURL: cloneString(sourceDef.FrontendStyleURL),
			EnvironmentKey:   target.Key,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *service) resolveEnvironment(ctx context.Context, key string, id *uuid.UUID) (*cmsenv.Environment, error) {
	if s == nil || s.envs == nil {
		return nil, errors.New("promotions: environment service unavailable")
	}
	if id != nil && *id != uuid.Nil {
		env, err := s.envs.GetEnvironment(ctx, *id)
		if err != nil {
			return nil, err
		}
		if !env.IsActive {
			return nil, cmsenv.ErrEnvironmentNotFound
		}
		return env, nil
	}
	trimmed := strings.TrimSpace(key)
	if trimmed != "" {
		if parsed, err := uuid.Parse(trimmed); err == nil {
			return s.resolveEnvironment(ctx, "", &parsed)
		}
	}
	normalized := cmsenv.NormalizeKey(trimmed)
	if normalized == "" {
		normalized = cmsenv.NormalizeKey(s.defaultEnvKey)
	}
	if normalized == "" {
		return nil, cmsenv.ErrEnvironmentNotFound
	}
	env, err := s.envs.GetEnvironmentByKey(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if !env.IsActive {
		return nil, cmsenv.ErrEnvironmentNotFound
	}
	return env, nil
}

func (s *service) resolveEnvironmentByID(ctx context.Context, id uuid.UUID) (*cmsenv.Environment, error) {
	if id == uuid.Nil {
		return s.resolveEnvironment(ctx, "", nil)
	}
	return s.resolveEnvironment(ctx, "", &id)
}

func (s *service) resolveContentEntryTypeFilter(ctx context.Context, source *cmsenv.Environment, req PromoteEnvironmentRequest) (*uuid.UUID, error) {
	if s == nil || s.contentTypes == nil {
		if len(req.ContentSlugs) > 0 || req.ContentEntryTypeID != nil || strings.TrimSpace(req.ContentEntryTypeSlug) != "" {
			return nil, errors.New("promotions: content type repository unavailable")
		}
		return nil, nil
	}
	hasSlugs := len(req.ContentSlugs) > 0
	rawSlug := strings.TrimSpace(req.ContentEntryTypeSlug)
	rawID := req.ContentEntryTypeID
	if hasSlugs && (rawID == nil || *rawID == uuid.Nil) && rawSlug == "" {
		return nil, ErrContentTypeRequiredForSlugs
	}

	var byID *content.ContentType
	var bySlug *content.ContentType
	if rawID != nil && *rawID != uuid.Nil {
		ct, err := s.contentTypes.GetByID(ctx, *rawID)
		if err != nil {
			return nil, err
		}
		if source != nil && ct.EnvironmentID != source.ID {
			return nil, ErrContentTypeEnvMismatch
		}
		byID = ct
	}
	if rawSlug != "" {
		if source == nil {
			return nil, ErrContentTypeRequiredForSlugs
		}
		ct, err := s.contentTypes.GetBySlug(ctx, rawSlug, source.ID.String())
		if err != nil {
			return nil, err
		}
		bySlug = ct
	}
	if byID != nil && bySlug != nil && byID.ID != bySlug.ID {
		return nil, ErrContentTypeFilterMismatch
	}
	if byID != nil {
		id := byID.ID
		return &id, nil
	}
	if bySlug != nil {
		id := bySlug.ID
		return &id, nil
	}
	return nil, nil
}

func (s *service) collectContentTypeIDs(ctx context.Context, envID string, ids []uuid.UUID, slugs []string) ([]uuid.UUID, error) {
	if len(ids) > 0 || len(slugs) > 0 {
		if len(slugs) == 0 {
			return ids, nil
		}
		records, err := s.contentTypes.List(ctx, envID)
		if err != nil {
			return nil, err
		}
		index := map[string]uuid.UUID{}
		for _, record := range records {
			if record == nil {
				continue
			}
			index[strings.ToLower(strings.TrimSpace(record.Slug))] = record.ID
		}
		for _, slug := range slugs {
			key := strings.ToLower(strings.TrimSpace(slug))
			if key == "" {
				continue
			}
			if id, ok := index[key]; ok {
				ids = append(ids, id)
			}
		}
		return ids, nil
	}
	records, err := s.contentTypes.List(ctx, envID)
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		out = append(out, record.ID)
	}
	return out, nil
}

func (s *service) collectContentEntryIDs(ctx context.Context, envID string, ids []uuid.UUID, slugs []string, typeID *uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) > 0 || len(slugs) > 0 {
		if len(slugs) == 0 {
			return ids, nil
		}
		records, err := s.contents.List(ctx, envID)
		if err != nil {
			return nil, err
		}
		index := map[string][]uuid.UUID{}
		for _, record := range records {
			if record == nil {
				continue
			}
			if typeID != nil && *typeID != uuid.Nil && record.ContentTypeID != *typeID {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(record.Slug))
			if key == "" {
				continue
			}
			index[key] = append(index[key], record.ID)
		}
		for _, slug := range slugs {
			key := strings.ToLower(strings.TrimSpace(slug))
			if key == "" {
				continue
			}
			if matches, ok := index[key]; ok {
				ids = append(ids, matches...)
			}
		}
		return ids, nil
	}
	records, err := s.contents.List(ctx, envID)
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		if typeID != nil && *typeID != uuid.Nil && record.ContentTypeID != *typeID {
			continue
		}
		out = append(out, record.ID)
	}
	return out, nil
}

func normalizeOptions(opts PromoteOptions) PromoteOptions {
	if opts.Mode == "" {
		opts.Mode = ModeStrict
	}
	if opts.PreferPublished == nil {
		opts.PreferPublished = boolPtr(true)
	}
	if opts.MigrateOnPromote == nil {
		opts.MigrateOnPromote = boolPtr(true)
	}
	return opts
}

func extractBlockSlugs(schemaPayload map[string]any) []string {
	meta := schema.ExtractMetadata(schemaPayload)
	if meta.BlockAvailability.Empty() {
		return nil
	}
	if len(meta.BlockAvailability.Allow) > 0 {
		return append([]string(nil), meta.BlockAvailability.Allow...)
	}
	if len(meta.BlockAvailability.Deny) > 0 {
		return append([]string(nil), meta.BlockAvailability.Deny...)
	}
	return nil
}

func normalizeContentTypeSchema(source *content.ContentType) (map[string]any, string, error) {
	if source == nil {
		return nil, "", content.ErrContentTypeRequired
	}
	schemaPayload := cloneMap(source.Schema)
	version := strings.TrimSpace(source.SchemaVersion)
	if version == "" {
		normalized, parsed, err := schema.EnsureSchemaVersion(schemaPayload, source.Slug)
		if err != nil {
			return nil, "", err
		}
		return normalized, parsed.String(), nil
	}
	meta := schema.ExtractMetadata(schemaPayload)
	meta.Slug = source.Slug
	meta.SchemaVersion = version
	return schema.ApplyMetadata(schemaPayload, meta), version, nil
}

func normalizeDefinitionSchema(definition *blocks.Definition) (map[string]any, string, error) {
	if definition == nil {
		return nil, "", errors.New("promotions: definition required")
	}
	schemaPayload := cloneMap(definition.Schema)
	version := strings.TrimSpace(definition.SchemaVersion)
	if version == "" {
		normalized, parsed, err := schema.EnsureSchemaVersion(schemaPayload, definition.Slug)
		if err != nil {
			return nil, "", err
		}
		return normalized, parsed.String(), nil
	}
	meta := schema.ExtractMetadata(schemaPayload)
	meta.Slug = definition.Slug
	meta.SchemaVersion = version
	return schema.ApplyMetadata(schemaPayload, meta), version, nil
}

func normalizeSchemaVersion(ct *content.ContentType) (map[string]any, schema.Version, error) {
	if ct == nil {
		return nil, schema.Version{}, content.ErrContentTypeRequired
	}
	normalized, version, err := schema.EnsureSchemaVersion(cloneMap(ct.Schema), ct.Slug)
	if err != nil {
		return nil, schema.Version{}, err
	}
	return normalized, version, nil
}

func chooseContentTypeStatus(promoteAsActive bool) string {
	if promoteAsActive {
		return content.ContentTypeStatusActive
	}
	return content.ContentTypeStatusDraft
}

func buildContentTypeItem(status string, sourceID, targetID uuid.UUID, version string, dryRun bool) *PromoteItem {
	details := map[string]any{"schema_version": version}
	if dryRun {
		details["dry_run"] = true
	}
	return &PromoteItem{
		Kind:     "content_type",
		SourceID: sourceID,
		TargetID: targetID,
		Status:   status,
		Details:  details,
	}
}

func buildContentEntryItem(status string, sourceID, targetID uuid.UUID, version string, dryRun bool) *PromoteItem {
	details := map[string]any{}
	if version != "" {
		details["schema_version"] = version
	}
	if dryRun {
		details["dry_run"] = true
	}
	return &PromoteItem{
		Kind:     "content_entry",
		SourceID: sourceID,
		TargetID: targetID,
		Status:   status,
		Details:  details,
	}
}

func updateSummary(counts *PromoteSummaryCounts, status string) {
	if counts == nil {
		return
	}
	switch status {
	case "created":
		counts.Created++
	case "updated":
		counts.Updated++
	case "skipped":
		counts.Skipped++
	case "failed":
		counts.Failed++
	}
}

func cloneContentTypeForEnv(source *content.ContentType, envID uuid.UUID) *content.ContentType {
	if source == nil {
		return nil
	}
	cloned := *source
	cloned.ID = uuid.New()
	cloned.EnvironmentID = envID
	cloned.Schema = cloneMap(source.Schema)
	cloned.UISchema = cloneMap(source.UISchema)
	cloned.Capabilities = cloneMap(source.Capabilities)
	cloned.SchemaHistory = cloneSchemaHistory(source.SchemaHistory)
	return &cloned
}

func newSchemaSnapshot(source *content.ContentType, version string, schemaPayload map[string]any, now time.Time) content.ContentTypeSchemaSnapshot {
	if source == nil {
		return content.ContentTypeSchemaSnapshot{}
	}
	return content.ContentTypeSchemaSnapshot{
		Version:      version,
		Schema:       cloneMap(schemaPayload),
		UISchema:     cloneMap(source.UISchema),
		Capabilities: cloneMap(source.Capabilities),
		Status:       source.Status,
		UpdatedAt:    now,
	}
}

func appendSchemaHistory(history []content.ContentTypeSchemaSnapshot, snapshot content.ContentTypeSchemaSnapshot) []content.ContentTypeSchemaSnapshot {
	if snapshot.Version == "" {
		return history
	}
	if len(history) == 0 {
		return []content.ContentTypeSchemaSnapshot{snapshot}
	}
	last := history[len(history)-1]
	if last.Version == snapshot.Version {
		history[len(history)-1] = snapshot
		return history
	}
	return append(history, snapshot)
}

func cloneSchemaHistory(history []content.ContentTypeSchemaSnapshot) []content.ContentTypeSchemaSnapshot {
	if len(history) == 0 {
		return nil
	}
	out := make([]content.ContentTypeSchemaSnapshot, len(history))
	for i, snapshot := range history {
		out[i] = content.ContentTypeSchemaSnapshot{
			Version:      snapshot.Version,
			Schema:       cloneMap(snapshot.Schema),
			UISchema:     cloneMap(snapshot.UISchema),
			Capabilities: cloneMap(snapshot.Capabilities),
			Status:       snapshot.Status,
			UpdatedAt:    snapshot.UpdatedAt,
		}
		if snapshot.UpdatedBy != nil {
			actor := *snapshot.UpdatedBy
			out[i].UpdatedBy = &actor
		}
	}
	return out
}

func cloneContentVersionSnapshot(snapshot content.ContentVersionSnapshot) content.ContentVersionSnapshot {
	cloned := content.ContentVersionSnapshot{
		Fields:       cloneMap(snapshot.Fields),
		Metadata:     cloneMap(snapshot.Metadata),
		Translations: nil,
	}
	if len(snapshot.Translations) == 0 {
		return cloned
	}
	translations := make([]content.ContentVersionTranslationSnapshot, len(snapshot.Translations))
	for i, tr := range snapshot.Translations {
		translations[i] = content.ContentVersionTranslationSnapshot{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: cloneString(tr.Summary),
			Content: cloneMap(tr.Content),
		}
	}
	cloned.Translations = translations
	return cloned
}

func applySnapshotSchemaVersion(snapshot content.ContentVersionSnapshot, version schema.Version) content.ContentVersionSnapshot {
	updated := cloneContentVersionSnapshot(snapshot)
	for idx, tr := range updated.Translations {
		payload := tr.Content
		if payload == nil {
			payload = map[string]any{}
		}
		updated.Translations[idx].Content = applySchemaVersion(stripSchemaVersion(payload), version)
	}
	return updated
}

func snapshotSchemaVersion(snapshot content.ContentVersionSnapshot) (schema.Version, bool) {
	if len(snapshot.Translations) > 0 {
		for _, tr := range snapshot.Translations {
			if v, ok := schema.RootSchemaVersion(tr.Content); ok {
				return v, true
			}
		}
	}
	if v, ok := schema.RootSchemaVersion(snapshot.Fields); ok {
		return v, true
	}
	return schema.Version{}, false
}

func stripSchemaVersion(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	clean := cloneMap(payload)
	delete(clean, schema.RootSchemaKey)
	return clean
}

func applySchemaVersion(payload map[string]any, version schema.Version) map[string]any {
	result := cloneMap(payload)
	if result == nil {
		result = map[string]any{}
	}
	result[schema.RootSchemaKey] = version.String()
	return result
}

func compareSchemaVersions(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	av, errA := schema.ParseVersion(a)
	bv, errB := schema.ParseVersion(b)
	if errA != nil || errB != nil {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
	return compareSemVer(av.SemVer, bv.SemVer)
}

func compareSemVer(a, b string) int {
	am, an, ap, okA := semverParts(a)
	bm, bn, bp, okB := semverParts(b)
	if !okA || !okB {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
	if am != bm {
		if am < bm {
			return -1
		}
		return 1
	}
	if an != bn {
		if an < bn {
			return -1
		}
		return 1
	}
	if ap != bp {
		if ap < bp {
			return -1
		}
		return 1
	}
	return 0
}

func semverParts(value string) (int, int, int, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "v")
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	major, err := parseInt(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err := parseInt(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	patch, err := parseInt(parts[2])
	if err != nil {
		return 0, 0, 0, false
	}
	return major, minor, patch, true
}

func parseInt(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, fmt.Errorf("empty")
	}
	var out int
	_, err := fmt.Sscanf(value, "%d", &out)
	return out, err
}

func nextContentVersionNumber(records []*content.ContentVersion) int {
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func pickActor(ids ...uuid.UUID) uuid.UUID {
	for _, id := range ids {
		if id != uuid.Nil {
			return id
		}
	}
	return uuid.Nil
}

func boolPtr(value bool) *bool {
	v := value
	return &v
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func targetSchemaVersion(snapshot content.ContentVersionSnapshot) string {
	if version, ok := snapshotSchemaVersion(snapshot); ok {
		return version.String()
	}
	return ""
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneUUIDPtr(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
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

func (s *service) emitPromotionActivity(ctx context.Context, objectType, verb string, sourceID, targetID uuid.UUID, sourceEnv *cmsenv.Environment, targetEnv *cmsenv.Environment, opts PromoteOptions) {
	if s == nil || s.activity == nil || !s.activity.Enabled() {
		return
	}
	meta := map[string]any{}
	if sourceEnv != nil {
		meta["source_environment_id"] = sourceEnv.ID.String()
		meta["source_environment_key"] = sourceEnv.Key
	}
	if targetEnv != nil {
		meta["target_environment_id"] = targetEnv.ID.String()
		meta["target_environment_key"] = targetEnv.Key
	}
	meta["promotion_mode"] = opts.Mode
	meta["promote_as_active"] = opts.PromoteAsActive
	meta["promote_as_published"] = opts.PromoteAsPublished
	_ = s.activity.Emit(ctx, activity.Event{
		Verb:       verb,
		ObjectType: objectType,
		ObjectID:   targetID.String(),
		Metadata:   meta,
	})
}
