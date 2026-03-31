package content

import (
	"context"
	"errors"
	"fmt"
	"strings"

	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/pkg/interfaces"
	sharedi18n "github.com/goliatone/go-i18n"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// NewAdminContentDBReadService constructs the DB-backed admin content read service.
func NewAdminContentDBReadService(db *bun.DB, contentSvc Service, contentTypes ContentTypeService, locales LocaleRepository, _ any, opts ...AdminContentReadOption) interfaces.AdminContentReadService {
	base := &adminContentReadService{
		content:      contentSvc,
		contentTypes: contentTypes,
		locales:      locales,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(base)
		}
	}
	return &adminContentDBReadService{
		db:   db,
		base: base,
	}
}

type adminContentDBReadService struct {
	db   *bun.DB
	base *adminContentReadService
}

type adminContentDBIDRow struct {
	ID uuid.UUID `bun:"id"`
}

type adminContentDBExpressions struct {
	title              string
	titleArgs          []any
	resolvedLocale     string
	resolvedLocaleArgs []any
	familyID           string
	contentType        string
}

func (s *adminContentDBReadService) List(ctx context.Context, opts interfaces.AdminContentListOptions) ([]interfaces.AdminContentRecord, int, error) {
	if s == nil || s.base == nil || s.base.content == nil {
		return nil, 0, errors.New("content: admin read service requires content service")
	}
	if s.db == nil {
		return nil, 0, errors.New("content: admin read service requires database")
	}

	requestedLocale := sharedi18n.NormalizeLocale(opts.Locale)
	fallbackLocale := sharedi18n.NormalizeLocale(opts.FallbackLocale)
	includeData, includeMetadata, includeBlocks := resolveAdminContentIncludes(true, opts.IncludeData, opts.IncludeMetadata, opts.IncludeBlocks, opts.DefaultIncludes)

	envID, err := resolveAdminContentEnvironment(ctx, s.base.content, opts.EnvironmentKey)
	if err != nil {
		return nil, 0, err
	}
	primaryLocaleID, primaryLocaleCode, err := s.resolveAdminContentLocale(ctx, requestedLocale)
	if err != nil {
		return nil, 0, err
	}
	fallbackLocaleID, fallbackLocaleCode, err := s.resolveAdminContentLocale(ctx, fallbackLocale)
	if err != nil {
		return nil, 0, err
	}
	primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode = dedupeAdminContentLocales(primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)

	countQuery, expr := s.newAdminContentDBQuery(primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)
	applyAdminContentDBEnvironment(countQuery, envID)
	if !applyAdminContentDBFilters(countQuery, expr, opts.Filters, opts.Search) {
		return []interfaces.AdminContentRecord{}, 0, nil
	}
	total, err := countQuery.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	listQuery, expr := s.newAdminContentDBQuery(primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)
	applyAdminContentDBEnvironment(listQuery, envID)
	if !applyAdminContentDBFilters(listQuery, expr, opts.Filters, opts.Search) {
		return []interfaces.AdminContentRecord{}, 0, nil
	}
	applyAdminContentDBSort(listQuery, expr, opts.SortBy, opts.SortDesc)
	applyAdminContentDBPagination(listQuery, opts.Page, opts.PerPage)
	listQuery.ColumnExpr("c.id AS id")

	var rows []adminContentDBIDRow
	if err := listQuery.Scan(ctx, &rows); err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return []interfaces.AdminContentRecord{}, total, nil
	}

	records := make([]interfaces.AdminContentRecord, 0, len(rows))
	for _, row := range rows {
		record, err := s.base.content.Get(ctx, row.ID, WithTranslations(), WithProjection(ContentProjectionAdmin))
		if err != nil {
			return nil, 0, err
		}
		item, err := s.base.buildRecord(ctx, record, requestedLocale, fallbackLocale, opts.AllowMissingTranslations, includeData, includeMetadata, includeBlocks)
		if err != nil {
			return nil, 0, err
		}
		records = append(records, item)
	}

	return projectAdminContentRecords(records, opts.Fields), total, nil
}

func (s *adminContentDBReadService) Get(ctx context.Context, id string, opts interfaces.AdminContentGetOptions) (*interfaces.AdminContentRecord, error) {
	if s == nil || s.base == nil {
		return nil, errors.New("content: admin read service requires content service")
	}
	return s.base.Get(ctx, id, opts)
}

func (s *adminContentDBReadService) resolveAdminContentLocale(ctx context.Context, code string) (uuid.UUID, string, error) {
	normalized := sharedi18n.NormalizeLocale(code)
	if normalized == "" {
		return uuid.Nil, "", nil
	}
	if s == nil || s.base == nil || s.base.locales == nil {
		return uuid.Nil, normalized, nil
	}
	record, err := s.base.locales.GetByCode(ctx, normalized)
	if err != nil {
		return uuid.Nil, "", err
	}
	if record == nil {
		return uuid.Nil, normalized, nil
	}
	return record.ID, sharedi18n.NormalizeLocale(record.Code), nil
}

func dedupeAdminContentLocales(primaryID uuid.UUID, primaryCode string, fallbackID uuid.UUID, fallbackCode string) (uuid.UUID, string, uuid.UUID, string) {
	primaryCode = sharedi18n.NormalizeLocale(primaryCode)
	fallbackCode = sharedi18n.NormalizeLocale(fallbackCode)
	if primaryID != uuid.Nil && primaryID == fallbackID {
		fallbackID = uuid.Nil
		fallbackCode = ""
	}
	if primaryCode != "" && primaryCode == fallbackCode {
		fallbackID = uuid.Nil
		fallbackCode = ""
	}
	return primaryID, primaryCode, fallbackID, fallbackCode
}

func resolveAdminContentEnvironment(ctx context.Context, svc Service, envKey string) (uuid.UUID, error) {
	if impl, ok := svc.(*service); ok && impl != nil {
		envID, _, err := impl.resolveEnvironment(ctx, envKey)
		if err != nil {
			return uuid.Nil, err
		}
		return envID, nil
	}

	trimmed := strings.TrimSpace(envKey)
	if trimmed != "" {
		if parsed, err := uuid.Parse(trimmed); err == nil {
			return parsed, nil
		}
	}
	normalized, err := cmsenv.ResolveKey(trimmed, "", false)
	if err != nil {
		return uuid.Nil, err
	}
	return cmsenv.IDForKey(normalized), nil
}

func (s *adminContentDBReadService) newAdminContentDBQuery(primaryLocaleID uuid.UUID, primaryLocaleCode string, fallbackLocaleID uuid.UUID, fallbackLocaleCode string) (*bun.SelectQuery, adminContentDBExpressions) {
	query := s.db.NewSelect().
		TableExpr("contents AS c").
		Join("LEFT JOIN content_types AS ctype ON ctype.id = c.content_type_id")

	reqJoin, reqArgs := adminContentTranslationJoin("ct_req", primaryLocaleID, primaryLocaleCode)
	query.Join(reqJoin, reqArgs...)
	fbJoin, fbArgs := adminContentTranslationJoin("ct_fb", fallbackLocaleID, fallbackLocaleCode)
	query.Join(fbJoin, fbArgs...)
	query.
		Join("LEFT JOIN content_translations AS ct_primary ON ct_primary.id = (" +
			"SELECT ctp.id FROM content_translations AS ctp " +
			"LEFT JOIN locales AS lp ON lp.id = ctp.locale_id " +
			"WHERE ctp.content_id = c.id AND LOWER(COALESCE(lp.code, '')) = LOWER(COALESCE(c.primary_locale, '')) " +
			"ORDER BY ctp.created_at ASC, ctp.id ASC LIMIT 1)").
		Join("LEFT JOIN content_translations AS ct_first ON ct_first.id = (" +
			"SELECT ctf.id FROM content_translations AS ctf " +
			"LEFT JOIN locales AS lf0 ON lf0.id = ctf.locale_id " +
			"WHERE ctf.content_id = c.id " +
			"ORDER BY LOWER(COALESCE(lf0.code, '')) ASC, ctf.created_at ASC, ctf.id ASC LIMIT 1)").
		Join("LEFT JOIN locales AS l_first ON l_first.id = ct_first.locale_id")

	expr := adminContentDBExpressions{
		title:              "COALESCE(ct_req.title, ct_fb.title, ct_primary.title, ct_first.title, '')",
		titleArgs:          nil,
		resolvedLocale:     "CASE WHEN ct_req.id IS NOT NULL THEN ? WHEN ct_fb.id IS NOT NULL THEN ? WHEN ct_primary.id IS NOT NULL THEN LOWER(COALESCE(c.primary_locale, '')) WHEN ct_first.id IS NOT NULL THEN LOWER(COALESCE(l_first.code, '')) ELSE '' END",
		resolvedLocaleArgs: []any{primaryLocaleCode, fallbackLocaleCode},
		familyID:           "COALESCE(ct_req.family_id, ct_fb.family_id, ct_primary.family_id, ct_first.family_id)",
		contentType:        "COALESCE(ctype.slug, ctype.name, '')",
	}
	return query, expr
}

func adminContentTranslationJoin(alias string, localeID uuid.UUID, localeCode string) (string, []any) {
	join := fmt.Sprintf("LEFT JOIN content_translations AS %s ON %s.content_id = c.id", alias, alias)
	if localeID != uuid.Nil {
		return join + fmt.Sprintf(" AND %s.locale_id = ?", alias), []any{localeID}
	}
	localeCode = sharedi18n.NormalizeLocale(localeCode)
	if localeCode == "" {
		return join + " AND 1=0", nil
	}
	return join + fmt.Sprintf(" AND %s.locale_id = (SELECT id FROM locales WHERE LOWER(code) = LOWER(?) LIMIT 1)", alias), []any{localeCode}
}

func applyAdminContentDBEnvironment(query *bun.SelectQuery, envID uuid.UUID) {
	if query == nil {
		return
	}
	if envID == uuid.Nil {
		return
	}
	query.Where("c.environment_id = ?", envID)
}

func applyAdminContentDBFilters(query *bun.SelectQuery, expr adminContentDBExpressions, filters map[string]any, search string) bool {
	if query == nil {
		return false
	}
	if len(filters) > 0 {
		if !applyAdminContentDBFilterValues(query, "LOWER(c.status)", filters["status"]) {
			return false
		}
		if !applyAdminContentDBLocaleFilterValues(query, "LOWER("+expr.resolvedLocale+")", filters["locale"], expr.resolvedLocaleArgs...) {
			return false
		}
		if !applyAdminContentDBLocaleFilterValues(query, "LOWER("+expr.resolvedLocale+")", filters["resolved_locale"], expr.resolvedLocaleArgs...) {
			return false
		}
		if !applyAdminContentDBFilterValues(query, "LOWER(c.slug)", filters["slug"]) {
			return false
		}
		if !applyAdminContentDBFilterValues(query, "LOWER("+expr.title+")", filters["title"], expr.titleArgs...) {
			return false
		}
		if !applyAdminContentDBFilterValues(query, "LOWER("+expr.contentType+")", firstNonNil(filters["content_type"], filters["content_type_slug"])) {
			return false
		}
		if !applyAdminContentDBFilterValues(query, "LOWER(CAST("+expr.familyID+" AS TEXT))", filters["family_id"]) {
			return false
		}
	}

	term := strings.TrimSpace(search)
	if term == "" {
		return true
	}
	like := "%" + strings.ToLower(term) + "%"
	query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
		args := append([]any{}, expr.titleArgs...)
		return q.
			Where("LOWER("+expr.title+") LIKE ?", append(args, like)...).
			WhereOr("LOWER(c.slug) LIKE ?", like).
			WhereOr("LOWER("+expr.contentType+") LIKE ?", like)
	})
	return true
}

func applyAdminContentDBFilterValues(query *bun.SelectQuery, field string, filter any, extraArgs ...any) bool {
	values, ok := normalizeAdminContentFilterValues(filter, func(value string) string {
		return strings.ToLower(strings.TrimSpace(value))
	})
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

func applyAdminContentDBLocaleFilterValues(query *bun.SelectQuery, field string, filter any, extraArgs ...any) bool {
	values, ok := normalizeAdminContentFilterValues(filter, func(value string) string {
		normalized := sharedi18n.NormalizeLocale(value)
		if normalized == "" {
			return ""
		}
		return strings.ToLower(normalized)
	})
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

func normalizeAdminContentFilterValues(filter any, normalize func(string) string) ([]string, bool) {
	if filter == nil {
		return nil, true
	}
	if normalize == nil {
		normalize = func(value string) string {
			return strings.TrimSpace(value)
		}
	}
	var values []string
	switch typed := filter.(type) {
	case string:
		if text := normalize(typed); text != "" {
			values = append(values, text)
		}
	case []string:
		if len(typed) == 0 {
			return nil, true
		}
		for _, entry := range typed {
			if text := normalize(entry); text != "" {
				values = append(values, text)
			}
		}
	case []any:
		if len(typed) == 0 {
			return nil, true
		}
		hasString := false
		for _, entry := range typed {
			if text, ok := entry.(string); ok {
				if normalized := normalize(text); normalized != "" {
					values = append(values, normalized)
				}
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

func applyAdminContentDBSort(query *bun.SelectQuery, expr adminContentDBExpressions, sortBy string, desc bool) {
	if query == nil {
		return
	}
	key := strings.TrimSpace(strings.ToLower(sortBy))
	if key == "" {
		key = "id"
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	switch key {
	case "title":
		query.OrderExpr("LOWER(" + expr.title + ") " + dir)
	case "slug":
		query.OrderExpr("LOWER(c.slug) " + dir)
	case "locale", "resolved_locale":
		query.OrderExpr("LOWER("+expr.resolvedLocale+") "+dir, expr.resolvedLocaleArgs...)
	case "content_type", "content_type_slug":
		query.OrderExpr("LOWER(" + expr.contentType + ") " + dir)
	case "status":
		query.OrderExpr("LOWER(c.status) " + dir)
	case "created_at":
		query.OrderExpr("c.created_at " + dir)
	case "updated_at":
		query.OrderExpr("c.updated_at " + dir)
	case "published_at":
		query.OrderExpr("c.published_at " + dir)
	default:
		query.OrderExpr("c.id " + dir)
	}
	query.OrderExpr("c.id ASC")
}

func applyAdminContentDBPagination(query *bun.SelectQuery, pageNum, perPage int) {
	if query == nil || pageNum <= 0 || perPage <= 0 {
		return
	}
	offset := (pageNum - 1) * perPage
	query.Offset(offset).Limit(perPage)
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
