package blocks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/identity"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/media"
	cmsschema "github.com/goliatone/go-cms/internal/schema"
	"github.com/goliatone/go-cms/internal/validation"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

var (
	ErrEmbeddedBlockTypeRequired      = errors.New("blocks: embedded block type required")
	ErrEmbeddedBlockDefinitionMissing = errors.New("blocks: embedded block definition missing")
	ErrEmbeddedBlocksBridgeUnwired    = errors.New("blocks: embedded blocks bridge is not configured")
)

// ContentPageResolver maps content entries to page identifiers.
type ContentPageResolver interface {
	PageIDsForContent(ctx context.Context, contentID uuid.UUID) ([]uuid.UUID, error)
}

// EmbeddedBlockBridge coordinates embedded block payloads with legacy block instances.
type EmbeddedBlockBridge struct {
	blocks            Service
	locales           content.LocaleRepository
	contentRepo       content.ContentRepository
	translationReader content.ContentTranslationReader
	pageResolver      ContentPageResolver
	logger            interfaces.Logger
	defaultRegion     string
	defaultLocale     string
	clock             func() time.Time
}

// EmbeddedBlockBridgeOption customises the embedded blocks bridge.
type EmbeddedBlockBridgeOption func(*EmbeddedBlockBridge)

// WithEmbeddedBlocksLogger overrides the logger used by the bridge.
func WithEmbeddedBlocksLogger(logger interfaces.Logger) EmbeddedBlockBridgeOption {
	return func(b *EmbeddedBlockBridge) {
		if logger != nil {
			b.logger = logger
		}
	}
}

// WithEmbeddedBlocksContentRepository wires a content repository for backfill/reporting.
func WithEmbeddedBlocksContentRepository(repo content.ContentRepository) EmbeddedBlockBridgeOption {
	return func(b *EmbeddedBlockBridge) {
		if repo != nil {
			b.contentRepo = repo
			if reader, ok := repo.(content.ContentTranslationReader); ok {
				b.translationReader = reader
			}
		}
	}
}

// WithEmbeddedBlocksTranslationReader overrides the translation reader used for hydration.
func WithEmbeddedBlocksTranslationReader(reader content.ContentTranslationReader) EmbeddedBlockBridgeOption {
	return func(b *EmbeddedBlockBridge) {
		if reader != nil {
			b.translationReader = reader
		}
	}
}

// WithEmbeddedBlocksDefaultRegion overrides the region used for embedded blocks.
func WithEmbeddedBlocksDefaultRegion(region string) EmbeddedBlockBridgeOption {
	return func(b *EmbeddedBlockBridge) {
		if strings.TrimSpace(region) != "" {
			b.defaultRegion = strings.TrimSpace(region)
		}
	}
}

// WithEmbeddedBlocksDefaultLocale overrides the locale used as base for structure.
func WithEmbeddedBlocksDefaultLocale(locale string) EmbeddedBlockBridgeOption {
	return func(b *EmbeddedBlockBridge) {
		if strings.TrimSpace(locale) != "" {
			b.defaultLocale = strings.TrimSpace(locale)
		}
	}
}

// WithEmbeddedBlocksClock overrides the clock used for synthetic timestamps.
func WithEmbeddedBlocksClock(clock func() time.Time) EmbeddedBlockBridgeOption {
	return func(b *EmbeddedBlockBridge) {
		if clock != nil {
			b.clock = clock
		}
	}
}

// NewEmbeddedBlockBridge constructs a bridge for embedded blocks.
func NewEmbeddedBlockBridge(blocksSvc Service, locales content.LocaleRepository, resolver ContentPageResolver, opts ...EmbeddedBlockBridgeOption) *EmbeddedBlockBridge {
	bridge := &EmbeddedBlockBridge{
		blocks:        blocksSvc,
		locales:       locales,
		pageResolver:  resolver,
		logger:        logging.ModuleLogger(nil, "cms.blocks.embedded"),
		defaultRegion: "blocks",
		clock:         time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(bridge)
		}
	}
	return bridge
}

// SyncEmbeddedBlocks projects embedded blocks into legacy block instances (dual-write).
func (b *EmbeddedBlockBridge) SyncEmbeddedBlocks(ctx context.Context, contentID uuid.UUID, translations []content.ContentTranslationInput, actor uuid.UUID) error {
	if b == nil || b.blocks == nil || b.pageResolver == nil {
		return nil
	}
	if contentID == uuid.Nil {
		return ErrEmbeddedBlocksBridgeUnwired
	}
	byLocale := b.collectEmbeddedBlocks(translations)
	if len(byLocale) == 0 {
		return nil
	}
	baseLocale, baseBlocks := pickBaseLocale(byLocale, b.defaultLocale)
	if len(baseBlocks) == 0 {
		return nil
	}
	pageIDs, err := b.pageResolver.PageIDsForContent(ctx, contentID)
	if err != nil {
		return err
	}
	for _, pageID := range pageIDs {
		if pageID == uuid.Nil {
			continue
		}
		if err := b.syncPageBlocks(ctx, pageID, baseLocale, byLocale, actor); err != nil {
			return err
		}
	}
	return nil
}

// MergeLegacyBlocks populates embedded block payloads when missing.
func (b *EmbeddedBlockBridge) MergeLegacyBlocks(ctx context.Context, record *content.Content) error {
	if b == nil || b.blocks == nil || b.pageResolver == nil || record == nil {
		return nil
	}
	translations := record.Translations
	if len(translations) == 0 && b.translationReader != nil {
		list, err := b.translationReader.ListTranslations(ctx, record.ID)
		if err != nil && !errors.Is(err, content.ErrContentTranslationLookupUnsupported) {
			return err
		}
		if len(list) > 0 {
			translations = list
		}
	}
	if len(translations) == 0 {
		return nil
	}
	missing := make(map[uuid.UUID]*content.ContentTranslation)
	for _, tr := range translations {
		if tr == nil {
			continue
		}
		if _, ok := content.ExtractEmbeddedBlocks(tr.Content); ok {
			continue
		}
		missing[tr.LocaleID] = tr
	}
	if len(missing) == 0 {
		record.Translations = translations
		return nil
	}
	pageIDs, err := b.pageResolver.PageIDsForContent(ctx, record.ID)
	if err != nil {
		return err
	}
	if len(pageIDs) == 0 {
		return nil
	}
	embeddedByLocale, err := b.embeddedBlocksFromLegacy(ctx, pageIDs[0])
	if err != nil {
		return err
	}
	for _, tr := range translations {
		if tr == nil {
			continue
		}
		if _, ok := missing[tr.LocaleID]; !ok {
			continue
		}
		blocks := embeddedByLocale[tr.LocaleID]
		if len(blocks) == 0 {
			continue
		}
		tr.Content = content.MergeEmbeddedBlocks(tr.Content, blocks)
	}
	record.Translations = translations
	return nil
}

// MigrateEmbeddedBlocks upgrades embedded blocks to the latest schema version.
func (b *EmbeddedBlockBridge) MigrateEmbeddedBlocks(ctx context.Context, locale string, blocks []map[string]any) ([]map[string]any, error) {
	if b == nil || b.blocks == nil {
		return nil, ErrEmbeddedBlocksBridgeUnwired
	}
	if len(blocks) == 0 {
		return blocks, nil
	}
	defsByName, defsByID, err := b.definitionLookups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(blocks))
	for idx, block := range blocks {
		if block == nil {
			out = append(out, block)
			continue
		}
		blockType := strings.TrimSpace(embeddedBlockType(block))
		if blockType == "" {
			return nil, &content.EmbeddedBlockValidationError{
				Mode: content.EmbeddedBlockValidationStrict,
				Issues: []content.EmbeddedBlockValidationIssue{{
					Locale:  locale,
					Index:   idx,
					Message: ErrEmbeddedBlockTypeRequired.Error(),
				}},
			}
		}
		def := defsByName[strings.ToLower(blockType)]
		if def == nil {
			return nil, &content.EmbeddedBlockValidationError{
				Mode: content.EmbeddedBlockValidationStrict,
				Issues: []content.EmbeddedBlockValidationIssue{{
					Locale:  locale,
					Index:   idx,
					Type:    blockType,
					Message: ErrEmbeddedBlockDefinitionMissing.Error(),
				}},
			}
		}
		if err := b.validateImmutableType(ctx, locale, idx, blockType, block, defsByID); err != nil {
			return nil, err
		}
		migrated, err := b.migrateEmbeddedBlock(def, block)
		if err != nil {
			return nil, err
		}
		out = append(out, migrated)
	}
	return out, nil
}

// ValidateEmbeddedBlocks validates embedded blocks against their schemas.
func (b *EmbeddedBlockBridge) ValidateEmbeddedBlocks(ctx context.Context, locale string, blocks []map[string]any, mode content.EmbeddedBlockValidationMode) error {
	if b == nil || b.blocks == nil {
		return ErrEmbeddedBlocksBridgeUnwired
	}
	if len(blocks) == 0 {
		return nil
	}
	defsByName, defsByID, err := b.definitionLookups(ctx)
	if err != nil {
		return err
	}
	issues := []content.EmbeddedBlockValidationIssue{}
	for idx, block := range blocks {
		if block == nil {
			continue
		}
		blockType := strings.TrimSpace(embeddedBlockType(block))
		if blockType == "" {
			issues = append(issues, content.EmbeddedBlockValidationIssue{
				Locale:  locale,
				Index:   idx,
				Message: ErrEmbeddedBlockTypeRequired.Error(),
			})
			continue
		}
		def := defsByName[strings.ToLower(blockType)]
		if def == nil {
			issues = append(issues, content.EmbeddedBlockValidationIssue{
				Locale:  locale,
				Index:   idx,
				Type:    blockType,
				Message: ErrEmbeddedBlockDefinitionMissing.Error(),
			})
			continue
		}
		if err := b.appendImmutableTypeIssue(ctx, locale, idx, blockType, block, defsByID, &issues); err != nil {
			return err
		}
		rawSchema, schemaVersion, err := b.schemaForEmbeddedBlock(ctx, def, block)
		if err != nil {
			issues = append(issues, content.EmbeddedBlockValidationIssue{
				Locale:  locale,
				Index:   idx,
				Type:    blockType,
				Schema:  schemaVersion,
				Message: err.Error(),
			})
			continue
		}
		payload := sanitizeEmbeddedBlockPayload(block)
		normalizedSchema := validation.NormalizeSchema(rawSchema)
		if normalizedSchema == nil {
			issues = append(issues, content.EmbeddedBlockValidationIssue{
				Locale:  locale,
				Index:   idx,
				Type:    blockType,
				Schema:  schemaVersion,
				Message: ErrEmbeddedBlockDefinitionMissing.Error(),
			})
			continue
		}
		normalizedSchema = ensureEmbeddedTypeInSchema(normalizedSchema, blockType)
		var validationErr error
		switch mode {
		case content.EmbeddedBlockValidationDraft:
			validationErr = validation.ValidatePartialPayload(normalizedSchema, payload)
		default:
			validationErr = validation.ValidatePayload(normalizedSchema, payload)
		}
		if validationErr == nil {
			continue
		}
		validationIssues := validation.Issues(validationErr)
		for _, issue := range validationIssues {
			issues = append(issues, content.EmbeddedBlockValidationIssue{
				Locale:  locale,
				Index:   idx,
				Type:    blockType,
				Schema:  schemaVersion,
				Field:   issue.Location,
				Message: issue.Message,
			})
		}
		if len(validationIssues) == 0 {
			issues = append(issues, content.EmbeddedBlockValidationIssue{
				Locale:  locale,
				Index:   idx,
				Type:    blockType,
				Schema:  schemaVersion,
				Message: validationErr.Error(),
			})
		}
	}
	if len(issues) == 0 {
		return nil
	}
	return &content.EmbeddedBlockValidationError{
		Mode:   mode,
		Issues: issues,
	}
}

// ValidateBlockAvailability enforces content-type block availability rules.
func (b *EmbeddedBlockBridge) ValidateBlockAvailability(_ context.Context, contentType string, availability cmsschema.BlockAvailability, blocks []map[string]any) error {
	if b == nil || b.blocks == nil {
		return ErrEmbeddedBlocksBridgeUnwired
	}
	if availability.Empty() || len(blocks) == 0 {
		return nil
	}
	issues := []content.EmbeddedBlockValidationIssue{}
	for idx, block := range blocks {
		if block == nil {
			continue
		}
		blockType := strings.TrimSpace(embeddedBlockType(block))
		if blockType == "" {
			continue
		}
		if availability.Allows(blockType) {
			continue
		}
		message := "embedded block type is not permitted"
		if trimmed := strings.TrimSpace(contentType); trimmed != "" {
			message = fmt.Sprintf("embedded block type is not permitted for content type %s", trimmed)
		}
		issues = append(issues, content.EmbeddedBlockValidationIssue{
			Index:   idx,
			Type:    blockType,
			Field:   content.EmbeddedBlockTypeKey,
			Message: message,
		})
	}
	if len(issues) == 0 {
		return nil
	}
	return &content.EmbeddedBlockValidationError{
		Mode:   content.EmbeddedBlockValidationStrict,
		Issues: issues,
	}
}

// InstancesFromEmbeddedContent builds in-memory block instances from embedded payloads.
func (b *EmbeddedBlockBridge) InstancesFromEmbeddedContent(ctx context.Context, contentID uuid.UUID, translations []*content.ContentTranslation) ([]*Instance, error) {
	if b == nil {
		return nil, nil
	}
	embedded := buildEmbeddedIndex(translations, b.defaultRegion)
	if len(embedded) == 0 {
		return nil, nil
	}
	definitions, err := b.definitionIndex(ctx)
	if err != nil {
		return nil, err
	}
	baseLocaleID := pickBaseEmbeddedLocale(translations, embedded, b.defaultLocale)
	baseGroups := embedded[baseLocaleID]
	if len(baseGroups) == 0 {
		return nil, nil
	}

	now := b.clock()
	var instances []*Instance
	for region, entries := range baseGroups {
		for idx, entry := range entries {
			def := definitions[strings.ToLower(entry.kind)]
			instanceID := entry.meta.instanceID
			if instanceID == nil {
				id := deterministicInstanceID(contentID, region, entry.position, entry.kind)
				instanceID = &id
			}
			inst := &Instance{
				ID:            *instanceID,
				DefinitionID:  uuid.Nil,
				Region:        region,
				Position:      entry.position,
				Configuration: cloneMap(entry.meta.configuration),
				CreatedAt:     now,
				UpdatedAt:     now,
				Translations:  []*Translation{},
				Definition:    def,
			}
			if def != nil {
				inst.DefinitionID = def.ID
			} else if strings.TrimSpace(entry.kind) != "" {
				inst.Definition = &Definition{Name: strings.TrimSpace(entry.kind)}
			}

			for localeID, regionGroups := range embedded {
				entries := regionGroups[region]
				if len(entries) <= idx {
					continue
				}
				localeEntry := entries[idx]
				if !strings.EqualFold(localeEntry.kind, entry.kind) {
					continue
				}
				fields := embeddedBlockFields(localeEntry.block)
				trID := deterministicTranslationID(*instanceID, localeID)
				tr := &Translation{
					ID:                trID,
					BlockInstanceID:   *instanceID,
					LocaleID:          localeID,
					Content:           fields,
					AttributeOverride: cloneMap(localeEntry.meta.attributeOverride),
					MediaBindings:     media.CloneBindingSet(localeEntry.meta.mediaBindings),
					CreatedAt:         now,
					UpdatedAt:         now,
				}
				inst.Translations = append(inst.Translations, tr)
			}
			instances = append(instances, inst)
		}
	}
	return instances, nil
}

// BackfillFromLegacy writes embedded blocks into stored content translations.
func (b *EmbeddedBlockBridge) BackfillFromLegacy(ctx context.Context, opts BackfillOptions) (BackfillReport, error) {
	if b == nil || b.blocks == nil || b.pageResolver == nil || b.contentRepo == nil {
		return BackfillReport{}, ErrEmbeddedBlocksBridgeUnwired
	}
	records, err := b.contentRepo.List(ctx)
	if err != nil {
		return BackfillReport{}, err
	}
	filtered := filterContent(records, opts.ContentIDs)
	report := BackfillReport{}
	for _, record := range filtered {
		if record == nil {
			continue
		}
		translations, err := b.loadTranslations(ctx, record)
		if err != nil {
			report.Errors = append(report.Errors, err)
			continue
		}
		if len(translations) == 0 {
			continue
		}
		pageIDs, err := b.pageResolver.PageIDsForContent(ctx, record.ID)
		if err != nil {
			report.Errors = append(report.Errors, err)
			continue
		}
		if len(pageIDs) == 0 {
			continue
		}
		embeddedByLocale, err := b.embeddedBlocksFromLegacy(ctx, pageIDs[0])
		if err != nil {
			report.Errors = append(report.Errors, err)
			continue
		}
		updated := false
		for _, tr := range translations {
			if tr == nil {
				continue
			}
			if _, ok := content.ExtractEmbeddedBlocks(tr.Content); ok && !opts.Force {
				continue
			}
			blocks := embeddedByLocale[tr.LocaleID]
			if len(blocks) == 0 {
				continue
			}
			tr.Content = content.MergeEmbeddedBlocks(tr.Content, blocks)
			updated = true
		}
		if !updated {
			continue
		}
		if opts.DryRun {
			report.ContentCount++
			continue
		}
		if err := b.contentRepo.ReplaceTranslations(ctx, record.ID, translations); err != nil {
			report.Errors = append(report.Errors, err)
			continue
		}
		report.ContentCount++
	}
	return report, nil
}

// ListConflicts returns embedded-vs-legacy conflict reports.
func (b *EmbeddedBlockBridge) ListConflicts(ctx context.Context, opts ConflictReportOptions) ([]content.EmbeddedBlockConflict, error) {
	if b == nil || b.blocks == nil || b.pageResolver == nil || b.contentRepo == nil {
		return nil, ErrEmbeddedBlocksBridgeUnwired
	}
	records, err := b.contentRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	filtered := filterContent(records, opts.ContentIDs)
	conflicts := []content.EmbeddedBlockConflict{}
	for _, record := range filtered {
		if record == nil {
			continue
		}
		translations, err := b.loadTranslations(ctx, record)
		if err != nil {
			return nil, err
		}
		if len(translations) == 0 {
			continue
		}
		pageIDs, err := b.pageResolver.PageIDsForContent(ctx, record.ID)
		if err != nil {
			return nil, err
		}
		for _, pageID := range pageIDs {
			pageConflicts, err := b.comparePageBlocks(ctx, record.ID, pageID, translations)
			if err != nil {
				return nil, err
			}
			conflicts = append(conflicts, pageConflicts...)
			if opts.Limit > 0 && len(conflicts) >= opts.Limit {
				return conflicts[:opts.Limit], nil
			}
		}
	}
	return conflicts, nil
}

// BackfillOptions controls embedded block backfill behaviour.
type BackfillOptions struct {
	ContentIDs []uuid.UUID
	Force      bool
	DryRun     bool
}

// BackfillReport captures backfill results.
type BackfillReport struct {
	ContentCount int
	Errors       []error
}

// ConflictReportOptions scopes conflict queries.
type ConflictReportOptions struct {
	ContentIDs []uuid.UUID
	Limit      int
}

type embeddedEntry struct {
	block    map[string]any
	meta     embeddedMeta
	region   string
	position int
	index    int
	kind     string
	schema   string
}

type embeddedMeta struct {
	instanceID        *uuid.UUID
	definitionID      *uuid.UUID
	region            string
	position          *int
	configuration     map[string]any
	attributeOverride map[string]any
	mediaBindings     media.BindingSet
}

func (b *EmbeddedBlockBridge) collectEmbeddedBlocks(translations []content.ContentTranslationInput) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, tr := range translations {
		locale := content.NormalizeLocale(tr.Locale)
		if locale == "" {
			continue
		}
		blocks := tr.Blocks
		if len(blocks) == 0 {
			if extracted, ok := content.ExtractEmbeddedBlocks(tr.Content); ok {
				blocks = extracted
			}
		}
		if len(blocks) == 0 {
			continue
		}
		out[locale] = blocks
	}
	return out
}

func pickBaseLocale(blocksByLocale map[string][]map[string]any, preferred string) (string, []map[string]any) {
	if len(blocksByLocale) == 0 {
		return "", nil
	}
	if preferred != "" {
		if blocks, ok := blocksByLocale[content.NormalizeLocale(preferred)]; ok {
			return preferred, blocks
		}
	}
	for locale, blocks := range blocksByLocale {
		return locale, blocks
	}
	return "", nil
}

func (b *EmbeddedBlockBridge) syncPageBlocks(ctx context.Context, pageID uuid.UUID, baseLocale string, blocksByLocale map[string][]map[string]any, actor uuid.UUID) error {
	definitions, err := b.definitionIndex(ctx)
	if err != nil {
		return err
	}
	legacy, err := b.blocks.ListPageInstances(ctx, pageID)
	if err != nil {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return err
		}
		legacy = nil
	}

	legacyByRegion := groupLegacyByRegion(legacy)
	embeddedByLocale := make(map[string]map[string][]embeddedEntry, len(blocksByLocale))
	for locale, blocks := range blocksByLocale {
		embeddedByLocale[locale] = groupEmbeddedBlocks(blocks, b.defaultRegion)
	}
	baseGroups := embeddedByLocale[content.NormalizeLocale(baseLocale)]
	for region, entries := range baseGroups {
		legacyInstances := legacyByRegion[region]
		if err := b.syncRegion(ctx, pageID, region, entries, embeddedByLocale, legacyInstances, definitions, actor); err != nil {
			return err
		}
	}
	return nil
}

func (b *EmbeddedBlockBridge) syncRegion(
	ctx context.Context,
	pageID uuid.UUID,
	region string,
	baseEntries []embeddedEntry,
	entriesByLocale map[string]map[string][]embeddedEntry,
	legacyInstances []*Instance,
	definitions map[string]*Definition,
	actor uuid.UUID,
) error {
	sort.SliceStable(legacyInstances, func(i, j int) bool {
		return legacyInstances[i].Position < legacyInstances[j].Position
	})
	legacyByID := map[uuid.UUID]*Instance{}
	for _, inst := range legacyInstances {
		if inst == nil {
			continue
		}
		legacyByID[inst.ID] = inst
	}
	used := map[uuid.UUID]struct{}{}
	nextIndex := 0

	for idx, entry := range baseEntries {
		def, ok := definitions[strings.ToLower(entry.kind)]
		if !ok || def == nil {
			return fmt.Errorf("%w: %s", ErrEmbeddedBlockDefinitionMissing, entry.kind)
		}
		instance := resolveLegacyInstance(entry, legacyByID, legacyInstances, used, &nextIndex)
		if instance != nil && instance.DefinitionID != def.ID {
			if err := b.blocks.DeleteInstance(ctx, DeleteInstanceRequest{
				ID:         instance.ID,
				DeletedBy:  actor,
				HardDelete: true,
			}); err != nil {
				return err
			}
			instance = nil
		}
		if instance == nil {
			newInstance, err := b.blocks.CreateInstance(ctx, CreateInstanceInput{
				DefinitionID:  def.ID,
				PageID:        &pageID,
				Region:        region,
				Position:      entry.position,
				Configuration: maps.Clone(entry.meta.configuration),
				CreatedBy:     actor,
				UpdatedBy:     actor,
			})
			if err != nil {
				return err
			}
			instance = newInstance
		} else {
			updateReq := UpdateInstanceInput{
				InstanceID: instance.ID,
				UpdatedBy:  pickActor(actor, instance.UpdatedBy, instance.CreatedBy),
			}
			needsUpdate := false
			if instance.Region != region {
				updateReq.Region = &region
				needsUpdate = true
			}
			if instance.Position != entry.position {
				pos := entry.position
				updateReq.Position = &pos
				needsUpdate = true
			}
			if entry.meta.configuration != nil && !deepEqual(instance.Configuration, entry.meta.configuration) {
				updateReq.Configuration = maps.Clone(entry.meta.configuration)
				needsUpdate = true
			}
			if needsUpdate {
				updated, err := b.blocks.UpdateInstance(ctx, updateReq)
				if err != nil {
					return err
				}
				if updated != nil {
					instance = updated
				}
			}
		}

		instance.Region = region
		instance.Position = entry.position
		if entry.meta.configuration != nil {
			instance.Configuration = maps.Clone(entry.meta.configuration)
		}

		used[instance.ID] = struct{}{}
		if err := b.syncTranslations(ctx, instance, idx, entriesByLocale, actor, def.Schema); err != nil {
			return err
		}
	}

	for _, inst := range legacyInstances {
		if inst == nil {
			continue
		}
		if _, ok := used[inst.ID]; ok {
			continue
		}
		if err := b.blocks.DeleteInstance(ctx, DeleteInstanceRequest{
			ID:         inst.ID,
			DeletedBy:  actor,
			HardDelete: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (b *EmbeddedBlockBridge) syncTranslations(
	ctx context.Context,
	instance *Instance,
	index int,
	entriesByLocale map[string]map[string][]embeddedEntry,
	actor uuid.UUID,
	schema map[string]any,
) error {
	if instance == nil {
		return nil
	}
	if b.locales == nil {
		return ErrEmbeddedBlocksBridgeUnwired
	}
	translationIndex := make(map[uuid.UUID]*Translation)
	for _, tr := range instance.Translations {
		if tr == nil {
			continue
		}
		translationIndex[tr.LocaleID] = tr
	}
	localeCache := map[string]uuid.UUID{}
	for locale, regionGroups := range entriesByLocale {
		entries := regionGroups[instance.Region]
		if len(entries) <= index {
			continue
		}
		entry := entries[index]
		fields := embeddedBlockFields(entry.block)
		if shouldIncludeEmbeddedType(schema) {
			if blockType := strings.TrimSpace(embeddedBlockType(entry.block)); blockType != "" {
				fields[content.EmbeddedBlockTypeKey] = blockType
			}
		}
		meta := entry.meta
		localeID, ok := localeCache[locale]
		if !ok {
			loc, err := b.locales.GetByCode(ctx, locale)
			if err != nil {
				continue
			}
			localeID = loc.ID
			localeCache[locale] = localeID
		}
		if existing := translationIndex[localeID]; existing != nil {
			_, err := b.blocks.UpdateTranslation(ctx, UpdateTranslationInput{
				BlockInstanceID:    instance.ID,
				LocaleID:           localeID,
				Content:            fields,
				AttributeOverrides: meta.attributeOverride,
				MediaBindings:      meta.mediaBindings,
				UpdatedBy:          pickActor(actor, instance.UpdatedBy, instance.CreatedBy),
			})
			if err != nil {
				return err
			}
			continue
		}
		if _, err := b.blocks.AddTranslation(ctx, AddTranslationInput{
			BlockInstanceID:    instance.ID,
			LocaleID:           localeID,
			Content:            fields,
			AttributeOverrides: meta.attributeOverride,
			MediaBindings:      meta.mediaBindings,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (b *EmbeddedBlockBridge) definitionIndex(ctx context.Context) (map[string]*Definition, error) {
	defs, err := b.blocks.ListDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	index := make(map[string]*Definition, len(defs))
	for _, def := range defs {
		if def == nil {
			continue
		}
		index[strings.ToLower(def.Name)] = def
	}
	return index, nil
}

func groupLegacyByRegion(instances []*Instance) map[string][]*Instance {
	out := map[string][]*Instance{}
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		key := strings.TrimSpace(inst.Region)
		out[key] = append(out[key], inst)
	}
	return out
}

func groupEmbeddedBlocks(blocks []map[string]any, defaultRegion string) map[string][]embeddedEntry {
	out := map[string][]embeddedEntry{}
	for idx, block := range blocks {
		entry := embeddedEntry{
			block:  block,
			meta:   parseEmbeddedMeta(block),
			index:  idx,
			kind:   strings.TrimSpace(embeddedBlockType(block)),
			schema: strings.TrimSpace(embeddedBlockSchema(block)),
		}
		if strings.TrimSpace(entry.kind) == "" {
			continue
		}
		entry.region = defaultRegion
		entry.position = idx
		if entry.meta.region != "" {
			entry.region = entry.meta.region
		}
		if entry.meta.position != nil {
			entry.position = *entry.meta.position
		}
		out[entry.region] = append(out[entry.region], entry)
	}
	for region := range out {
		sort.SliceStable(out[region], func(i, j int) bool {
			if out[region][i].position == out[region][j].position {
				return out[region][i].index < out[region][j].index
			}
			return out[region][i].position < out[region][j].position
		})
	}
	return out
}

func resolveLegacyInstance(entry embeddedEntry, legacyByID map[uuid.UUID]*Instance, legacy []*Instance, used map[uuid.UUID]struct{}, next *int) *Instance {
	if entry.meta.instanceID != nil {
		if inst, ok := legacyByID[*entry.meta.instanceID]; ok {
			if _, usedAlready := used[inst.ID]; !usedAlready {
				return inst
			}
		}
	}
	for *next < len(legacy) {
		inst := legacy[*next]
		*next++
		if inst == nil {
			continue
		}
		if _, usedAlready := used[inst.ID]; usedAlready {
			continue
		}
		return inst
	}
	return nil
}

func embeddedBlockType(block map[string]any) string {
	if block == nil {
		return ""
	}
	if raw, ok := block[content.EmbeddedBlockTypeKey]; ok {
		if str, ok := raw.(string); ok {
			return str
		}
	}
	return ""
}

func embeddedBlockSchema(block map[string]any) string {
	if block == nil {
		return ""
	}
	if raw, ok := block[content.EmbeddedBlockSchemaKey]; ok {
		if str, ok := raw.(string); ok {
			return str
		}
	}
	return ""
}

func embeddedBlockFields(block map[string]any) map[string]any {
	if block == nil {
		return map[string]any{}
	}
	fields := cloneMap(block)
	delete(fields, content.EmbeddedBlockTypeKey)
	delete(fields, content.EmbeddedBlockSchemaKey)
	delete(fields, content.EmbeddedBlockMetaKey)
	return fields
}

func shouldIncludeEmbeddedType(schema map[string]any) bool {
	normalized := validation.NormalizeSchema(schema)
	if normalized == nil {
		return false
	}
	if props, ok := normalized["properties"].(map[string]any); ok {
		if _, ok := props[content.EmbeddedBlockTypeKey]; ok {
			return true
		}
	}
	if required, ok := normalized["required"]; ok {
		switch typed := required.(type) {
		case []string:
			for _, entry := range typed {
				if entry == content.EmbeddedBlockTypeKey {
					return true
				}
			}
		case []any:
			for _, entry := range typed {
				if value, ok := entry.(string); ok && value == content.EmbeddedBlockTypeKey {
					return true
				}
			}
		}
	}
	return false
}

func parseEmbeddedMeta(block map[string]any) embeddedMeta {
	meta := embeddedMeta{}
	if block == nil {
		return meta
	}
	raw, ok := block[content.EmbeddedBlockMetaKey]
	if !ok {
		return meta
	}
	typed, ok := raw.(map[string]any)
	if !ok {
		return meta
	}
	if value, ok := typed["instance_id"].(string); ok && value != "" {
		if parsed, err := uuid.Parse(value); err == nil {
			meta.instanceID = &parsed
		}
	}
	if value, ok := typed["definition_id"].(string); ok && value != "" {
		if parsed, err := uuid.Parse(value); err == nil {
			meta.definitionID = &parsed
		}
	}
	if value, ok := typed["region"].(string); ok {
		meta.region = strings.TrimSpace(value)
	}
	if value, ok := typed["position"].(float64); ok {
		pos := int(value)
		meta.position = &pos
	} else if value, ok := typed["position"].(int); ok {
		pos := value
		meta.position = &pos
	}
	if cfg, ok := typed["configuration"].(map[string]any); ok {
		meta.configuration = cloneMap(cfg)
	}
	if overrides, ok := typed["attribute_overrides"].(map[string]any); ok {
		meta.attributeOverride = cloneMap(overrides)
	}
	if bindings := parseBindings(typed["media_bindings"]); len(bindings) > 0 {
		meta.mediaBindings = bindings
	}
	return meta
}

func sanitizeEmbeddedBlockPayload(block map[string]any) map[string]any {
	if block == nil {
		return map[string]any{}
	}
	clean := cloneMap(block)
	delete(clean, content.EmbeddedBlockSchemaKey)
	delete(clean, content.EmbeddedBlockMetaKey)
	return clean
}

func ensureEmbeddedTypeInSchema(schema map[string]any, blockType string) map[string]any {
	if schema == nil {
		return nil
	}
	clone := cloneMap(schema)
	props := map[string]any{}
	if existing, ok := clone["properties"].(map[string]any); ok {
		props = cloneMap(existing)
	}
	if _, ok := props[content.EmbeddedBlockTypeKey]; !ok {
		props[content.EmbeddedBlockTypeKey] = map[string]any{
			"const": strings.TrimSpace(blockType),
		}
	}
	clone["properties"] = props
	ensureRequiredProperty(clone, content.EmbeddedBlockTypeKey)
	return clone
}

func ensureRequiredProperty(schema map[string]any, name string) {
	if schema == nil || strings.TrimSpace(name) == "" {
		return
	}
	if raw, ok := schema["required"]; ok {
		switch typed := raw.(type) {
		case []string:
			for _, entry := range typed {
				if entry == name {
					return
				}
			}
			schema["required"] = append(typed, name)
			return
		case []any:
			for _, entry := range typed {
				if value, ok := entry.(string); ok && value == name {
					return
				}
			}
			schema["required"] = append(typed, name)
			return
		}
	}
	schema["required"] = []string{name}
}

func parseEmbeddedSchemaVersion(block map[string]any) (cmsschema.Version, bool, error) {
	raw := strings.TrimSpace(embeddedBlockSchema(block))
	if raw == "" {
		return cmsschema.Version{}, false, nil
	}
	version, err := cmsschema.ParseVersion(raw)
	if err != nil {
		return cmsschema.Version{}, false, ErrDefinitionSchemaVersionInvalid
	}
	return version, true, nil
}

func (b *EmbeddedBlockBridge) definitionLookups(ctx context.Context) (map[string]*Definition, map[uuid.UUID]*Definition, error) {
	defs, err := b.blocks.ListDefinitions(ctx)
	if err != nil {
		return nil, nil, err
	}
	byName := make(map[string]*Definition, len(defs))
	byID := make(map[uuid.UUID]*Definition, len(defs))
	for _, def := range defs {
		if def == nil {
			continue
		}
		byName[strings.ToLower(strings.TrimSpace(def.Name))] = def
		byID[def.ID] = def
	}
	return byName, byID, nil
}

func (b *EmbeddedBlockBridge) schemaForEmbeddedBlock(ctx context.Context, def *Definition, block map[string]any) (map[string]any, string, error) {
	if def == nil {
		return nil, "", ErrEmbeddedBlockDefinitionMissing
	}
	target, err := resolveDefinitionSchemaVersion(def.Schema, def.Name)
	if err != nil {
		return nil, "", err
	}
	current, ok, err := parseEmbeddedSchemaVersion(block)
	if err != nil {
		return nil, "", err
	}
	if ok {
		if strings.TrimSpace(current.Slug) != "" && !strings.EqualFold(current.Slug, target.Slug) {
			return nil, current.String(), ErrDefinitionSchemaVersionInvalid
		}
		if current.String() == target.String() {
			return def.Schema, current.String(), nil
		}
		versionSchema, err := b.definitionSchemaVersion(ctx, def, current)
		if err != nil {
			return nil, current.String(), err
		}
		return versionSchema, current.String(), nil
	}
	return def.Schema, target.String(), nil
}

func (b *EmbeddedBlockBridge) definitionSchemaVersion(ctx context.Context, def *Definition, version cmsschema.Version) (map[string]any, error) {
	if def == nil {
		return nil, ErrEmbeddedBlockDefinitionMissing
	}
	if strings.TrimSpace(version.String()) == "" {
		return def.Schema, nil
	}
	record, err := b.blocks.GetDefinitionVersion(ctx, def.ID, version.String())
	if err != nil {
		return nil, err
	}
	if record == nil || record.Schema == nil {
		return nil, ErrEmbeddedBlockDefinitionMissing
	}
	return record.Schema, nil
}

func (b *EmbeddedBlockBridge) migrateEmbeddedBlock(def *Definition, block map[string]any) (map[string]any, error) {
	if def == nil {
		return nil, ErrEmbeddedBlockDefinitionMissing
	}
	target, err := resolveDefinitionSchemaVersion(def.Schema, def.Name)
	if err != nil {
		return nil, err
	}
	current, hasCurrent, err := parseEmbeddedSchemaVersion(block)
	if err != nil {
		return nil, err
	}
	if hasCurrent {
		if strings.TrimSpace(current.Slug) != "" && !strings.EqualFold(current.Slug, target.Slug) {
			return nil, ErrDefinitionSchemaVersionInvalid
		}
	}
	payload := sanitizeEmbeddedBlockPayload(block)
	migrated := payload
	if hasCurrent && current.String() != target.String() {
		migrator := b.schemaMigrator()
		if migrator == nil {
			return nil, ErrBlockSchemaMigrationRequired
		}
		migratedPayload, err := migrator.Migrate(target.Slug, current.String(), target.String(), payload)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBlockSchemaMigrationRequired, err)
		}
		migrated = migratedPayload
	}
	blockType := strings.TrimSpace(embeddedBlockType(block))
	if blockType != "" {
		migrated[content.EmbeddedBlockTypeKey] = blockType
	}
	if meta, ok := block[content.EmbeddedBlockMetaKey]; ok {
		migrated[content.EmbeddedBlockMetaKey] = meta
	}
	return applySchemaVersion(migrated, target), nil
}

func (b *EmbeddedBlockBridge) schemaMigrator() *Migrator {
	if b == nil {
		return nil
	}
	if svc, ok := b.blocks.(*service); ok && svc != nil {
		return svc.schemaMigrator
	}
	return nil
}

func (b *EmbeddedBlockBridge) validateImmutableType(ctx context.Context, locale string, index int, blockType string, block map[string]any, defsByID map[uuid.UUID]*Definition) error {
	issues := []content.EmbeddedBlockValidationIssue{}
	if err := b.appendImmutableTypeIssue(ctx, locale, index, blockType, block, defsByID, &issues); err != nil {
		return err
	}
	if len(issues) == 0 {
		return nil
	}
	return &content.EmbeddedBlockValidationError{
		Mode:   content.EmbeddedBlockValidationStrict,
		Issues: issues,
	}
}

func (b *EmbeddedBlockBridge) appendImmutableTypeIssue(ctx context.Context, locale string, index int, blockType string, block map[string]any, defsByID map[uuid.UUID]*Definition, issues *[]content.EmbeddedBlockValidationIssue) error {
	meta := parseEmbeddedMeta(block)
	expected, err := b.expectedTypeForMeta(ctx, meta, defsByID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(expected) == "" {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(expected), strings.TrimSpace(blockType)) {
		return nil
	}
	*issues = append(*issues, content.EmbeddedBlockValidationIssue{
		Locale:  locale,
		Index:   index,
		Type:    blockType,
		Message: fmt.Sprintf("embedded block _type is immutable; expected %s", expected),
	})
	return nil
}

func (b *EmbeddedBlockBridge) expectedTypeForMeta(ctx context.Context, meta embeddedMeta, defsByID map[uuid.UUID]*Definition) (string, error) {
	if meta.definitionID != nil {
		def, err := b.definitionByID(ctx, *meta.definitionID, defsByID)
		if err != nil {
			return "", err
		}
		if def != nil {
			return def.Name, nil
		}
	}
	if meta.instanceID != nil {
		instance, err := b.instanceByID(ctx, *meta.instanceID)
		if err != nil {
			return "", err
		}
		if instance == nil {
			return "", nil
		}
		def, err := b.definitionByID(ctx, instance.DefinitionID, defsByID)
		if err != nil {
			return "", err
		}
		if def != nil {
			return def.Name, nil
		}
	}
	return "", nil
}

func (b *EmbeddedBlockBridge) definitionByID(ctx context.Context, id uuid.UUID, defsByID map[uuid.UUID]*Definition) (*Definition, error) {
	if def, ok := defsByID[id]; ok {
		return def, nil
	}
	return b.blocks.GetDefinition(ctx, id)
}

func (b *EmbeddedBlockBridge) instanceByID(ctx context.Context, id uuid.UUID) (*Instance, error) {
	svc, ok := b.blocks.(*service)
	if !ok || svc == nil || svc.instances == nil {
		return nil, ErrEmbeddedBlocksBridgeUnwired
	}
	return svc.instances.GetByID(ctx, id)
}

func parseBindings(value any) media.BindingSet {
	if value == nil {
		return nil
	}
	if bindings, ok := value.(media.BindingSet); ok {
		return media.CloneBindingSet(bindings)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var bindings media.BindingSet
	if err := json.Unmarshal(raw, &bindings); err != nil {
		return nil
	}
	return media.CloneBindingSet(bindings)
}

func (b *EmbeddedBlockBridge) embeddedBlocksFromLegacy(ctx context.Context, pageID uuid.UUID) (map[uuid.UUID][]map[string]any, error) {
	instances, err := b.blocks.ListPageInstances(ctx, pageID)
	if err != nil {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
		instances = nil
	}
	sort.SliceStable(instances, func(i, j int) bool {
		if instances[i] == nil || instances[j] == nil {
			return false
		}
		regionI := strings.TrimSpace(instances[i].Region)
		regionJ := strings.TrimSpace(instances[j].Region)
		if regionI == regionJ {
			return instances[i].Position < instances[j].Position
		}
		return regionI < regionJ
	})
	definitions, err := b.definitionIndex(ctx)
	if err != nil {
		return nil, err
	}
	byLocale := map[uuid.UUID][]map[string]any{}
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		defName := definitionName(definitions, inst.DefinitionID)
		if defName == "" && inst.Definition != nil {
			defName = inst.Definition.Name
		}
		for _, tr := range inst.Translations {
			if tr == nil {
				continue
			}
			block := buildEmbeddedBlock(defName, tr, inst)
			byLocale[tr.LocaleID] = append(byLocale[tr.LocaleID], block)
		}
	}
	return byLocale, nil
}

func buildEmbeddedBlock(defName string, tr *Translation, inst *Instance) map[string]any {
	block := map[string]any{
		content.EmbeddedBlockTypeKey: strings.TrimSpace(defName),
	}
	if tr != nil {
		for key, value := range tr.Content {
			block[key] = value
		}
	}
	meta := map[string]any{}
	if inst != nil {
		meta["instance_id"] = inst.ID.String()
		meta["definition_id"] = inst.DefinitionID.String()
		meta["region"] = inst.Region
		meta["position"] = inst.Position
		if len(inst.Configuration) > 0 {
			meta["configuration"] = cloneMap(inst.Configuration)
		}
	}
	if tr != nil {
		if len(tr.AttributeOverride) > 0 {
			meta["attribute_overrides"] = cloneMap(tr.AttributeOverride)
		}
		if len(tr.MediaBindings) > 0 {
			meta["media_bindings"] = media.CloneBindingSet(tr.MediaBindings)
		}
	}
	if len(meta) > 0 {
		block[content.EmbeddedBlockMetaKey] = meta
	}
	return block
}

func (b *EmbeddedBlockBridge) comparePageBlocks(
	ctx context.Context,
	contentID uuid.UUID,
	pageID uuid.UUID,
	translations []*content.ContentTranslation,
) ([]content.EmbeddedBlockConflict, error) {
	conflicts := []content.EmbeddedBlockConflict{}
	embedded := buildEmbeddedIndex(translations, b.defaultRegion)
	nameByID := map[uuid.UUID]string{}
	defs, err := b.definitionIndex(ctx)
	if err == nil {
		for _, def := range defs {
			if def == nil {
				continue
			}
			nameByID[def.ID] = def.Name
		}
	}
	legacy, err := b.blocks.ListPageInstances(ctx, pageID)
	if err != nil {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
		legacy = nil
	}
	legacyByRegion := groupLegacyByRegion(legacy)
	for localeID, regionGroups := range embedded {
		for region, entries := range regionGroups {
			legacyEntries := legacyByRegion[region]
			sort.SliceStable(legacyEntries, func(i, j int) bool { return legacyEntries[i].Position < legacyEntries[j].Position })
			for i := 0; i < max(len(entries), len(legacyEntries)); i++ {
				var embeddedEntry *embeddedEntry
				if i < len(entries) {
					entry := entries[i]
					embeddedEntry = &entry
				}
				var legacyInst *Instance
				if i < len(legacyEntries) {
					legacyInst = legacyEntries[i]
				}
				conflicts = append(conflicts, compareEmbeddedLegacy(contentID, pageID, localeID, region, i, embeddedEntry, legacyInst, nameByID)...)
			}
		}
	}
	return conflicts, nil
}

func compareEmbeddedLegacy(
	contentID uuid.UUID,
	pageID uuid.UUID,
	localeID uuid.UUID,
	region string,
	index int,
	embedded *embeddedEntry,
	legacy *Instance,
	nameByID map[uuid.UUID]string,
) []content.EmbeddedBlockConflict {
	conflicts := []content.EmbeddedBlockConflict{}
	if embedded == nil && legacy == nil {
		return conflicts
	}
	locale := localeID.String()
	if embedded == nil && legacy != nil {
		legacyType := legacyDefinitionName(legacy, nameByID)
		conflicts = append(conflicts, content.EmbeddedBlockConflict{
			ContentID:        contentID,
			PageID:           pageID,
			Locale:           locale,
			Region:           region,
			Index:            index,
			Issue:            content.ConflictEmbeddedMissing,
			LegacyType:       legacyType,
			LegacyInstanceID: legacy.ID,
		})
		return conflicts
	}
	if embedded != nil && legacy == nil {
		conflicts = append(conflicts, content.EmbeddedBlockConflict{
			ContentID:      contentID,
			PageID:         pageID,
			Locale:         locale,
			Region:         region,
			Index:          index,
			Issue:          content.ConflictLegacyMissing,
			EmbeddedType:   embedded.kind,
			EmbeddedSchema: embedded.schema,
		})
		return conflicts
	}
	if embedded != nil && legacy != nil {
		legacyType := legacyDefinitionName(legacy, nameByID)
		if legacyType != "" && !strings.EqualFold(embedded.kind, legacyType) {
			conflicts = append(conflicts, content.EmbeddedBlockConflict{
				ContentID:        contentID,
				PageID:           pageID,
				Locale:           locale,
				Region:           region,
				Index:            index,
				Issue:            content.ConflictTypeMismatch,
				EmbeddedType:     embedded.kind,
				LegacyType:       legacyType,
				LegacyInstanceID: legacy.ID,
			})
		}
		tr := findTranslation(legacy, localeID)
		if tr != nil {
			embeddedFields := embeddedBlockFields(embedded.block)
			legacyFields := cloneMap(tr.Content)
			delete(legacyFields, content.EmbeddedBlockSchemaKey)
			if !deepEqual(embeddedFields, legacyFields) {
				conflicts = append(conflicts, content.EmbeddedBlockConflict{
					ContentID:        contentID,
					PageID:           pageID,
					Locale:           locale,
					Region:           region,
					Index:            index,
					Issue:            content.ConflictContentMismatch,
					EmbeddedType:     embedded.kind,
					LegacyType:       legacyType,
					LegacyInstanceID: legacy.ID,
				})
			}
			if embedded.schema != "" {
				if legacySchema, _ := tr.Content[content.EmbeddedBlockSchemaKey].(string); legacySchema != "" && legacySchema != embedded.schema {
					conflicts = append(conflicts, content.EmbeddedBlockConflict{
						ContentID:        contentID,
						PageID:           pageID,
						Locale:           locale,
						Region:           region,
						Index:            index,
						Issue:            content.ConflictSchemaMismatch,
						EmbeddedSchema:   embedded.schema,
						LegacySchema:     legacySchema,
						LegacyInstanceID: legacy.ID,
					})
				}
			}
		}
		if embedded.meta.configuration != nil && !deepEqual(embedded.meta.configuration, legacy.Configuration) {
			conflicts = append(conflicts, content.EmbeddedBlockConflict{
				ContentID:        contentID,
				PageID:           pageID,
				Locale:           locale,
				Region:           region,
				Index:            index,
				Issue:            content.ConflictConfigMismatch,
				LegacyInstanceID: legacy.ID,
			})
		}
		if tr := findTranslation(legacy, localeID); tr != nil {
			if embedded.meta.attributeOverride != nil && !deepEqual(embedded.meta.attributeOverride, tr.AttributeOverride) {
				conflicts = append(conflicts, content.EmbeddedBlockConflict{
					ContentID:        contentID,
					PageID:           pageID,
					Locale:           locale,
					Region:           region,
					Index:            index,
					Issue:            content.ConflictAttrsMismatch,
					LegacyInstanceID: legacy.ID,
				})
			}
			if embedded.meta.mediaBindings != nil && !deepEqual(embedded.meta.mediaBindings, tr.MediaBindings) {
				conflicts = append(conflicts, content.EmbeddedBlockConflict{
					ContentID:        contentID,
					PageID:           pageID,
					Locale:           locale,
					Region:           region,
					Index:            index,
					Issue:            content.ConflictMediaMismatch,
					LegacyInstanceID: legacy.ID,
				})
			}
		}
	}
	return conflicts
}

func buildEmbeddedIndex(translations []*content.ContentTranslation, defaultRegion string) map[uuid.UUID]map[string][]embeddedEntry {
	index := map[uuid.UUID]map[string][]embeddedEntry{}
	for _, tr := range translations {
		if tr == nil {
			continue
		}
		blocks, ok := content.ExtractEmbeddedBlocks(tr.Content)
		if !ok {
			continue
		}
		index[tr.LocaleID] = groupEmbeddedBlocks(blocks, defaultRegion)
	}
	return index
}

func pickBaseEmbeddedLocale(translations []*content.ContentTranslation, embedded map[uuid.UUID]map[string][]embeddedEntry, preferred string) uuid.UUID {
	if len(embedded) == 0 {
		return uuid.Nil
	}
	if strings.TrimSpace(preferred) != "" {
		for _, tr := range translations {
			if tr == nil || tr.Locale == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(tr.Locale.Code), strings.TrimSpace(preferred)) {
				if _, ok := embedded[tr.LocaleID]; ok {
					return tr.LocaleID
				}
			}
		}
	}
	keys := make([]string, 0, len(embedded))
	keyMap := make(map[string]uuid.UUID, len(embedded))
	for localeID := range embedded {
		key := localeID.String()
		keys = append(keys, key)
		keyMap[key] = localeID
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return uuid.Nil
	}
	return keyMap[keys[0]]
}

func findTranslation(instance *Instance, localeID uuid.UUID) *Translation {
	if instance == nil {
		return nil
	}
	for _, tr := range instance.Translations {
		if tr == nil {
			continue
		}
		if tr.LocaleID == localeID {
			return tr
		}
	}
	return nil
}

func definitionName(definitions map[string]*Definition, id uuid.UUID) string {
	for _, def := range definitions {
		if def != nil && def.ID == id {
			return def.Name
		}
	}
	return ""
}

func legacyDefinitionName(instance *Instance, nameByID map[uuid.UUID]string) string {
	if instance == nil {
		return ""
	}
	if instance.Definition != nil && strings.TrimSpace(instance.Definition.Name) != "" {
		return instance.Definition.Name
	}
	if nameByID != nil {
		if name, ok := nameByID[instance.DefinitionID]; ok {
			return name
		}
	}
	return ""
}

func filterContent(records []*content.Content, filter []uuid.UUID) []*content.Content {
	if len(filter) == 0 {
		return records
	}
	allowed := make(map[uuid.UUID]struct{}, len(filter))
	for _, id := range filter {
		if id != uuid.Nil {
			allowed[id] = struct{}{}
		}
	}
	result := make([]*content.Content, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		if _, ok := allowed[record.ID]; ok {
			result = append(result, record)
		}
	}
	return result
}

func (b *EmbeddedBlockBridge) loadTranslations(ctx context.Context, record *content.Content) ([]*content.ContentTranslation, error) {
	if record == nil {
		return nil, nil
	}
	if len(record.Translations) > 0 {
		return record.Translations, nil
	}
	if b.translationReader == nil {
		return nil, nil
	}
	translations, err := b.translationReader.ListTranslations(ctx, record.ID)
	if err != nil && !errors.Is(err, content.ErrContentTranslationLookupUnsupported) {
		return nil, err
	}
	return translations, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func deepEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

func deterministicInstanceID(contentID uuid.UUID, region string, position int, kind string) uuid.UUID {
	key := fmt.Sprintf("go-cms:embedded_block:%s:%s:%d:%s", contentID.String(), strings.TrimSpace(region), position, strings.TrimSpace(kind))
	return identity.UUID(key)
}

func deterministicTranslationID(instanceID uuid.UUID, localeID uuid.UUID) uuid.UUID {
	key := fmt.Sprintf("go-cms:embedded_block_translation:%s:%s", instanceID.String(), localeID.String())
	return identity.UUID(key)
}
