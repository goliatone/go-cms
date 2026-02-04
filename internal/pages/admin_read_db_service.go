package pages

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	cmsschema "github.com/goliatone/go-cms/internal/schema"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// NewAdminPageDBReadService constructs a DB-backed admin page read service.
func NewAdminPageDBReadService(db *bun.DB, pageSvc Service, contentSvc content.Service, locales content.LocaleRepository, opts ...AdminPageReadOption) interfaces.AdminPageReadService {
	base := &adminPageReadService{
		pages:   pageSvc,
		content: contentSvc,
		locales: locales,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(base)
		}
	}
	return &adminPageDBReadService{
		db:   db,
		base: base,
	}
}

type adminPageDBReadService struct {
	db   *bun.DB
	base *adminPageReadService
}

// List returns admin page records using SQL-backed filtering/sorting/pagination.
func (s *adminPageDBReadService) List(ctx context.Context, opts interfaces.AdminPageListOptions) ([]interfaces.AdminPageRecord, int, error) {
	if s == nil || s.base == nil || s.base.pages == nil {
		return nil, 0, errors.New("pages: admin read service requires page service")
	}
	if s.db == nil {
		return nil, 0, errors.New("pages: admin read service requires database")
	}

	includes := resolveAdminPageIncludes(true, opts.IncludeContent, opts.IncludeBlocks, opts.IncludeData, opts.DefaultIncludes)
	requestedLocale := strings.TrimSpace(opts.Locale)
	primaryLocaleID, primaryLocaleCode, err := s.base.resolveLocale(ctx, requestedLocale)
	if err != nil {
		return nil, 0, err
	}
	fallbackLocaleID, fallbackLocaleCode, err := s.base.resolveLocale(ctx, opts.FallbackLocale)
	if err != nil {
		return nil, 0, err
	}
	primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode = dedupeAdminLocales(primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)

	envFilter, err := resolveAdminPageEnvironment(ctx, s.base.pages, opts.EnvironmentKey)
	if err != nil {
		return nil, 0, err
	}

	if requestedLocale != "" && !opts.AllowMissingTranslations {
		missing, err := s.countMissingTranslations(ctx, envFilter, primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)
		if err != nil {
			return nil, 0, err
		}
		if missing > 0 {
			return nil, 0, ErrPageTranslationNotFound
		}
	}

	countQuery, expr, err := s.newAdminPageDBQuery(ctx, envFilter, primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)
	if err != nil {
		return nil, 0, err
	}
	if !applyAdminPageDBFilters(countQuery, expr, opts.Filters, opts.Search) {
		return []interfaces.AdminPageRecord{}, 0, nil
	}
	total, err := countQuery.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	listQuery, expr, err := s.newAdminPageDBQuery(ctx, envFilter, primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)
	if err != nil {
		return nil, 0, err
	}
	if !applyAdminPageDBFilters(listQuery, expr, opts.Filters, opts.Search) {
		return []interfaces.AdminPageRecord{}, 0, nil
	}
	applyAdminPageDBSort(listQuery, expr, opts.SortBy, opts.SortDesc)
	applyAdminPageDBPagination(listQuery, opts.Page, opts.PerPage)
	listQuery.
		ColumnExpr("p.id AS id").
		ColumnExpr("p.content_id AS content_id").
		ColumnExpr("p.template_id AS template_id").
		ColumnExpr("p.slug AS slug").
		ColumnExpr("p.status AS status").
		ColumnExpr("p.parent_id AS parent_id").
		ColumnExpr("p.published_at AS published_at").
		ColumnExpr("p.created_at AS created_at").
		ColumnExpr("p.updated_at AS updated_at").
		ColumnExpr(expr.title+" AS title").
		ColumnExpr(expr.path+" AS path").
		ColumnExpr(expr.metaTitle+" AS meta_title").
		ColumnExpr(expr.metaDescription+" AS meta_description").
		ColumnExpr(expr.summary+" AS summary").
		ColumnExpr(expr.translationGroupID+" AS translation_group_id").
		ColumnExpr("ct.content AS content_payload").
		ColumnExpr(expr.schemaVersion+" AS schema_version").
		ColumnExpr(expr.resolvedLocale+" AS resolved_locale", expr.resolvedLocaleArgs...)

	var rows []adminPageDBRow
	if err := listQuery.Scan(ctx, &rows); err != nil {
		return nil, 0, err
	}

	if len(rows) == 0 {
		return []interfaces.AdminPageRecord{}, total, nil
	}

	records := make([]interfaces.AdminPageRecord, 0, len(rows))
	state := adminReadContext{
		requestedLocale:  requestedLocale,
		primaryLocaleID:  primaryLocaleID,
		primaryLocale:    primaryLocaleCode,
		fallbackLocaleID: fallbackLocaleID,
		fallbackLocale:   fallbackLocaleCode,
		allowMissing:     opts.AllowMissingTranslations,
		includes:         includes,
	}
	for _, row := range rows {
		if includes.IncludeContent || includes.IncludeBlocks || includes.IncludeData {
			page, err := s.base.pages.Get(ctx, row.ID)
			if err != nil {
				return nil, 0, err
			}
			record, err := s.base.buildRecord(ctx, page, state)
			if err != nil {
				return nil, 0, err
			}
			records = append(records, record)
			continue
		}
		record := mapAdminPageDBRow(row, requestedLocale)
		records = append(records, record)
	}

	return records, total, nil
}

// Get delegates to the default admin page read service implementation.
func (s *adminPageDBReadService) Get(ctx context.Context, id string, opts interfaces.AdminPageGetOptions) (*interfaces.AdminPageRecord, error) {
	if s == nil || s.base == nil {
		return nil, errors.New("pages: admin read service requires page service")
	}
	return s.base.Get(ctx, id, opts)
}

type adminPageDBRow struct {
	ID                 uuid.UUID      `bun:"id"`
	ContentID          uuid.UUID      `bun:"content_id"`
	TranslationGroupID *uuid.UUID     `bun:"translation_group_id"`
	TemplateID         uuid.UUID      `bun:"template_id"`
	Title              string         `bun:"title"`
	Slug               string         `bun:"slug"`
	Path               string         `bun:"path"`
	ResolvedLocale     string         `bun:"resolved_locale"`
	Status             string         `bun:"status"`
	ParentID           *uuid.UUID     `bun:"parent_id"`
	MetaTitle          string         `bun:"meta_title"`
	MetaDescription    string         `bun:"meta_description"`
	Summary            *string        `bun:"summary"`
	ContentPayload     map[string]any `bun:"content_payload,type:jsonb"`
	SchemaVersion      string         `bun:"schema_version"`
	PublishedAt        *time.Time     `bun:"published_at"`
	CreatedAt          *time.Time     `bun:"created_at"`
	UpdatedAt          *time.Time     `bun:"updated_at"`
}

type adminPageDBExpressions struct {
	title              string
	path               string
	summary            string
	metaTitle          string
	metaDescription    string
	translationGroupID string
	schemaVersion      string
	resolvedLocale     string
	resolvedLocaleArgs []any
}

type adminPageEnvironmentFilter struct {
	id  uuid.UUID
	key string
}

func resolveAdminPageEnvironment(ctx context.Context, svc Service, envKey string) (adminPageEnvironmentFilter, error) {
	filter := adminPageEnvironmentFilter{key: strings.TrimSpace(envKey)}
	pageSvc, ok := svc.(*pageService)
	if !ok || pageSvc == nil {
		return filter, nil
	}
	envID, _, err := pageSvc.resolveEnvironment(ctx, filter.key)
	if err != nil {
		return filter, err
	}
	filter.id = envID
	return filter, nil
}

func (s *adminPageDBReadService) newAdminPageDBQuery(ctx context.Context, env adminPageEnvironmentFilter, primaryLocaleID uuid.UUID, primaryLocaleCode string, fallbackLocaleID uuid.UUID, fallbackLocaleCode string) (*bun.SelectQuery, adminPageDBExpressions, error) {
	query := s.db.NewSelect().TableExpr("pages AS p")
	applyAdminPageDBEnvironment(query, env)

	reqJoin, reqArgs := pageTranslationJoin("pt_req", primaryLocaleID, primaryLocaleCode)
	query.Join(reqJoin, reqArgs...)
	fbJoin, fbArgs := pageTranslationJoin("pt_fb", fallbackLocaleID, fallbackLocaleCode)
	query.Join(fbJoin, fbArgs...)

	query.
		Join("LEFT JOIN content_translations AS ct ON ct.content_id = p.content_id AND ct.locale_id = COALESCE(pt_req.locale_id, pt_fb.locale_id)").
		Join("LEFT JOIN contents AS c ON c.id = p.content_id").
		Join("LEFT JOIN content_types AS ctype ON ctype.id = c.content_type_id")

	expr := adminPageDBExpressions{
		title:              "COALESCE(pt_req.title, pt_fb.title, '')",
		path:               "COALESCE(pt_req.path, pt_fb.path, '')",
		summary:            "COALESCE(pt_req.summary, pt_fb.summary, ct.summary, '')",
		metaTitle:          "COALESCE(pt_req.seo_title, pt_fb.seo_title, '')",
		metaDescription:    "COALESCE(pt_req.seo_description, pt_fb.seo_description, '')",
		translationGroupID: "COALESCE(pt_req.translation_group_id, pt_fb.translation_group_id, ct.translation_group_id)",
		schemaVersion:      "COALESCE(ctype.schema_version, '')",
		resolvedLocale:     "CASE WHEN pt_req.id IS NOT NULL THEN ? WHEN pt_fb.id IS NOT NULL THEN ? ELSE '' END",
		resolvedLocaleArgs: []any{primaryLocaleCode, fallbackLocaleCode},
	}
	return query, expr, nil
}

func (s *adminPageDBReadService) countMissingTranslations(ctx context.Context, env adminPageEnvironmentFilter, primaryLocaleID uuid.UUID, primaryLocaleCode string, fallbackLocaleID uuid.UUID, fallbackLocaleCode string) (int, error) {
	query, _, err := s.newAdminPageDBQuery(ctx, env, primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)
	if err != nil {
		return 0, err
	}
	query.Where("pt_req.id IS NULL")
	if fallbackLocaleID != uuid.Nil || strings.TrimSpace(fallbackLocaleCode) != "" {
		query.Where("pt_fb.id IS NULL")
	}
	return query.Count(ctx)
}

func applyAdminPageDBEnvironment(query *bun.SelectQuery, env adminPageEnvironmentFilter) {
	if query == nil {
		return
	}
	trimmed := strings.TrimSpace(env.key)
	if env.id != uuid.Nil {
		query.Where("p.environment_id = ?", env.id)
		return
	}
	if trimmed == "" {
		query.Where("p.environment_id = (SELECT id FROM environments WHERE is_default = TRUE LIMIT 1)")
		return
	}
	if envID, err := uuid.Parse(trimmed); err == nil {
		query.Where("p.environment_id = ?", envID)
		return
	}
	query.Where("p.environment_id = (SELECT id FROM environments WHERE key = ? LIMIT 1)", trimmed)
}

func pageTranslationJoin(alias string, localeID uuid.UUID, localeCode string) (string, []any) {
	join := fmt.Sprintf("LEFT JOIN page_translations AS %s ON %s.page_id = p.id", alias, alias)
	if localeID != uuid.Nil {
		return join + fmt.Sprintf(" AND %s.locale_id = ?", alias), []any{localeID}
	}
	if strings.TrimSpace(localeCode) == "" {
		return join + " AND 1=0", nil
	}
	return join + fmt.Sprintf(" AND %s.locale_id = (SELECT id FROM locales WHERE LOWER(code) = LOWER(?) LIMIT 1)", alias), []any{localeCode}
}

func applyAdminPageDBFilters(query *bun.SelectQuery, expr adminPageDBExpressions, filters map[string]any, search string) bool {
	if query == nil {
		return false
	}
	if len(filters) > 0 {
		if !applyAdminPageDBFilterValues(query, "LOWER(p.status)", filters["status"]) {
			return false
		}
		if !applyAdminPageDBFilterValues(query, "LOWER("+expr.resolvedLocale+")", filters["locale"], expr.resolvedLocaleArgs...) {
			return false
		}
		if !applyAdminPageDBFilterValues(query, "LOWER(CAST(p.template_id AS TEXT))", filters["template_id"]) {
			return false
		}
		if !applyAdminPageDBFilterValues(query, "LOWER(CAST(p.content_id AS TEXT))", filters["content_id"]) {
			return false
		}
		if !applyAdminPageDBFilterValues(query, "LOWER(CAST(p.parent_id AS TEXT))", filters["parent_id"]) {
			return false
		}
	}

	term := strings.TrimSpace(search)
	if term == "" {
		return true
	}
	like := "%" + strings.ToLower(term) + "%"
	query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.
			Where("LOWER("+expr.title+") LIKE ?", like).
			WhereOr("LOWER(p.slug) LIKE ?", like).
			WhereOr("LOWER("+expr.path+") LIKE ?", like).
			WhereOr("LOWER("+expr.summary+") LIKE ?", like)
	})
	return true
}

func applyAdminPageDBFilterValues(query *bun.SelectQuery, field string, filter any, extraArgs ...any) bool {
	values, ok := normalizeAdminPageFilterValues(filter)
	if !ok {
		query.Where("1=0")
		return false
	}
	if len(values) == 0 {
		return true
	}
	args := append([]any{}, extraArgs...)
	args = append(args, bun.In(values))
	query.Where(field+" IN (?)", args...)
	return true
}

func normalizeAdminPageFilterValues(filter any) ([]string, bool) {
	if filter == nil {
		return nil, true
	}
	var values []string
	switch typed := filter.(type) {
	case string:
		if text := strings.TrimSpace(typed); text != "" {
			values = append(values, strings.ToLower(text))
		}
	case []string:
		if len(typed) == 0 {
			return nil, true
		}
		for _, entry := range typed {
			text := strings.TrimSpace(entry)
			values = append(values, strings.ToLower(text))
		}
	case []any:
		if len(typed) == 0 {
			return nil, true
		}
		hasString := false
		for _, entry := range typed {
			if text, ok := entry.(string); ok {
				trimmed := strings.TrimSpace(text)
				values = append(values, strings.ToLower(trimmed))
				hasString = true
			}
		}
		if !hasString {
			return nil, false
		}
	default:
		return nil, false
	}
	return values, true
}

func applyAdminPageDBSort(query *bun.SelectQuery, expr adminPageDBExpressions, sortBy string, desc bool) {
	if query == nil {
		return
	}
	key := strings.TrimSpace(strings.ToLower(sortBy))
	if key == "" {
		return
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	switch key {
	case "title":
		query.OrderExpr("LOWER(" + expr.title + ") " + dir)
	case "slug":
		query.OrderExpr("LOWER(p.slug) " + dir)
	case "path":
		query.OrderExpr("LOWER(" + expr.path + ") " + dir)
	case "status":
		query.OrderExpr("LOWER(p.status) " + dir)
	case "created_at":
		query.OrderExpr("p.created_at " + dir)
	case "updated_at":
		query.OrderExpr("p.updated_at " + dir)
	case "published_at":
		query.OrderExpr("p.published_at " + dir)
	}
}

func applyAdminPageDBPagination(query *bun.SelectQuery, pageNum, perPage int) {
	if query == nil || pageNum <= 0 || perPage <= 0 {
		return
	}
	offset := (pageNum - 1) * perPage
	query.Offset(offset).Limit(perPage)
}

func mapAdminPageDBRow(row adminPageDBRow, requestedLocale string) interfaces.AdminPageRecord {
	record := interfaces.AdminPageRecord{
		ID:              row.ID,
		ContentID:       row.ContentID,
		TemplateID:      row.TemplateID,
		Slug:            row.Slug,
		Status:          row.Status,
		ParentID:        row.ParentID,
		RequestedLocale: strings.TrimSpace(requestedLocale),
		ResolvedLocale:  strings.TrimSpace(row.ResolvedLocale),
		PublishedAt:     cloneTimePtr(row.PublishedAt),
		CreatedAt:       cloneTimePtr(row.CreatedAt),
		UpdatedAt:       cloneTimePtr(row.UpdatedAt),
	}

	record.TranslationGroupID = row.TranslationGroupID
	record.Title = row.Title
	record.Path = row.Path
	record.MetaTitle = strings.TrimSpace(row.MetaTitle)
	record.MetaDescription = strings.TrimSpace(row.MetaDescription)
	record.Summary = cloneStringPtr(row.Summary)
	record.Tags = extractTags(row.ContentPayload)

	record.SchemaVersion = strings.TrimSpace(row.SchemaVersion)
	if record.SchemaVersion == "" && row.ContentPayload != nil {
		if version, ok := row.ContentPayload[cmsschema.RootSchemaKey].(string); ok {
			record.SchemaVersion = strings.TrimSpace(version)
		}
	}

	return record
}

func dedupeAdminLocales(primaryID uuid.UUID, primaryCode string, fallbackID uuid.UUID, fallbackCode string) (uuid.UUID, string, uuid.UUID, string) {
	if primaryID != uuid.Nil && fallbackID == primaryID {
		return primaryID, primaryCode, uuid.Nil, ""
	}
	if primaryCode != "" && fallbackCode != "" && strings.EqualFold(primaryCode, fallbackCode) {
		return primaryID, primaryCode, uuid.Nil, ""
	}
	return primaryID, primaryCode, fallbackID, fallbackCode
}
