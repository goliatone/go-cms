package pages

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	cmsschema "github.com/goliatone/go-cms/internal/schema"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// AdminPageReadOption configures the admin read service.
type AdminPageReadOption func(*adminPageReadService)

// WithAdminPageReadLogger overrides the logger used by the admin read service.
func WithAdminPageReadLogger(logger interfaces.Logger) AdminPageReadOption {
	return func(s *adminPageReadService) {
		s.logger = logger
	}
}

// NewAdminPageReadService constructs the default admin read service.
func NewAdminPageReadService(pageSvc Service, contentSvc content.Service, locales content.LocaleRepository, opts ...AdminPageReadOption) interfaces.AdminPageReadService {
	service := &adminPageReadService{
		pages:   pageSvc,
		content: contentSvc,
		locales: locales,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

type adminPageReadService struct {
	pages   Service
	content content.Service
	locales content.LocaleRepository
	logger  interfaces.Logger
}

// List returns admin page records with optional filtering and pagination.
func (s *adminPageReadService) List(ctx context.Context, opts interfaces.AdminPageListOptions) ([]interfaces.AdminPageRecord, int, error) {
	if s == nil || s.pages == nil {
		return nil, 0, errors.New("pages: admin read service requires page service")
	}

	includes := resolveAdminPageIncludes(true, opts.IncludeContent, opts.IncludeBlocks, opts.IncludeData, opts.DefaultIncludes)
	requestedLocale := strings.TrimSpace(opts.Locale)
	primaryLocaleID, primaryLocaleCode, err := s.resolveLocale(ctx, requestedLocale)
	if err != nil {
		return nil, 0, err
	}
	fallbackLocaleID, fallbackLocaleCode, err := s.resolveLocale(ctx, opts.FallbackLocale)
	if err != nil {
		return nil, 0, err
	}

	pages, err := s.pages.List(ctx, opts.EnvironmentKey)
	if err != nil {
		return nil, 0, err
	}

	filtered := make([]interfaces.AdminPageRecord, 0, len(pages))
	for _, page := range pages {
		record, err := s.buildRecord(ctx, page, adminReadContext{
			requestedLocale:  requestedLocale,
			primaryLocaleID:  primaryLocaleID,
			primaryLocale:    primaryLocaleCode,
			fallbackLocaleID: fallbackLocaleID,
			fallbackLocale:   fallbackLocaleCode,
			allowMissing:     true,
			includes:         includes,
		})
		if err != nil {
			return nil, 0, err
		}
		if !matchesAdminPageFilters(record, opts.Filters, opts.Search) {
			continue
		}
		filtered = append(filtered, record)
	}

	sortAdminPageRecords(filtered, opts.SortBy, opts.SortDesc)
	total := len(filtered)

	pageNum := opts.Page
	perPage := opts.PerPage
	if pageNum <= 0 || perPage <= 0 {
		return filtered, total, nil
	}

	start := (pageNum - 1) * perPage
	if start >= total {
		return []interfaces.AdminPageRecord{}, total, nil
	}
	end := start + perPage
	if end > total {
		end = total
	}
	return filtered[start:end], total, nil
}

// Get returns a single admin page record by identifier.
func (s *adminPageReadService) Get(ctx context.Context, id string, opts interfaces.AdminPageGetOptions) (*interfaces.AdminPageRecord, error) {
	if s == nil || s.pages == nil {
		return nil, errors.New("pages: admin read service requires page service")
	}
	pageID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil || pageID == uuid.Nil {
		return nil, ErrPageRequired
	}

	includes := resolveAdminPageIncludes(false, opts.IncludeContent, opts.IncludeBlocks, opts.IncludeData, opts.DefaultIncludes)
	requestedLocale := strings.TrimSpace(opts.Locale)
	primaryLocaleID, primaryLocaleCode, err := s.resolveLocale(ctx, requestedLocale)
	if err != nil {
		return nil, err
	}
	fallbackLocaleID, fallbackLocaleCode, err := s.resolveLocale(ctx, opts.FallbackLocale)
	if err != nil {
		return nil, err
	}

	var record *Page
	if strings.TrimSpace(opts.EnvironmentKey) != "" {
		listed, err := s.pages.List(ctx, opts.EnvironmentKey)
		if err != nil {
			return nil, err
		}
		for _, candidate := range listed {
			if candidate != nil && candidate.ID == pageID {
				record = candidate
				break
			}
		}
		if record == nil {
			return nil, &PageNotFoundError{Key: pageID.String()}
		}
	} else {
		record, err = s.pages.Get(ctx, pageID)
		if err != nil {
			return nil, err
		}
	}

	result, err := s.buildRecord(ctx, record, adminReadContext{
		requestedLocale:  requestedLocale,
		primaryLocaleID:  primaryLocaleID,
		primaryLocale:    primaryLocaleCode,
		fallbackLocaleID: fallbackLocaleID,
		fallbackLocale:   fallbackLocaleCode,
		allowMissing:     opts.AllowMissingTranslations,
		includes:         includes,
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

type adminReadContext struct {
	requestedLocale  string
	primaryLocaleID  uuid.UUID
	primaryLocale    string
	fallbackLocaleID uuid.UUID
	fallbackLocale   string
	allowMissing     bool
	includes         interfaces.AdminPageIncludeOptions
}

func (s *adminPageReadService) resolveLocale(ctx context.Context, code string) (uuid.UUID, string, error) {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return uuid.Nil, "", nil
	}
	if s == nil || s.locales == nil {
		return uuid.Nil, trimmed, nil
	}
	locale, err := s.locales.GetByCode(ctx, trimmed)
	if err != nil {
		return uuid.Nil, trimmed, err
	}
	if locale == nil {
		return uuid.Nil, trimmed, fmt.Errorf("pages: locale %q not found", trimmed)
	}
	return locale.ID, locale.Code, nil
}

func (s *adminPageReadService) buildRecord(ctx context.Context, page *Page, state adminReadContext) (interfaces.AdminPageRecord, error) {
	if page == nil {
		return interfaces.AdminPageRecord{}, &PageNotFoundError{Key: ""}
	}

	record := interfaces.AdminPageRecord{
		ID:              page.ID,
		ContentID:       page.ContentID,
		TemplateID:      page.TemplateID,
		Slug:            page.Slug,
		Status:          page.Status,
		ParentID:        page.ParentID,
		RequestedLocale: state.requestedLocale,
		PublishedAt:     cloneTimePtr(page.PublishedAt),
		CreatedAt:       cloneTimePtr(&page.CreatedAt),
		UpdatedAt:       cloneTimePtr(&page.UpdatedAt),
	}

	pageAvailableLocales := collectAdminPageLocales(ctx, s.locales, page)
	pageTranslation, pageResolvedLocale := resolvePageTranslation(page.Translations, state.primaryLocaleID, state.primaryLocale, state.fallbackLocaleID, state.fallbackLocale)
	record.ResolvedLocale = pageResolvedLocale
	record.Translation = buildAdminPageTranslationBundle(state.requestedLocale, pageResolvedLocale, pageTranslation, pageAvailableLocales, page.PrimaryLocale)
	if pageTranslation == nil {
		if !state.allowMissing && strings.TrimSpace(state.requestedLocale) != "" {
			return interfaces.AdminPageRecord{}, errors.Join(interfaces.ErrTranslationMissing, ErrPageTranslationNotFound)
		}
	} else {
		record.TranslationGroupID = pageTranslation.TranslationGroupID
		record.Title = pageTranslation.Title
		record.Path = pageTranslation.Path
		record.MetaTitle = stringValue(pageTranslation.SEOTitle)
		record.MetaDescription = stringValue(pageTranslation.SEODescription)
		record.Summary = cloneStringPtr(pageTranslation.Summary)
	}

	contentRecord := page.Content
	needsContent := state.includes.IncludeContent || state.includes.IncludeData || state.includes.IncludeBlocks
	if (needsContent || contentRecord == nil) && s != nil && s.content != nil && page.ContentID != uuid.Nil {
		fetched, err := s.content.Get(ctx, page.ContentID)
		if err == nil {
			contentRecord = fetched
		}
	}

	contentAvailableLocales := collectAdminContentLocales(ctx, s.locales, contentRecord)
	contentPrimaryLocale := ""
	if contentRecord != nil {
		contentPrimaryLocale = contentRecord.PrimaryLocale
	}
	contentTranslation, contentResolvedLocale := resolveContentTranslation(contentRecord, state.primaryLocaleID, state.primaryLocale, state.fallbackLocaleID, state.fallbackLocale)
	record.ContentTranslation = buildAdminContentTranslationBundle(state.requestedLocale, contentResolvedLocale, contentTranslation, contentAvailableLocales, contentPrimaryLocale)
	if contentTranslation != nil && record.Summary == nil {
		record.Summary = cloneStringPtr(contentTranslation.Summary)
	}

	if contentRecord != nil && contentRecord.Type != nil {
		record.SchemaVersion = strings.TrimSpace(contentRecord.Type.SchemaVersion)
	}
	if record.SchemaVersion == "" && contentTranslation != nil {
		if version, ok := contentTranslation.Content[cmsschema.RootSchemaKey].(string); ok {
			record.SchemaVersion = strings.TrimSpace(version)
		}
	}

	if contentTranslation != nil {
		record.Tags = extractTags(contentTranslation.Content)
		if record.TranslationGroupID == nil && contentTranslation.TranslationGroupID != nil {
			record.TranslationGroupID = contentTranslation.TranslationGroupID
		}
	}

	if state.includes.IncludeData {
		record.Data = buildTranslationData(pageTranslation, contentTranslation, record.RequestedLocale, record.ResolvedLocale)
		if record.MetaTitle == "" {
			record.MetaTitle = stringFromData(record.Data, "meta_title")
		}
		if record.MetaDescription == "" {
			record.MetaDescription = stringFromData(record.Data, "meta_description")
		}
		if len(record.Tags) == 0 {
			record.Tags = extractTags(record.Data)
		}
	}

	if state.includes.IncludeContent {
		record.Content = normalizeContentPayload(contentTranslation)
	}

	if state.includes.IncludeBlocks {
		if contentTranslation != nil {
			if embedded, ok := content.ExtractEmbeddedBlocks(contentTranslation.Content); ok {
				record.Blocks = embedded
			}
		}
		if record.Blocks == nil {
			record.Blocks = extractLegacyBlockIDs(page.Blocks)
		}
	}

	return record, nil
}

func resolveAdminPageIncludes(isList bool, includeContent, includeBlocks, includeData bool, defaults *interfaces.AdminPageIncludeDefaults) interfaces.AdminPageIncludeOptions {
	if includeContent || includeBlocks || includeData {
		return interfaces.AdminPageIncludeOptions{
			IncludeContent: includeContent,
			IncludeBlocks:  includeBlocks,
			IncludeData:    includeData,
		}
	}
	if defaults != nil {
		if isList {
			return defaults.List
		}
		return defaults.Get
	}
	if isList {
		return interfaces.AdminPageIncludeOptions{}
	}
	return interfaces.AdminPageIncludeOptions{
		IncludeContent: true,
		IncludeBlocks:  false,
		IncludeData:    true,
	}
}

func resolvePageTranslation(translations []*PageTranslation, primaryID uuid.UUID, primaryCode string, fallbackID uuid.UUID, fallbackCode string) (*PageTranslation, string) {
	if len(translations) == 0 {
		return nil, ""
	}
	if primaryID != uuid.Nil {
		for _, tr := range translations {
			if tr != nil && tr.LocaleID == primaryID {
				return tr, primaryCode
			}
		}
	}
	if primaryCode != "" {
		for _, tr := range translations {
			if tr != nil && strings.EqualFold(tr.Locale, primaryCode) {
				return tr, primaryCode
			}
		}
	}
	if fallbackID != uuid.Nil {
		for _, tr := range translations {
			if tr != nil && tr.LocaleID == fallbackID {
				return tr, fallbackCode
			}
		}
	}
	if fallbackCode != "" {
		for _, tr := range translations {
			if tr != nil && strings.EqualFold(tr.Locale, fallbackCode) {
				return tr, fallbackCode
			}
		}
	}
	return nil, ""
}

func resolveContentTranslation(record *content.Content, primaryID uuid.UUID, primaryCode string, fallbackID uuid.UUID, fallbackCode string) (*content.ContentTranslation, string) {
	if record == nil || len(record.Translations) == 0 {
		return nil, ""
	}
	if primaryID != uuid.Nil {
		for _, tr := range record.Translations {
			if tr != nil && tr.LocaleID == primaryID {
				return tr, resolveContentTranslationLocale(tr, primaryCode)
			}
		}
	}
	if primaryCode != "" {
		for _, tr := range record.Translations {
			if tr == nil {
				continue
			}
			if tr.Locale != nil && strings.EqualFold(tr.Locale.Code, primaryCode) {
				return tr, resolveContentTranslationLocale(tr, primaryCode)
			}
		}
	}
	if fallbackID != uuid.Nil {
		for _, tr := range record.Translations {
			if tr != nil && tr.LocaleID == fallbackID {
				return tr, resolveContentTranslationLocale(tr, fallbackCode)
			}
		}
	}
	if fallbackCode != "" {
		for _, tr := range record.Translations {
			if tr == nil {
				continue
			}
			if tr.Locale != nil && strings.EqualFold(tr.Locale.Code, fallbackCode) {
				return tr, resolveContentTranslationLocale(tr, fallbackCode)
			}
		}
	}
	return nil, ""
}

func resolveContentTranslationLocale(tr *content.ContentTranslation, fallback string) string {
	if tr == nil {
		return strings.TrimSpace(fallback)
	}
	if tr.Locale != nil {
		if code := strings.TrimSpace(tr.Locale.Code); code != "" {
			return code
		}
	}
	return strings.TrimSpace(fallback)
}

func collectAdminPageLocales(ctx context.Context, locales content.LocaleRepository, page *Page) []string {
	if page == nil || len(page.Translations) == 0 {
		return nil
	}
	localesList := make([]string, 0, len(page.Translations))
	seen := map[string]struct{}{}
	for _, tr := range page.Translations {
		if tr == nil {
			continue
		}
		code := strings.TrimSpace(tr.Locale)
		if code == "" && locales != nil && tr.LocaleID != uuid.Nil {
			code = adminLocaleCodeByID(ctx, locales, tr.LocaleID)
		}
		if code == "" {
			code = tr.LocaleID.String()
		}
		if code == "" {
			continue
		}
		key := strings.ToLower(code)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		localesList = append(localesList, code)
	}
	if len(localesList) == 0 {
		return nil
	}
	return localesList
}

func collectAdminContentLocales(ctx context.Context, locales content.LocaleRepository, record *content.Content) []string {
	if record == nil || len(record.Translations) == 0 {
		return nil
	}
	localesList := make([]string, 0, len(record.Translations))
	seen := map[string]struct{}{}
	for _, tr := range record.Translations {
		if tr == nil {
			continue
		}
		code := ""
		if tr.Locale != nil {
			code = strings.TrimSpace(tr.Locale.Code)
		}
		if code == "" && locales != nil && tr.LocaleID != uuid.Nil {
			code = adminLocaleCodeByID(ctx, locales, tr.LocaleID)
		}
		if code == "" {
			code = tr.LocaleID.String()
		}
		if code == "" {
			continue
		}
		key := strings.ToLower(code)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		localesList = append(localesList, code)
	}
	if len(localesList) == 0 {
		return nil
	}
	return localesList
}

func adminLocaleCodeByID(ctx context.Context, locales content.LocaleRepository, id uuid.UUID) string {
	if locales == nil || id == uuid.Nil {
		return ""
	}
	locale, err := locales.GetByID(ctx, id)
	if err != nil || locale == nil {
		return ""
	}
	return strings.TrimSpace(locale.Code)
}

func buildTranslationData(pageTranslation *PageTranslation, contentTranslation *content.ContentTranslation, requestedLocale, resolvedLocale string) map[string]any {
	var data map[string]any
	if contentTranslation != nil {
		data = cloneAdminMap(contentTranslation.Content)
	}
	if data == nil {
		data = map[string]any{}
	}
	if pageTranslation != nil {
		if strings.TrimSpace(pageTranslation.Title) != "" {
			data["title"] = pageTranslation.Title
		}
		if strings.TrimSpace(pageTranslation.Path) != "" {
			data["path"] = pageTranslation.Path
		}
		if pageTranslation.Summary != nil {
			data["summary"] = *pageTranslation.Summary
		}
		if pageTranslation.SEOTitle != nil {
			data["meta_title"] = *pageTranslation.SEOTitle
		}
		if pageTranslation.SEODescription != nil {
			data["meta_description"] = *pageTranslation.SEODescription
		}
	}
	if contentTranslation != nil && contentTranslation.Summary != nil {
		if _, ok := data["summary"]; !ok {
			data["summary"] = *contentTranslation.Summary
		}
	}
	if strings.TrimSpace(requestedLocale) != "" {
		data["requested_locale"] = requestedLocale
	}
	if strings.TrimSpace(resolvedLocale) != "" {
		data["resolved_locale"] = resolvedLocale
	}
	return data
}

func normalizeContentPayload(translation *content.ContentTranslation) any {
	if translation == nil || translation.Content == nil {
		return nil
	}
	if len(translation.Content) == 1 {
		if value, ok := translation.Content["content"]; ok {
			if text, ok := value.(string); ok {
				return text
			}
		}
		if value, ok := translation.Content["body"]; ok {
			if text, ok := value.(string); ok {
				return text
			}
		}
	}
	return cloneAdminMap(translation.Content)
}

func extractLegacyBlockIDs(blocks []*blocks.Instance) []string {
	if len(blocks) == 0 {
		return nil
	}
	ids := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block == nil || block.ID == uuid.Nil {
			continue
		}
		ids = append(ids, block.ID.String())
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func extractTags(payload map[string]any) []string {
	if payload == nil {
		return nil
	}
	raw, ok := payload["tags"]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, entry := range typed {
			if text, ok := entry.(string); ok {
				out = append(out, text)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return nil
	}
}

func stringFromData(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	if value, ok := data[key]; ok {
		if text, ok := value.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func cloneAdminMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	return maps.Clone(input)
}

func buildAdminPageTranslationBundle(requestedLocale, resolvedLocale string, translation *PageTranslation, availableLocales []string, primaryLocale string) interfaces.TranslationBundle[interfaces.PageTranslation] {
	meta := buildAdminTranslationMeta(requestedLocale, resolvedLocale, availableLocales, primaryLocale)
	bundle := interfaces.TranslationBundle[interfaces.PageTranslation]{
		Meta: meta,
	}
	if translation == nil {
		return bundle
	}
	value := toInterfacesPageTranslation(translation, meta.ResolvedLocale)
	if value == nil {
		return bundle
	}
	if meta.RequestedLocale != "" && strings.EqualFold(meta.RequestedLocale, meta.ResolvedLocale) {
		bundle.Requested = value
		bundle.Resolved = value
		return bundle
	}
	bundle.Resolved = value
	return bundle
}

func buildAdminContentTranslationBundle(requestedLocale, resolvedLocale string, translation *content.ContentTranslation, availableLocales []string, primaryLocale string) interfaces.TranslationBundle[interfaces.ContentTranslation] {
	meta := buildAdminTranslationMeta(requestedLocale, resolvedLocale, availableLocales, primaryLocale)
	bundle := interfaces.TranslationBundle[interfaces.ContentTranslation]{
		Meta: meta,
	}
	if translation == nil {
		return bundle
	}
	value := toInterfacesContentTranslation(translation, meta.ResolvedLocale)
	if value == nil {
		return bundle
	}
	if meta.RequestedLocale != "" && strings.EqualFold(meta.RequestedLocale, meta.ResolvedLocale) {
		bundle.Requested = value
		bundle.Resolved = value
		return bundle
	}
	bundle.Resolved = value
	return bundle
}

func buildAdminTranslationMeta(requestedLocale, resolvedLocale string, availableLocales []string, primaryLocale string) interfaces.TranslationMeta {
	requested := strings.TrimSpace(requestedLocale)
	resolved := strings.TrimSpace(resolvedLocale)
	meta := interfaces.TranslationMeta{
		RequestedLocale: requested,
		ResolvedLocale:  resolved,
		PrimaryLocale:   strings.TrimSpace(primaryLocale),
	}
	if len(availableLocales) > 0 {
		meta.AvailableLocales = append([]string(nil), availableLocales...)
	}
	if requested != "" && !strings.EqualFold(requested, resolved) {
		meta.MissingRequestedLocale = true
		if resolved != "" {
			meta.FallbackUsed = true
		}
	}
	return meta
}

func toInterfacesPageTranslation(translation *PageTranslation, locale string) *interfaces.PageTranslation {
	if translation == nil {
		return nil
	}
	resolvedLocale := strings.TrimSpace(locale)
	if strings.TrimSpace(translation.Locale) != "" {
		resolvedLocale = strings.TrimSpace(translation.Locale)
	}
	return &interfaces.PageTranslation{
		ID:      translation.ID,
		Locale:  resolvedLocale,
		Title:   translation.Title,
		Path:    translation.Path,
		Summary: cloneStringPtr(translation.Summary),
	}
}

func toInterfacesContentTranslation(translation *content.ContentTranslation, locale string) *interfaces.ContentTranslation {
	if translation == nil {
		return nil
	}
	resolvedLocale := strings.TrimSpace(locale)
	if translation.Locale != nil && strings.TrimSpace(translation.Locale.Code) != "" {
		resolvedLocale = strings.TrimSpace(translation.Locale.Code)
	}
	return &interfaces.ContentTranslation{
		ID:      translation.ID,
		Locale:  resolvedLocale,
		Title:   translation.Title,
		Summary: cloneStringPtr(translation.Summary),
		Fields:  cloneAdminMap(translation.Content),
	}
}

func matchesAdminPageFilters(record interfaces.AdminPageRecord, filters map[string]any, search string) bool {
	if len(filters) > 0 {
		if !filterStringMatch(record.Status, filters["status"]) {
			return false
		}
		if !filterStringMatch(record.ResolvedLocale, filters["locale"]) {
			return false
		}
		if !filterStringMatch(record.TemplateID.String(), filters["template_id"]) {
			return false
		}
		if !filterStringMatch(record.ContentID.String(), filters["content_id"]) {
			return false
		}
		if record.ParentID != nil {
			if !filterStringMatch(record.ParentID.String(), filters["parent_id"]) {
				return false
			}
		} else if hasFilter(filters["parent_id"]) {
			return false
		}
	}

	term := strings.TrimSpace(search)
	if term == "" {
		return true
	}
	term = strings.ToLower(term)
	if strings.Contains(strings.ToLower(record.Title), term) {
		return true
	}
	if strings.Contains(strings.ToLower(record.Slug), term) {
		return true
	}
	if strings.Contains(strings.ToLower(record.Path), term) {
		return true
	}
	if record.Summary != nil && strings.Contains(strings.ToLower(*record.Summary), term) {
		return true
	}
	return false
}

func filterStringMatch(value string, filter any) bool {
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
			if text, ok := entry.(string); ok && strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(text)) {
				return true
			}
		}
	}
	return false
}

func hasFilter(filter any) bool {
	switch typed := filter.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case []string:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	default:
		return false
	}
}

func sortAdminPageRecords(records []interfaces.AdminPageRecord, sortBy string, desc bool) {
	key := strings.TrimSpace(strings.ToLower(sortBy))
	if key == "" || len(records) < 2 {
		return
	}
	sort.Slice(records, func(i, j int) bool {
		less := compareAdminPageRecord(records[i], records[j], key)
		if desc {
			return !less
		}
		return less
	})
}

func compareAdminPageRecord(left, right interfaces.AdminPageRecord, key string) bool {
	switch key {
	case "title":
		return strings.ToLower(left.Title) < strings.ToLower(right.Title)
	case "slug":
		return strings.ToLower(left.Slug) < strings.ToLower(right.Slug)
	case "path":
		return strings.ToLower(left.Path) < strings.ToLower(right.Path)
	case "status":
		return strings.ToLower(left.Status) < strings.ToLower(right.Status)
	case "created_at":
		return timePtrValue(left.CreatedAt).Before(timePtrValue(right.CreatedAt))
	case "updated_at":
		return timePtrValue(left.UpdatedAt).Before(timePtrValue(right.UpdatedAt))
	case "published_at":
		return timePtrValue(left.PublishedAt).Before(timePtrValue(right.PublishedAt))
	default:
		return false
	}
}

func timePtrValue(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
