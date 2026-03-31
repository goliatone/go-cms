package blocks

import (
	"context"
	"errors"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/goliatone/go-cms/content"
	"github.com/goliatone/go-cms/internal/adminreadutil"
	internalcontent "github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// AdminBlockPageResolver resolves page IDs for one content record.
type AdminBlockPageResolver func(ctx context.Context, contentID uuid.UUID, envKey string) ([]uuid.UUID, error)

// AdminBlockReadOption configures the admin block read service.
type AdminBlockReadOption func(*adminBlockReadService)

// WithAdminBlockReadLogger overrides the logger used by the admin block read service.
func WithAdminBlockReadLogger(logger interfaces.Logger) AdminBlockReadOption {
	return func(s *adminBlockReadService) {
		s.logger = logger
	}
}

// NewAdminBlockReadService constructs the admin block read service.
func NewAdminBlockReadService(blockSvc Service, contentTypes internalcontent.ContentTypeService, locales internalcontent.LocaleRepository, pageResolver AdminBlockPageResolver, opts ...AdminBlockReadOption) interfaces.AdminBlockReadService {
	service := &adminBlockReadService{
		blocks:       blockSvc,
		contentTypes: contentTypes,
		locales:      locales,
		pageResolver: pageResolver,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

type adminBlockReadService struct {
	blocks       Service
	contentTypes internalcontent.ContentTypeService
	locales      internalcontent.LocaleRepository
	pageResolver AdminBlockPageResolver
	logger       interfaces.Logger
}

// ListDefinitions returns admin-shaped block definitions.
func (s *adminBlockReadService) ListDefinitions(ctx context.Context, opts interfaces.AdminBlockDefinitionListOptions) ([]interfaces.AdminBlockDefinitionRecord, int, error) {
	if s == nil || s.blocks == nil {
		return nil, 0, errors.New("blocks: admin read service requires block service")
	}
	definitions, err := s.blocks.ListDefinitions(ctx, strings.TrimSpace(opts.EnvironmentKey))
	if err != nil {
		return nil, 0, err
	}
	allowedTypes, restricted := s.allowedBlockTypes(ctx, opts.Filters)
	filtered := make([]interfaces.AdminBlockDefinitionRecord, 0, len(definitions))
	for _, definition := range definitions {
		if definition == nil {
			continue
		}
		item := mapAdminBlockDefinition(definition, strings.TrimSpace(opts.EnvironmentKey))
		if restricted && !definitionMatchesAllowedTypes(item, allowedTypes) {
			continue
		}
		if !matchesAdminBlockDefinitionFilters(item, opts.Filters, opts.Search) {
			continue
		}
		filtered = append(filtered, item)
	}
	sortAdminBlockDefinitions(filtered, opts.SortBy, opts.SortDesc)
	total := len(filtered)
	if opts.Page > 0 && opts.PerPage > 0 {
		start := (opts.Page - 1) * opts.PerPage
		if start >= total {
			filtered = []interfaces.AdminBlockDefinitionRecord{}
		} else {
			end := min(start+opts.PerPage, total)
			filtered = filtered[start:end]
		}
	}
	return projectAdminBlockDefinitionRecords(filtered, opts.Fields), total, nil
}

// GetDefinition returns one admin-shaped block definition by uuid, slug, or type.
func (s *adminBlockReadService) GetDefinition(ctx context.Context, id string, opts interfaces.AdminBlockDefinitionGetOptions) (*interfaces.AdminBlockDefinitionRecord, error) {
	if s == nil || s.blocks == nil {
		return nil, errors.New("blocks: admin read service requires block service")
	}
	target := strings.TrimSpace(id)
	if target == "" {
		return nil, ErrDefinitionIDRequired
	}
	if parsed, err := uuid.Parse(target); err == nil && parsed != uuid.Nil {
		definition, err := s.blocks.GetDefinition(ctx, parsed)
		if err == nil && definition != nil {
			item := mapAdminBlockDefinition(definition, strings.TrimSpace(opts.EnvironmentKey))
			return &item, nil
		}
	}
	definitions, _, err := s.ListDefinitions(ctx, interfaces.AdminBlockDefinitionListOptions{
		EnvironmentKey: opts.EnvironmentKey,
	})
	if err != nil {
		return nil, err
	}
	for _, definition := range definitions {
		if strings.EqualFold(definition.Slug, target) || strings.EqualFold(definition.Type, target) || strings.EqualFold(definition.ID.String(), target) {
			copy := definition
			return &copy, nil
		}
	}
	return nil, &NotFoundError{Resource: "block_definition", Key: target}
}

// ListDefinitionVersions returns admin-shaped definition versions.
func (s *adminBlockReadService) ListDefinitionVersions(ctx context.Context, definitionID string) ([]interfaces.AdminBlockDefinitionVersionRecord, error) {
	if s == nil || s.blocks == nil {
		return nil, errors.New("blocks: admin read service requires block service")
	}
	definition, err := s.GetDefinition(ctx, definitionID, interfaces.AdminBlockDefinitionGetOptions{})
	if err != nil {
		return nil, err
	}
	versions, err := s.blocks.ListDefinitionVersions(ctx, definition.ID)
	if err != nil {
		return nil, err
	}
	out := make([]interfaces.AdminBlockDefinitionVersionRecord, 0, len(versions))
	for _, version := range versions {
		if version == nil {
			continue
		}
		item := interfaces.AdminBlockDefinitionVersionRecord{
			ID:              version.ID,
			DefinitionID:    version.DefinitionID,
			SchemaVersion:   strings.TrimSpace(version.SchemaVersion),
			Schema:          cloneAdminBlockMap(version.Schema),
			Defaults:        cloneAdminBlockMap(version.Defaults),
			MigrationStatus: ResolveDefinitionMigrationStatus(version.Schema, version.SchemaVersion),
			CreatedAt:       cloneAdminBlockTime(&version.CreatedAt),
			UpdatedAt:       cloneAdminBlockTime(&version.UpdatedAt),
		}
		out = append(out, item)
	}
	return out, nil
}

// ListContentBlocks returns admin-shaped block instances for one content/page record.
func (s *adminBlockReadService) ListContentBlocks(ctx context.Context, contentID string, opts interfaces.AdminBlockListOptions) ([]interfaces.AdminBlockRecord, error) {
	if s == nil || s.blocks == nil {
		return nil, errors.New("blocks: admin read service requires block service")
	}
	parsedContentID, err := uuid.Parse(strings.TrimSpace(contentID))
	if err != nil || parsedContentID == uuid.Nil {
		return nil, internalcontent.ErrContentIDRequired
	}
	pageIDs, err := s.resolvePageIDs(ctx, parsedContentID, strings.TrimSpace(opts.EnvironmentKey))
	if err != nil {
		return nil, err
	}
	if len(pageIDs) == 0 {
		return nil, nil
	}
	primaryLocaleID, primaryLocaleCode, err := s.resolveLocale(ctx, opts.Locale)
	if err != nil {
		return nil, err
	}
	fallbackLocaleID, fallbackLocaleCode, err := s.resolveLocale(ctx, opts.FallbackLocale)
	if err != nil {
		return nil, err
	}
	primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode = adminreadutil.DedupeLocalePreference(primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)

	out := make([]interfaces.AdminBlockRecord, 0)
	for _, pageID := range pageIDs {
		instances, err := s.blocks.ListPageInstances(ctx, pageID)
		if err != nil {
			return nil, err
		}
		for _, instance := range instances {
			if instance == nil {
				continue
			}
			definition, _ := s.blocks.GetDefinition(ctx, instance.DefinitionID)
			out = append(out, s.mapBlockRecord(ctx, instance, definition, parsedContentID, primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode))
		}
	}
	return out, nil
}

func (s *adminBlockReadService) resolvePageIDs(ctx context.Context, contentID uuid.UUID, envKey string) ([]uuid.UUID, error) {
	if s == nil || s.pageResolver == nil || contentID == uuid.Nil {
		return nil, nil
	}
	return s.pageResolver(ctx, contentID, envKey)
}

func (s *adminBlockReadService) resolveLocale(ctx context.Context, code string) (uuid.UUID, string, error) {
	normalized := adminreadutil.NormalizeLocale(code)
	if normalized == "" {
		return uuid.Nil, "", nil
	}
	if s == nil || s.locales == nil {
		return uuid.Nil, normalized, nil
	}
	locale, err := s.locales.GetByCode(ctx, normalized)
	if err != nil {
		var notFound *internalcontent.NotFoundError
		if errors.As(err, &notFound) {
			return uuid.Nil, normalized, errors.Join(internalcontent.ErrUnknownLocale, err)
		}
		return uuid.Nil, normalized, err
	}
	if locale == nil {
		return uuid.Nil, normalized, internalcontent.ErrUnknownLocale
	}
	return locale.ID, adminreadutil.NormalizeLocale(locale.Code), nil
}

func (s *adminBlockReadService) localeCodeByID(ctx context.Context, id uuid.UUID) string {
	if s == nil || s.locales == nil || id == uuid.Nil {
		return ""
	}
	locale, err := s.locales.GetByID(ctx, id)
	if err != nil || locale == nil {
		return ""
	}
	return adminreadutil.NormalizeLocale(locale.Code)
}

func (s *adminBlockReadService) mapBlockRecord(ctx context.Context, instance *Instance, definition *Definition, contentID uuid.UUID, primaryLocaleID uuid.UUID, primaryLocaleCode string, fallbackLocaleID uuid.UUID, fallbackLocaleCode string) interfaces.AdminBlockRecord {
	translation, resolvedLocale := s.resolveBlockTranslation(ctx, instance, primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)
	data := cloneAdminBlockMap(instance.Configuration)
	if translation != nil && len(translation.Content) > 0 {
		data = cloneAdminBlockMap(translation.Content)
	}
	blockType := blockDefinitionType(definition)
	blockSchemaKey := ""
	if definition != nil {
		blockSchemaKey = strings.TrimSpace(firstNonEmptyBlockValue(definition.SchemaVersion, definition.Slug))
	}
	return interfaces.AdminBlockRecord{
		ID:             instance.ID,
		DefinitionID:   instance.DefinitionID,
		ContentID:      contentID,
		Region:         instance.Region,
		Locale:         resolvedLocale,
		Status:         firstNonEmptyBlockValue(instanceStatus(instance), "draft"),
		Data:           data,
		Position:       instance.Position,
		BlockType:      blockType,
		BlockSchemaKey: blockSchemaKey,
	}
}

func (s *adminBlockReadService) resolveBlockTranslation(ctx context.Context, instance *Instance, primaryLocaleID uuid.UUID, primaryLocaleCode string, fallbackLocaleID uuid.UUID, fallbackLocaleCode string) (*Translation, string) {
	if instance == nil || len(instance.Translations) == 0 {
		return nil, ""
	}
	if primaryLocaleID != uuid.Nil {
		for _, translation := range instance.Translations {
			if translation != nil && translation.LocaleID == primaryLocaleID {
				return translation, s.localeCodeByID(ctx, translation.LocaleID)
			}
		}
	}
	if primaryLocaleCode != "" {
		for _, translation := range instance.Translations {
			if translation != nil && adminreadutil.NormalizeLocale(s.localeCodeByID(ctx, translation.LocaleID)) == adminreadutil.NormalizeLocale(primaryLocaleCode) {
				return translation, s.localeCodeByID(ctx, translation.LocaleID)
			}
		}
	}
	if fallbackLocaleID != uuid.Nil {
		for _, translation := range instance.Translations {
			if translation != nil && translation.LocaleID == fallbackLocaleID {
				return translation, s.localeCodeByID(ctx, translation.LocaleID)
			}
		}
	}
	if fallbackLocaleCode != "" {
		for _, translation := range instance.Translations {
			if translation != nil && adminreadutil.NormalizeLocale(s.localeCodeByID(ctx, translation.LocaleID)) == adminreadutil.NormalizeLocale(fallbackLocaleCode) {
				return translation, s.localeCodeByID(ctx, translation.LocaleID)
			}
		}
	}
	return nil, ""
}

func mapAdminBlockDefinition(definition *Definition, environmentKey string) interfaces.AdminBlockDefinitionRecord {
	if definition == nil {
		return interfaces.AdminBlockDefinitionRecord{}
	}
	schemaVersion := strings.TrimSpace(definition.SchemaVersion)
	if schemaVersion == "" {
		schemaVersion = schemaVersionFromSchema(definition.Schema)
	}
	return interfaces.AdminBlockDefinitionRecord{
		ID:              definition.ID,
		Name:            definition.Name,
		Slug:            firstNonEmptyBlockValue(definition.Slug, blockDefinitionType(definition)),
		Type:            blockDefinitionType(definition),
		Description:     cloneStringValue(definition.Description),
		Icon:            cloneStringValue(definition.Icon),
		Category:        firstNonEmptyBlockValue(derefAdminBlockString(definition.Category), "custom"),
		Status:          firstNonEmptyBlockValue(definition.Status, "draft"),
		Channel:         strings.TrimSpace(environmentKey),
		Schema:          cloneAdminBlockMap(definition.Schema),
		UISchema:        cloneAdminBlockMap(definition.UISchema),
		SchemaVersion:   schemaVersion,
		MigrationStatus: ResolveDefinitionMigrationStatus(definition.Schema, schemaVersion),
		CreatedAt:       cloneAdminBlockTime(&definition.CreatedAt),
		UpdatedAt:       cloneAdminBlockTime(&definition.UpdatedAt),
	}
}

func (s *adminBlockReadService) allowedBlockTypes(ctx context.Context, filters map[string]any) (map[string]struct{}, bool) {
	if len(filters) == 0 || s == nil || s.contentTypes == nil {
		return nil, false
	}
	target := strings.TrimSpace(firstNonEmptyBlockValue(stringFromAny(filters["content_type"]), stringFromAny(filters["content_type_slug"]), stringFromAny(filters["content_type_id"])))
	if target == "" {
		return nil, false
	}
	ct := s.resolveContentType(ctx, target)
	if ct == nil {
		return nil, false
	}
	types, ok := blockTypesFromContentType(ct)
	if !ok {
		return nil, false
	}
	allowed := map[string]struct{}{}
	for _, value := range types {
		if normalized := strings.ToLower(strings.TrimSpace(value)); normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}
	return allowed, true
}

func (s *adminBlockReadService) resolveContentType(ctx context.Context, key string) *internalcontent.ContentType {
	if s == nil || s.contentTypes == nil || strings.TrimSpace(key) == "" {
		return nil
	}
	if record, err := s.contentTypes.GetBySlug(ctx, key); err == nil && record != nil {
		return record
	}
	if parsed, err := uuid.Parse(strings.TrimSpace(key)); err == nil && parsed != uuid.Nil {
		if record, err := s.contentTypes.Get(ctx, parsed); err == nil && record != nil {
			return record
		}
	}
	records, err := s.contentTypes.List(ctx)
	if err != nil {
		return nil
	}
	needle := strings.ToLower(strings.TrimSpace(key))
	for _, record := range records {
		if record == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(record.Name), needle) || strings.EqualFold(strings.TrimSpace(record.Slug), needle) {
			return record
		}
	}
	return nil
}

func matchesAdminBlockDefinitionFilters(record interfaces.AdminBlockDefinitionRecord, filters map[string]any, search string) bool {
	if len(filters) > 0 {
		if !filterAdminBlockString(record.Category, filters["category"]) {
			return false
		}
		if !filterAdminBlockString(record.Status, filters["status"]) {
			return false
		}
		if !filterAdminBlockString(record.Channel, firstNonNilBlock(filters["channel"], filters["environment"], filters["content_channel"])) {
			return false
		}
	}
	term := strings.ToLower(strings.TrimSpace(search))
	if term == "" {
		return true
	}
	return strings.Contains(strings.ToLower(record.Name), term) ||
		strings.Contains(strings.ToLower(record.Slug), term) ||
		strings.Contains(strings.ToLower(record.Type), term)
}

func definitionMatchesAllowedTypes(record interfaces.AdminBlockDefinitionRecord, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range []string{record.Type, record.Slug, record.Name, record.ID.String()} {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		if normalized == "" {
			continue
		}
		if _, ok := allowed[normalized]; ok {
			return true
		}
	}
	return false
}

func sortAdminBlockDefinitions(records []interfaces.AdminBlockDefinitionRecord, sortBy string, desc bool) {
	key := strings.ToLower(strings.TrimSpace(sortBy))
	if key == "" || len(records) < 2 {
		return
	}
	sort.Slice(records, func(i, j int) bool {
		less := compareAdminBlockDefinition(records[i], records[j], key)
		if desc {
			return !less
		}
		return less
	})
}

func compareAdminBlockDefinition(left, right interfaces.AdminBlockDefinitionRecord, key string) bool {
	switch key {
	case "name":
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	case "slug":
		return strings.ToLower(left.Slug) < strings.ToLower(right.Slug)
	case "type":
		return strings.ToLower(left.Type) < strings.ToLower(right.Type)
	case "category":
		return strings.ToLower(left.Category) < strings.ToLower(right.Category)
	case "status":
		return strings.ToLower(left.Status) < strings.ToLower(right.Status)
	case "created_at":
		return adminBlockTimeValue(left.CreatedAt).Before(adminBlockTimeValue(right.CreatedAt))
	case "updated_at":
		return adminBlockTimeValue(left.UpdatedAt).Before(adminBlockTimeValue(right.UpdatedAt))
	default:
		return false
	}
}

func adminBlockTimeValue(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func projectAdminBlockDefinitionRecords(records []interfaces.AdminBlockDefinitionRecord, fields []string) []interfaces.AdminBlockDefinitionRecord {
	selected := normalizeAdminBlockDefinitionFields(fields)
	if len(selected) == 0 || len(records) == 0 {
		return records
	}
	out := make([]interfaces.AdminBlockDefinitionRecord, len(records))
	for idx := range records {
		out[idx] = projectAdminBlockDefinitionRecord(records[idx], selected)
	}
	return out
}

func normalizeAdminBlockDefinitionFields(fields []string) map[string]struct{} {
	if len(fields) == 0 {
		return nil
	}
	selected := map[string]struct{}{"id": {}}
	for _, raw := range fields {
		field := strings.ToLower(strings.TrimSpace(raw))
		if field != "" {
			selected[field] = struct{}{}
		}
	}
	return selected
}

func projectAdminBlockDefinitionRecord(record interfaces.AdminBlockDefinitionRecord, selected map[string]struct{}) interfaces.AdminBlockDefinitionRecord {
	if len(selected) == 0 {
		return record
	}
	var out interfaces.AdminBlockDefinitionRecord
	if _, ok := selected["id"]; ok {
		out.ID = record.ID
	}
	if _, ok := selected["name"]; ok {
		out.Name = record.Name
	}
	if _, ok := selected["slug"]; ok {
		out.Slug = record.Slug
	}
	if _, ok := selected["type"]; ok {
		out.Type = record.Type
	}
	if _, ok := selected["description"]; ok {
		out.Description = cloneStringValue(record.Description)
	}
	if _, ok := selected["icon"]; ok {
		out.Icon = cloneStringValue(record.Icon)
	}
	if _, ok := selected["category"]; ok {
		out.Category = record.Category
	}
	if _, ok := selected["status"]; ok {
		out.Status = record.Status
	}
	if _, ok := selected["channel"]; ok {
		out.Channel = record.Channel
	}
	if _, ok := selected["schema"]; ok {
		out.Schema = cloneAdminBlockMap(record.Schema)
	}
	if _, ok := selected["ui_schema"]; ok {
		out.UISchema = cloneAdminBlockMap(record.UISchema)
	}
	if _, ok := selected["schema_version"]; ok {
		out.SchemaVersion = record.SchemaVersion
	}
	if _, ok := selected["migration_status"]; ok {
		out.MigrationStatus = record.MigrationStatus
	}
	if _, ok := selected["created_at"]; ok {
		out.CreatedAt = cloneAdminBlockTime(record.CreatedAt)
	}
	if _, ok := selected["updated_at"]; ok {
		out.UpdatedAt = cloneAdminBlockTime(record.UpdatedAt)
	}
	return out
}

func blockDefinitionType(definition *Definition) string {
	if definition == nil {
		return ""
	}
	return firstNonEmptyBlockValue(definition.Slug, definition.Name, definition.ID.String())
}

func blockTypesFromContentType(contentType *internalcontent.ContentType) ([]string, bool) {
	if contentType == nil {
		return nil, false
	}
	if types, ok := blockTypesFromCapabilities(contentType.Capabilities); ok {
		return types, true
	}
	return blockTypesFromSchema(contentType.Schema)
}

func blockTypesFromCapabilities(capabilities map[string]any) ([]string, bool) {
	contracts := content.ParseContentTypeCapabilityContracts(capabilities)
	for _, key := range []string{"blocks", "block_types", "allowed_blocks", "allowedBlockTypes"} {
		if raw, ok := contracts.Normalized[key]; ok {
			if values := normalizeBlockTypeCandidates(raw); len(values) > 0 {
				return values, true
			}
		}
	}
	return nil, false
}

func blockTypesFromSchema(schema map[string]any) ([]string, bool) {
	if len(schema) == 0 {
		return nil, false
	}
	props, _ := schema["properties"].(map[string]any)
	if len(props) == 0 {
		return nil, false
	}
	blocksField, _ := props["blocks"].(map[string]any)
	if len(blocksField) == 0 {
		return nil, false
	}
	if values := normalizeBlockTypeCandidates(blocksField["x-block-types"]); len(values) > 0 {
		return values, true
	}
	items, _ := blocksField["items"].(map[string]any)
	if len(items) == 0 {
		return nil, false
	}
	if values := normalizeBlockTypeCandidates(items["x-block-types"]); len(values) > 0 {
		return values, true
	}
	for _, key := range []string{"oneOf", "anyOf"} {
		rawVariants, _ := items[key].([]any)
		if len(rawVariants) == 0 {
			continue
		}
		values := make([]string, 0, len(rawVariants))
		for _, raw := range rawVariants {
			variant, _ := raw.(map[string]any)
			if len(variant) == 0 {
				continue
			}
			if valuesFromVariant := normalizeBlockTypeCandidates(variant["x-block-type"]); len(valuesFromVariant) > 0 {
				values = append(values, valuesFromVariant...)
				continue
			}
			if props, ok := variant["properties"].(map[string]any); ok {
				if typeField, ok := props["_type"].(map[string]any); ok {
					values = append(values, normalizeBlockTypeCandidates(typeField["const"])...)
				}
			}
		}
		if len(values) > 0 {
			return values, true
		}
	}
	return nil, false
}

func normalizeBlockTypeCandidates(raw any) []string {
	switch typed := raw.(type) {
	case string:
		if trimmed := strings.TrimSpace(typed); trimmed != "" {
			return []string{trimmed}
		}
	case []string:
		out := make([]string, 0, len(typed))
		for _, value := range typed {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, value := range typed {
			if trimmed := strings.TrimSpace(stringFromAny(value)); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	case map[string]any:
		for _, key := range []string{"types", "allowed", "block_types", "blockTypes"} {
			if nested, ok := typed[key]; ok {
				return normalizeBlockTypeCandidates(nested)
			}
		}
	}
	return nil
}

func filterAdminBlockString(value string, filter any) bool {
	if filter == nil {
		return true
	}
	switch typed := filter.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return true
		}
		return strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(typed))
	case []string:
		if len(typed) == 0 {
			return true
		}
		for _, entry := range typed {
			if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(entry)) {
				return true
			}
		}
	case []any:
		if len(typed) == 0 {
			return true
		}
		for _, entry := range typed {
			if text := stringFromAny(entry); strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(text)) {
				return true
			}
		}
	}
	return false
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func cloneAdminBlockMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	maps.Copy(out, src)
	return out
}

func cloneAdminBlockTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func firstNonEmptyBlockValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func derefAdminBlockString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstNonNilBlock(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func instanceStatus(instance *Instance) string {
	if instance == nil {
		return ""
	}
	if instance.PublishedVersion != nil {
		return "published"
	}
	return "draft"
}
