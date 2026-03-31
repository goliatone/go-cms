package content

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
	sharedi18n "github.com/goliatone/go-i18n"
	"github.com/google/uuid"
)

type AdminContentReadOption func(*adminContentReadService)
type AdminContentWriteOption func(*adminContentWriteService)

func WithAdminContentReadLogger(logger interfaces.Logger) AdminContentReadOption {
	return func(s *adminContentReadService) {
		s.logger = logger
	}
}

func WithAdminContentWriteLogger(logger interfaces.Logger) AdminContentWriteOption {
	return func(s *adminContentWriteService) {
		s.logger = logger
	}
}

func NewAdminContentReadService(contentSvc Service, contentTypes ContentTypeService, locales LocaleRepository, _ any, opts ...AdminContentReadOption) interfaces.AdminContentReadService {
	service := &adminContentReadService{
		content:      contentSvc,
		contentTypes: contentTypes,
		locales:      locales,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func NewAdminContentWriteService(contentSvc Service, contentTypes ContentTypeService, locales LocaleRepository, opts ...AdminContentWriteOption) interfaces.AdminContentWriteService {
	service := &adminContentWriteService{
		content:      contentSvc,
		contentTypes: contentTypes,
		locales:      locales,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

type adminContentReadService struct {
	content      Service
	contentTypes ContentTypeService
	locales      LocaleRepository
	logger       interfaces.Logger
}

type adminContentWriteService struct {
	content      Service
	contentTypes ContentTypeService
	locales      LocaleRepository
	logger       interfaces.Logger
}

func (s *adminContentReadService) List(ctx context.Context, opts interfaces.AdminContentListOptions) ([]interfaces.AdminContentRecord, int, error) {
	if s == nil || s.content == nil {
		return nil, 0, errors.New("content: admin read service requires content service")
	}
	records, err := s.content.List(ctx, adminContentReadOptions(opts)...)
	if err != nil {
		return nil, 0, err
	}
	includeData, includeMetadata, includeBlocks := resolveAdminContentIncludes(true, opts.IncludeData, opts.IncludeMetadata, opts.IncludeBlocks, opts.DefaultIncludes)
	requested := sharedi18n.NormalizeLocale(opts.Locale)
	fallback := sharedi18n.NormalizeLocale(opts.FallbackLocale)

	filtered := make([]interfaces.AdminContentRecord, 0, len(records))
	for _, record := range records {
		item, err := s.buildRecord(ctx, record, requested, fallback, opts.AllowMissingTranslations, includeData, includeMetadata, includeBlocks)
		if err != nil {
			return nil, 0, err
		}
		if !matchesAdminContentFilters(item, opts.Filters, opts.Search) {
			continue
		}
		filtered = append(filtered, item)
	}

	sortAdminContentRecords(filtered, opts.SortBy, opts.SortDesc)
	total := len(filtered)
	filtered = paginateAdminContentRecords(filtered, opts.Page, opts.PerPage)
	return projectAdminContentRecords(filtered, opts.Fields), total, nil
}

func (s *adminContentReadService) Get(ctx context.Context, id string, opts interfaces.AdminContentGetOptions) (*interfaces.AdminContentRecord, error) {
	if s == nil || s.content == nil {
		return nil, errors.New("content: admin read service requires content service")
	}
	contentID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil || contentID == uuid.Nil {
		return nil, ErrContentIDRequired
	}
	record, err := s.content.Get(ctx, contentID, adminContentGetOptions(opts)...)
	if err != nil {
		return nil, err
	}
	includeData, includeMetadata, includeBlocks := resolveAdminContentIncludes(false, opts.IncludeData, opts.IncludeMetadata, opts.IncludeBlocks, opts.DefaultIncludes)
	item, err := s.buildRecord(ctx, record, sharedi18n.NormalizeLocale(opts.Locale), sharedi18n.NormalizeLocale(opts.FallbackLocale), opts.AllowMissingTranslations, includeData, includeMetadata, includeBlocks)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *adminContentWriteService) Create(ctx context.Context, req interfaces.AdminContentCreateRequest) (*interfaces.AdminContentRecord, error) {
	if s == nil || s.content == nil {
		return nil, errors.New("content: admin write service requires content service")
	}
	contentTypeID, err := s.resolveContentTypeID(ctx, req.ContentTypeID, req.ContentTypeSlug, req.ContentType, req.EnvironmentKey)
	if err != nil {
		return nil, err
	}
	created, err := s.content.Create(ctx, CreateContentRequest{
		ContentTypeID:            contentTypeID,
		Slug:                     strings.TrimSpace(req.Slug),
		Status:                   strings.TrimSpace(req.Status),
		EnvironmentKey:           strings.TrimSpace(req.EnvironmentKey),
		CreatedBy:                req.CreatedBy,
		UpdatedBy:                req.UpdatedBy,
		Metadata:                 adminContentMetadata(req.Navigation, req.EffectiveMenuLocations, req.Metadata),
		Translations:             []ContentTranslationInput{adminContentTranslationInput(req.Locale, req.Title, req.Data, req.Blocks, req.EmbeddedBlocks, req.SchemaVersion)},
		AllowMissingTranslations: req.AllowMissingTranslations,
	})
	if err != nil {
		return nil, err
	}
	return s.projectRecord(ctx, created, req.Locale, "", req.AllowMissingTranslations)
}

func (s *adminContentWriteService) Update(ctx context.Context, req interfaces.AdminContentUpdateRequest) (*interfaces.AdminContentRecord, error) {
	if s == nil || s.content == nil {
		return nil, errors.New("content: admin write service requires content service")
	}
	updated, err := s.content.Update(ctx, UpdateContentRequest{
		ID:                       req.ID,
		Status:                   strings.TrimSpace(req.Status),
		EnvironmentKey:           strings.TrimSpace(req.EnvironmentKey),
		UpdatedBy:                req.UpdatedBy,
		Metadata:                 adminContentMetadata(req.Navigation, req.EffectiveMenuLocations, req.Metadata),
		Translations:             []ContentTranslationInput{adminContentTranslationInput(req.Locale, req.Title, req.Data, req.Blocks, req.EmbeddedBlocks, req.SchemaVersion)},
		AllowMissingTranslations: req.AllowMissingTranslations,
	})
	if err != nil {
		return nil, err
	}
	return s.projectRecord(ctx, updated, req.Locale, "", req.AllowMissingTranslations)
}

func (s *adminContentWriteService) Delete(ctx context.Context, req interfaces.AdminContentDeleteRequest) error {
	if s == nil || s.content == nil {
		return errors.New("content: admin write service requires content service")
	}
	return s.content.Delete(ctx, DeleteContentRequest{
		ID:         req.ID,
		DeletedBy:  req.DeletedBy,
		HardDelete: req.HardDelete,
	})
}

func (s *adminContentWriteService) CreateTranslation(ctx context.Context, req interfaces.AdminContentCreateTranslationRequest) (*interfaces.AdminContentRecord, error) {
	if s == nil || s.content == nil {
		return nil, errors.New("content: admin write service requires content service")
	}
	creator, ok := s.content.(TranslationCreator)
	if !ok || creator == nil {
		return nil, ErrSourceNotFound
	}
	created, err := creator.CreateTranslation(ctx, CreateContentTranslationRequest{
		SourceID:       req.SourceID,
		SourceLocale:   sharedi18n.NormalizeLocale(req.SourceLocale),
		TargetLocale:   sharedi18n.NormalizeLocale(req.TargetLocale),
		EnvironmentKey: strings.TrimSpace(req.EnvironmentKey),
		ActorID:        req.ActorID,
		Status:         strings.TrimSpace(req.Status),
	})
	if err != nil {
		return nil, err
	}
	return s.projectRecord(ctx, created, req.TargetLocale, req.SourceLocale, true)
}

func (s *adminContentWriteService) projectRecord(ctx context.Context, record *Content, locale, fallback string, allowMissing bool) (*interfaces.AdminContentRecord, error) {
	readSvc := adminContentReadService{
		content:      s.content,
		contentTypes: s.contentTypes,
		locales:      s.locales,
	}
	item, err := readSvc.buildRecord(ctx, record, sharedi18n.NormalizeLocale(locale), sharedi18n.NormalizeLocale(fallback), allowMissing, true, true, true)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *adminContentWriteService) resolveContentTypeID(ctx context.Context, explicit uuid.UUID, slugValue, nameValue, env string) (uuid.UUID, error) {
	if explicit != uuid.Nil {
		return explicit, nil
	}
	if s == nil || s.contentTypes == nil {
		return uuid.Nil, ErrContentTypeRequired
	}
	slugValue = strings.TrimSpace(adminFirstNonEmptyString(slugValue, nameValue))
	if slugValue == "" {
		return uuid.Nil, ErrContentTypeRequired
	}
	record, err := s.contentTypes.GetBySlug(ctx, slugValue, strings.TrimSpace(env))
	if err != nil {
		return uuid.Nil, err
	}
	if record == nil || record.ID == uuid.Nil {
		return uuid.Nil, ErrContentTypeRequired
	}
	return record.ID, nil
}

func (s *adminContentReadService) buildRecord(ctx context.Context, record *Content, requested, fallback string, allowMissing, includeData, includeMetadata, includeBlocks bool) (interfaces.AdminContentRecord, error) {
	if record == nil {
		return interfaces.AdminContentRecord{}, &NotFoundError{Resource: "content", Key: ""}
	}
	contentType := s.resolveContentType(ctx, record)
	translation, resolvedLocale, availableLocales := s.resolveTranslation(ctx, record, requested, fallback)
	missingRequested := requested != "" && resolvedLocale != requested
	if translation == nil {
		missingRequested = requested != ""
	}
	if !allowMissing && missingRequested {
		return interfaces.AdminContentRecord{}, errors.Join(interfaces.ErrTranslationMissing, ErrContentTranslationNotFound)
	}

	navigation, _ := NormalizeNavigationOverrides(mapValue(record.Metadata, entryFieldNavigation))
	visibility := ResolveNavigationVisibility(contentType, record.Metadata)
	effectiveLocations := append([]string{}, visibility.EffectiveMenuLocations...)
	if len(effectiveLocations) == 0 {
		effectiveLocations = cloneStringSlice(anySliceToStrings(mapValue(record.Metadata, entryFieldEffectiveMenuLocations)))
	}

	item := interfaces.AdminContentRecord{
		ID:                     record.ID,
		Slug:                   record.Slug,
		Locale:                 resolvedLocale,
		RequestedLocale:        requested,
		ResolvedLocale:         resolvedLocale,
		AvailableLocales:       append([]string{}, availableLocales...),
		MissingRequestedLocale: missingRequested,
		Navigation:             navigation,
		EffectiveMenuLocations: effectiveLocations,
		Status:                 record.Status,
		CreatedAt:              adminCloneTimeValuePtr(record.CreatedAt),
		UpdatedAt:              adminCloneTimeValuePtr(record.UpdatedAt),
		PublishedAt:            adminCloneTimePtr(record.PublishedAt),
	}
	if contentType != nil {
		item.ContentType = strings.TrimSpace(adminFirstNonEmptyString(contentType.Slug, contentType.Name))
		item.ContentTypeSlug = DeriveContentTypeSlug(contentType)
		item.SchemaVersion = strings.TrimSpace(contentType.SchemaVersion)
	}
	if includeMetadata {
		item.Metadata = cloneAnyMap(record.Metadata)
	}
	if translation != nil {
		item.Title = translation.Title
		if translation.FamilyID != nil && *translation.FamilyID != uuid.Nil {
			value := *translation.FamilyID
			item.FamilyID = &value
		}
		payload := cloneAnyMap(translation.Content)
		if item.SchemaVersion == "" {
			item.SchemaVersion = strings.TrimSpace(adminToString(payload["_schema"]))
		}
		if includeData {
			item.Data = payload
		}
		if includeBlocks {
			if embedded, ok := ExtractEmbeddedBlocks(payload); ok {
				item.EmbeddedBlocks = cloneAnyMapSlice(embedded)
			} else {
				item.Blocks = cloneStringSlice(anySliceToStrings(payload["blocks"]))
			}
		}
	}
	return item, nil
}

func (s *adminContentReadService) resolveContentType(ctx context.Context, record *Content) *ContentType {
	if record == nil {
		return nil
	}
	if record.Type != nil {
		return record.Type
	}
	if s == nil || s.contentTypes == nil || record.ContentTypeID == uuid.Nil {
		return nil
	}
	contentType, err := s.contentTypes.Get(ctx, record.ContentTypeID)
	if err != nil {
		return nil
	}
	return contentType
}

func (s *adminContentReadService) resolveTranslation(ctx context.Context, record *Content, requested, fallback string) (*ContentTranslation, string, []string) {
	if record == nil {
		return nil, "", nil
	}
	translations := record.Translations
	if len(translations) == 0 {
		return nil, "", nil
	}

	byLocale := make(map[string]*ContentTranslation, len(translations))
	available := make([]string, 0, len(translations))
	for _, translation := range translations {
		code := s.localeCode(ctx, translation)
		if code == "" {
			continue
		}
		if _, ok := byLocale[code]; !ok {
			available = append(available, code)
			byLocale[code] = translation
		}
	}
	slices.Sort(available)

	requested = sharedi18n.NormalizeLocale(requested)
	fallback = sharedi18n.NormalizeLocale(fallback)
	if requested == "" {
		requested = sharedi18n.NormalizeLocale(record.PrimaryLocale)
	}
	if requested != "" {
		if translation := byLocale[requested]; translation != nil {
			return translation, requested, available
		}
	}
	if fallback != "" {
		if translation := byLocale[fallback]; translation != nil {
			return translation, fallback, available
		}
	}
	if primary := sharedi18n.NormalizeLocale(record.PrimaryLocale); primary != "" {
		if translation := byLocale[primary]; translation != nil {
			return translation, primary, available
		}
	}
	if len(available) == 0 {
		return nil, "", nil
	}
	return byLocale[available[0]], available[0], available
}

func (s *adminContentReadService) localeCode(ctx context.Context, translation *ContentTranslation) string {
	if translation == nil {
		return ""
	}
	if translation.Locale != nil {
		return sharedi18n.NormalizeLocale(translation.Locale.Code)
	}
	if s == nil || s.locales == nil || translation.LocaleID == uuid.Nil {
		return ""
	}
	locale, err := s.locales.GetByID(ctx, translation.LocaleID)
	if err != nil || locale == nil {
		return ""
	}
	return sharedi18n.NormalizeLocale(locale.Code)
}

func adminContentReadOptions(opts interfaces.AdminContentListOptions) []ContentListOption {
	out := make([]ContentListOption, 0, 4)
	if env := strings.TrimSpace(opts.EnvironmentKey); env != "" {
		out = append(out, ContentListOption(env))
	}
	out = append(out, WithTranslations(), WithProjection(ContentProjectionAdmin))
	return out
}

func adminContentGetOptions(opts interfaces.AdminContentGetOptions) []ContentGetOption {
	out := make([]ContentGetOption, 0, 4)
	if env := strings.TrimSpace(opts.EnvironmentKey); env != "" {
		out = append(out, ContentGetOption(env))
	}
	out = append(out, WithTranslations(), WithProjection(ContentProjectionAdmin))
	return out
}

func resolveAdminContentIncludes(isList bool, includeData, includeMetadata, includeBlocks bool, defaults *interfaces.AdminContentIncludeDefaults) (bool, bool, bool) {
	if defaults == nil {
		return true, true, true
	}
	base := defaults.Get
	if isList {
		base = defaults.List
	}
	return includeData || base.IncludeData, includeMetadata || base.IncludeMetadata, includeBlocks || base.IncludeBlocks
}

func matchesAdminContentFilters(record interfaces.AdminContentRecord, filters map[string]any, search string) bool {
	if strings.TrimSpace(search) != "" {
		query := strings.ToLower(strings.TrimSpace(search))
		if !strings.Contains(strings.ToLower(record.Title), query) &&
			!strings.Contains(strings.ToLower(record.Slug), query) &&
			!strings.Contains(strings.ToLower(record.ContentType), query) {
			return false
		}
	}
	for key, value := range filters {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "", "_search":
		case "locale":
			if !adminFilterStringMatch(record.Locale, value) {
				return false
			}
		case "resolved_locale":
			if !adminFilterStringMatch(record.ResolvedLocale, value) {
				return false
			}
		case "slug":
			if !adminFilterStringMatch(record.Slug, value) {
				return false
			}
		case "title":
			if !adminFilterStringMatch(record.Title, value) {
				return false
			}
		case "status":
			if !adminFilterStringMatch(record.Status, value) {
				return false
			}
		case "content_type", "content_type_slug":
			if !adminFilterStringMatch(adminFirstNonEmptyString(record.ContentTypeSlug, record.ContentType), value) {
				return false
			}
		case "family_id":
			if record.FamilyID == nil {
				return false
			}
			if !adminFilterStringMatch(record.FamilyID.String(), value) {
				return false
			}
		}
	}
	return true
}

func sortAdminContentRecords(records []interfaces.AdminContentRecord, sortBy string, desc bool) {
	key := strings.ToLower(strings.TrimSpace(sortBy))
	if key == "" {
		key = "id"
	}
	sort.SliceStable(records, func(i, j int) bool {
		less := compareAdminContentRecord(records[i], records[j], key)
		if desc {
			return !less
		}
		return less
	})
}

func compareAdminContentRecord(left, right interfaces.AdminContentRecord, key string) bool {
	switch key {
	case "title":
		return adminCompareStrings(left.Title, right.Title)
	case "slug":
		return adminCompareStrings(left.Slug, right.Slug)
	case "locale":
		return adminCompareStrings(left.Locale, right.Locale)
	case "content_type", "content_type_slug":
		return adminCompareStrings(adminFirstNonEmptyString(left.ContentTypeSlug, left.ContentType), adminFirstNonEmptyString(right.ContentTypeSlug, right.ContentType))
	case "status":
		return adminCompareStrings(left.Status, right.Status)
	case "created_at":
		return adminCompareTimePtrs(left.CreatedAt, right.CreatedAt)
	case "updated_at":
		return adminCompareTimePtrs(left.UpdatedAt, right.UpdatedAt)
	case "published_at":
		return adminCompareTimePtrs(left.PublishedAt, right.PublishedAt)
	default:
		return adminCompareStrings(left.ID.String(), right.ID.String())
	}
}

func projectAdminContentRecords(records []interfaces.AdminContentRecord, fields []string) []interfaces.AdminContentRecord {
	selected := normalizeAdminContentFields(fields)
	if len(selected) == 0 {
		out := make([]interfaces.AdminContentRecord, len(records))
		copy(out, records)
		return out
	}
	out := make([]interfaces.AdminContentRecord, len(records))
	for idx := range records {
		out[idx] = projectAdminContentRecord(records[idx], selected)
	}
	return out
}

func normalizeAdminContentFields(fields []string) map[string]struct{} {
	if len(fields) == 0 {
		return nil
	}
	selected := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		key := strings.ToLower(strings.TrimSpace(field))
		if key == "" {
			continue
		}
		selected[key] = struct{}{}
	}
	return selected
}

func projectAdminContentRecord(record interfaces.AdminContentRecord, selected map[string]struct{}) interfaces.AdminContentRecord {
	if len(selected) == 0 {
		return record
	}
	out := interfaces.AdminContentRecord{ID: record.ID}
	for field := range selected {
		switch field {
		case "family_id":
			out.FamilyID = cloneUUIDPointer(record.FamilyID)
		case "title":
			out.Title = record.Title
		case "slug":
			out.Slug = record.Slug
		case "locale":
			out.Locale = record.Locale
		case "requested_locale":
			out.RequestedLocale = record.RequestedLocale
		case "resolved_locale":
			out.ResolvedLocale = record.ResolvedLocale
		case "available_locales":
			out.AvailableLocales = append([]string{}, record.AvailableLocales...)
		case "missing_requested_locale":
			out.MissingRequestedLocale = record.MissingRequestedLocale
		case "navigation":
			out.Navigation = cloneStringMap(record.Navigation)
		case "effective_menu_locations":
			out.EffectiveMenuLocations = append([]string{}, record.EffectiveMenuLocations...)
		case "content_type":
			out.ContentType = record.ContentType
		case "content_type_slug":
			out.ContentTypeSlug = record.ContentTypeSlug
		case "status":
			out.Status = record.Status
		case "blocks":
			out.Blocks = append([]string{}, record.Blocks...)
		case "embedded_blocks":
			out.EmbeddedBlocks = cloneAnyMapSlice(record.EmbeddedBlocks)
		case "schema_version":
			out.SchemaVersion = record.SchemaVersion
		case "data":
			out.Data = cloneAnyMap(record.Data)
		case "metadata":
			out.Metadata = cloneAnyMap(record.Metadata)
		case "created_at":
			out.CreatedAt = adminCloneTimePtr(record.CreatedAt)
		case "updated_at":
			out.UpdatedAt = adminCloneTimePtr(record.UpdatedAt)
		case "published_at":
			out.PublishedAt = adminCloneTimePtr(record.PublishedAt)
		}
	}
	return out
}

func paginateAdminContentRecords(records []interfaces.AdminContentRecord, page, perPage int) []interfaces.AdminContentRecord {
	if page <= 0 || perPage <= 0 {
		out := make([]interfaces.AdminContentRecord, len(records))
		copy(out, records)
		return out
	}
	start := (page - 1) * perPage
	if start >= len(records) {
		return []interfaces.AdminContentRecord{}
	}
	end := min(start+perPage, len(records))
	out := make([]interfaces.AdminContentRecord, end-start)
	copy(out, records[start:end])
	return out
}

func adminContentMetadata(navigation map[string]string, effectiveMenuLocations []string, metadata map[string]any) map[string]any {
	cloned := cloneAnyMap(metadata)
	if len(navigation) == 0 && len(effectiveMenuLocations) == 0 {
		return cloned
	}
	if cloned == nil {
		cloned = map[string]any{}
	}
	normalized := make(map[string]any, len(navigation))
	for key, value := range navigation {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		normalized[key] = value
	}
	if len(normalized) > 0 {
		cloned[entryFieldNavigation] = normalized
	}
	if len(effectiveMenuLocations) > 0 {
		cloned[entryFieldEffectiveMenuLocations] = cloneStringSlice(effectiveMenuLocations)
	}
	return cloned
}

func adminContentTranslationInput(locale, title string, data map[string]any, blocks []string, embedded []map[string]any, schemaVersion string) ContentTranslationInput {
	payload := cloneAnyMap(data)
	if payload == nil {
		payload = map[string]any{}
	}
	if len(blocks) > 0 {
		payload["blocks"] = cloneStringSlice(blocks)
	}
	if len(embedded) > 0 {
		payload = MergeEmbeddedBlocks(payload, embedded)
	}
	if strings.TrimSpace(schemaVersion) != "" && strings.TrimSpace(adminToString(payload["_schema"])) == "" {
		payload["_schema"] = strings.TrimSpace(schemaVersion)
	}
	return ContentTranslationInput{
		Locale:  sharedi18n.NormalizeLocale(locale),
		Title:   strings.TrimSpace(title),
		Content: payload,
	}
}

func anySliceToStrings(value any) []string {
	switch typed := value.(type) {
	case []string:
		return cloneStringSlice(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(adminToString(item))
			if text == "" {
				continue
			}
			out = append(out, text)
		}
		return out
	default:
		return nil
	}
}

func anySliceToMapSlice(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return cloneAnyMapSlice(typed)
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			record, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, cloneAnyMap(record))
		}
		return out
	default:
		return nil
	}
}

func cloneAnyMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]any, len(value))
	maps.Copy(out, value)
	return out
}

func cloneAnyMapSlice(values []map[string]any) []map[string]any {
	if len(values) == 0 {
		return nil
	}
	out := make([]map[string]any, len(values))
	for idx := range values {
		out[idx] = cloneAnyMap(values[idx])
	}
	return out
}

func cloneStringMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]string, len(value))
	maps.Copy(out, value)
	return out
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func adminCloneTimeValuePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value
	return &copy
}

func adminCloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func mapValue(value map[string]any, key string) any {
	if len(value) == 0 {
		return nil
	}
	return value[key]
}

func adminToString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func adminFirstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func adminCompareStrings(left, right string) bool {
	return strings.ToLower(strings.TrimSpace(left)) < strings.ToLower(strings.TrimSpace(right))
}

func adminCompareTimePtrs(left, right *time.Time) bool {
	if left == nil {
		return right != nil
	}
	if right == nil {
		return false
	}
	return left.Before(*right)
}

func adminFilterStringMatch(actual string, expected any) bool {
	want := strings.TrimSpace(adminToString(expected))
	if want == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(actual), want)
}
