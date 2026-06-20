package content

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/pkg/interfaces"
	sharedi18n "github.com/goliatone/go-i18n"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
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

type adminContentDBFamilyPageRow struct {
	FamilyID            uuid.UUID `bun:"family_id"`
	FamilySortTitle     string    `bun:"family_sort_title"`
	FamilySortCreatedAt time.Time `bun:"family_sort_created_at"`
	FamilySortUpdatedAt time.Time `bun:"family_sort_updated_at"`
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
	applyAdminContentDBContentTypeScope(countQuery, opts.ContentTypeID, opts.ContentTypeSlug)
	if ok, err := applyAdminContentDBFilters(countQuery, expr, opts.Filters, opts.Search, adminContentDBDynamicFilterRowScope); err != nil {
		return nil, 0, err
	} else if !ok {
		return []interfaces.AdminContentRecord{}, 0, nil
	}
	total, err := countQuery.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	listQuery, expr := s.newAdminContentDBQuery(primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)
	applyAdminContentDBEnvironment(listQuery, envID)
	applyAdminContentDBContentTypeScope(listQuery, opts.ContentTypeID, opts.ContentTypeSlug)
	if ok, err := applyAdminContentDBFilters(listQuery, expr, opts.Filters, opts.Search, adminContentDBDynamicFilterRowScope); err != nil {
		return nil, 0, err
	} else if !ok {
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

func (s *adminContentDBReadService) ListFamilies(ctx context.Context, opts interfaces.AdminContentFamilyListOptions) (interfaces.AdminContentFamilyListResult, error) {
	result := interfaces.AdminContentFamilyListResult{
		Page:    normalizedAdminContentPage(opts.Page),
		PerPage: normalizedAdminContentPerPage(opts.PerPage),
	}
	if s == nil || s.base == nil || s.base.content == nil {
		return result, errors.New("content: admin read service requires content service")
	}
	if s.db == nil {
		return result, errors.New("content: admin read service requires database")
	}

	requestedLocale := sharedi18n.NormalizeLocale(opts.Locale)
	fallbackLocale := sharedi18n.NormalizeLocale(opts.FallbackLocale)
	includeData, includeMetadata, includeBlocks := resolveAdminContentIncludes(true, opts.IncludeData, opts.IncludeMetadata, opts.IncludeBlocks, opts.DefaultIncludes)

	envID, err := resolveAdminContentEnvironment(ctx, s.base.content, opts.EnvironmentKey)
	if err != nil {
		return result, err
	}
	primaryLocaleID, primaryLocaleCode, err := s.resolveAdminContentLocale(ctx, requestedLocale)
	if err != nil {
		return result, err
	}
	fallbackLocaleID, fallbackLocaleCode, err := s.resolveAdminContentLocale(ctx, fallbackLocale)
	if err != nil {
		return result, err
	}
	primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode = dedupeAdminContentLocales(primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)

	countQuery, expr := s.newAdminContentDBQuery(primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)
	applyAdminContentDBEnvironment(countQuery, envID)
	applyAdminContentDBContentTypeScope(countQuery, opts.ContentTypeID, opts.ContentTypeSlug)
	if ok, err := applyAdminContentDBFilters(countQuery, expr, opts.Filters, opts.Search, adminContentDBDynamicFilterFamilyScope); err != nil {
		return result, err
	} else if !ok {
		return result, nil
	}
	countQuery.Where(expr.familyID + " IS NOT NULL")
	var familyTotal int
	if err := countQuery.ColumnExpr("COUNT(DISTINCT "+expr.familyID+")").Scan(ctx, &familyTotal); err != nil {
		return result, err
	}
	result.FamilyTotal = familyTotal
	if familyTotal == 0 {
		return result, nil
	}

	contentCountQuery, expr := s.newAdminContentDBQuery(primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)
	applyAdminContentDBEnvironment(contentCountQuery, envID)
	applyAdminContentDBContentTypeScope(contentCountQuery, opts.ContentTypeID, opts.ContentTypeSlug)
	if ok, err := applyAdminContentDBFilters(contentCountQuery, expr, opts.Filters, opts.Search, adminContentDBDynamicFilterFamilyScope); err != nil {
		return result, err
	} else if ok {
		contentCountQuery.Where(expr.familyID + " IS NOT NULL")
		total, err := contentCountQuery.Count(ctx)
		if err != nil {
			return result, err
		}
		result.ContentTotal = total
	}

	pageQuery, expr := s.newAdminContentDBQuery(primaryLocaleID, primaryLocaleCode, fallbackLocaleID, fallbackLocaleCode)
	applyAdminContentDBEnvironment(pageQuery, envID)
	applyAdminContentDBContentTypeScope(pageQuery, opts.ContentTypeID, opts.ContentTypeSlug)
	if ok, err := applyAdminContentDBFilters(pageQuery, expr, opts.Filters, opts.Search, adminContentDBDynamicFilterFamilyScope); err != nil {
		return result, err
	} else if !ok {
		return result, nil
	}
	pageQuery.Where(expr.familyID + " IS NOT NULL")
	pageQuery.ColumnExpr(expr.familyID + " AS family_id")
	pageQuery.GroupExpr(expr.familyID)
	if err := applyAdminContentDBFamilySort(pageQuery, expr, opts.SortBy, opts.SortDesc); err != nil {
		return result, err
	}
	applyAdminContentDBPagination(pageQuery, result.Page, result.PerPage)

	var familyRows []adminContentDBFamilyPageRow
	if err := pageQuery.Scan(ctx, &familyRows); err != nil {
		return result, err
	}
	if len(familyRows) == 0 {
		return result, nil
	}

	familyIDs := make([]uuid.UUID, 0, len(familyRows))
	familyOrder := make([]string, 0, len(familyRows))
	for _, row := range familyRows {
		if row.FamilyID == uuid.Nil {
			continue
		}
		familyIDs = append(familyIDs, row.FamilyID)
		familyOrder = append(familyOrder, row.FamilyID.String())
	}
	if len(familyIDs) == 0 {
		return result, nil
	}

	variantRecords, err := s.loadAdminContentDBFamilyVariants(ctx, envID, opts.ContentTypeID, opts.ContentTypeSlug, familyIDs, requestedLocale, includeData, includeMetadata, includeBlocks, opts.AllowMissingTranslations)
	if err != nil {
		return result, err
	}
	byFamily := map[string][]interfaces.AdminContentRecord{}
	for _, record := range variantRecords {
		familyID := adminContentRecordFamilyID(record)
		if familyID == "" {
			continue
		}
		byFamily[familyID] = append(byFamily[familyID], record)
	}

	families := make([]interfaces.AdminContentFamilyRecord, 0, len(familyOrder))
	for _, familyID := range familyOrder {
		variants := byFamily[familyID]
		if len(variants) == 0 {
			continue
		}
		sortAdminContentFamilyVariants(variants, requestedLocale)
		parent := variants[0]
		families = append(families, interfaces.AdminContentFamilyRecord{
			FamilyID:         familyID,
			Title:            parent.Title,
			Slug:             parent.Slug,
			Locale:           parent.Locale,
			AvailableLocales: append([]string{}, parent.AvailableLocales...),
			ContentType:      parent.ContentType,
			ContentTypeSlug:  parent.ContentTypeSlug,
			Status:           parent.Status,
			Data:             parent.Data,
			Metadata:         parent.Metadata,
			Variants:         projectAdminContentRecords(variants, opts.Fields),
		})
	}
	result.Families = families
	return result, nil
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

func applyAdminContentDBContentTypeScope(query *bun.SelectQuery, contentTypeID string, contentTypeSlug string) {
	if query == nil {
		return
	}
	trimmedID := strings.TrimSpace(contentTypeID)
	if trimmedID != "" {
		if parsed, err := uuid.Parse(trimmedID); err == nil && parsed != uuid.Nil {
			query.Where("c.content_type_id = ?", parsed)
			return
		}
	}
	normalizedSlug := strings.ToLower(strings.TrimSpace(contentTypeSlug))
	if normalizedSlug == "" {
		return
	}
	query.Where("LOWER(COALESCE(ctype.slug, ctype.name, '')) = ?", normalizedSlug)
}

type adminContentDBFilterPredicate struct {
	SourceKey string
	Field     string
	Operator  string
	Values    []string
}

type adminContentDBFilterTarget struct {
	Expr string
	Args []any
}

type adminContentDBDynamicFilterScope int

const (
	adminContentDBDynamicFilterRowScope adminContentDBDynamicFilterScope = iota
	adminContentDBDynamicFilterFamilyScope
)

const adminContentDBLikeEscapeClause = " ESCAPE '\\'"

func applyAdminContentDBFilters(query *bun.SelectQuery, expr adminContentDBExpressions, filters map[string]any, search string, dynamicScope adminContentDBDynamicFilterScope) (bool, error) {
	if query == nil {
		return false, nil
	}
	predicates, ok, err := normalizeAdminContentDBFilterPredicates(filters)
	if err != nil {
		return false, err
	}
	if !ok {
		query.Where("1=0")
		return false, nil
	}
	for _, predicate := range predicates {
		applied, err := applyAdminContentDBFilterPredicate(query, expr, predicate, dynamicScope)
		if err != nil {
			return false, err
		}
		if !applied {
			query.Where("1=0")
			return false, nil
		}
	}

	term := strings.TrimSpace(search)
	if term == "" {
		return true, nil
	}
	like := adminContentDBContainsPattern(term)
	query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
		args := append([]any{}, expr.titleArgs...)
		return q.
			Where("LOWER("+expr.title+") LIKE ?"+adminContentDBLikeEscapeClause, append(args, like)...).
			WhereOr("LOWER(c.slug) LIKE ?"+adminContentDBLikeEscapeClause, like).
			WhereOr("LOWER("+expr.contentType+") LIKE ?"+adminContentDBLikeEscapeClause, like)
	})
	return true, nil
}

func normalizeAdminContentDBFilterPredicates(filters map[string]any) ([]adminContentDBFilterPredicate, bool, error) {
	if len(filters) == 0 {
		return nil, true, nil
	}
	keys := make([]string, 0, len(filters))
	for key := range filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]adminContentDBFilterPredicate, 0, len(keys))
	for _, key := range keys {
		field, operator := parseAdminContentDBFilterKey(key)
		if field == "" || field == "_search" || field == "environment" || isAdminContentDBListModeFilterField(field) {
			continue
		}
		if !isSupportedAdminContentDBFilterOperator(operator) {
			return nil, false, adminContentDBUnsupportedFilter("unsupported filter operator", map[string]any{
				"field":    field,
				"operator": operator,
				"key":      key,
			})
		}
		values, ok := normalizeAdminContentFilterValues(filters[key], func(value string) string {
			return strings.TrimSpace(value)
		})
		if !ok {
			return nil, false, nil
		}
		if len(values) == 0 {
			continue
		}
		out = append(out, adminContentDBFilterPredicate{
			SourceKey: key,
			Field:     field,
			Operator:  operator,
			Values:    values,
		})
	}
	return out, true, nil
}

func parseAdminContentDBFilterKey(key string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(key), "__", 2)
	field := strings.ToLower(strings.TrimSpace(parts[0]))
	operator := "eq"
	if len(parts) == 2 {
		if parsed := strings.ToLower(strings.TrimSpace(parts[1])); parsed != "" {
			operator = parsed
		}
	}
	return field, operator
}

func isSupportedAdminContentDBFilterOperator(operator string) bool {
	switch strings.ToLower(strings.TrimSpace(operator)) {
	case "", "eq", "in", "ilike":
		return true
	default:
		return false
	}
}

func applyAdminContentDBFilterPredicate(query *bun.SelectQuery, expr adminContentDBExpressions, predicate adminContentDBFilterPredicate, dynamicScope adminContentDBDynamicFilterScope) (bool, error) {
	target, dynamic, ok := resolveAdminContentDBFilterTarget(query, expr, predicate.Field, dynamicScope)
	if !ok {
		return false, adminContentDBUnsupportedFilter("unsupported filter field", map[string]any{
			"field":    predicate.Field,
			"operator": predicate.Operator,
			"key":      predicate.SourceKey,
		})
	}
	if dynamic {
		return applyAdminContentDBDynamicFilterPredicate(query, target, predicate), nil
	}
	return applyAdminContentDBTargetFilterPredicate(query, target, predicate), nil
}

func resolveAdminContentDBFilterTarget(query *bun.SelectQuery, expr adminContentDBExpressions, field string, dynamicScope adminContentDBDynamicFilterScope) (adminContentDBFilterTarget, bool, bool) {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "id":
		return adminContentDBFilterTarget{Expr: "CAST(c.id AS TEXT)"}, false, true
	case "status":
		return adminContentDBFilterTarget{Expr: "c.status"}, false, true
	case "locale", "resolved_locale":
		return adminContentDBFilterTarget{Expr: expr.resolvedLocale, Args: append([]any{}, expr.resolvedLocaleArgs...)}, false, true
	case "slug":
		return adminContentDBFilterTarget{Expr: "c.slug"}, false, true
	case "title":
		return adminContentDBFilterTarget{Expr: expr.title, Args: append([]any{}, expr.titleArgs...)}, false, true
	case "content_type", "content_type_slug":
		return adminContentDBFilterTarget{Expr: expr.contentType}, false, true
	case "family_id":
		return adminContentDBFilterTarget{Expr: "CAST(" + expr.familyID + " AS TEXT)"}, false, true
	case "created_at":
		return adminContentDBFilterTarget{Expr: "CAST(c.created_at AS TEXT)"}, false, true
	case "updated_at":
		return adminContentDBFilterTarget{Expr: "CAST(c.updated_at AS TEXT)"}, false, true
	case "published_at":
		return adminContentDBFilterTarget{Expr: "CAST(c.published_at AS TEXT)"}, false, true
	}
	if !isSafeAdminContentDBJSONKey(field) {
		return adminContentDBFilterTarget{}, false, false
	}
	if dynamicScope == adminContentDBDynamicFilterFamilyScope {
		return adminContentDBFilterTarget{Expr: adminContentDBJSONTextExpr(query, "ct_filter.content", field)}, true, true
	}
	source := "(COALESCE(ct_req.content, ct_fb.content, ct_primary.content, ct_first.content))"
	return adminContentDBFilterTarget{Expr: adminContentDBJSONTextExpr(query, source, field)}, false, true
}

func applyAdminContentDBTargetFilterPredicate(query *bun.SelectQuery, target adminContentDBFilterTarget, predicate adminContentDBFilterPredicate) bool {
	values := normalizeAdminContentDBPredicateValues(predicate.Values)
	if len(values) == 0 {
		return true
	}
	args := append([]any{}, target.Args...)
	switch strings.ToLower(strings.TrimSpace(predicate.Operator)) {
	case "in", "eq", "":
		args = append(args, bun.In(values))
		query.Where("LOWER(COALESCE("+target.Expr+", '')) IN (?)", args...)
	case "ilike":
		query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return applyAdminContentDBLikeGroup(q, "LOWER(COALESCE("+target.Expr+", ''))", target.Args, values)
		})
	}
	return true
}

func applyAdminContentDBDynamicFilterPredicate(query *bun.SelectQuery, target adminContentDBFilterTarget, predicate adminContentDBFilterPredicate) bool {
	values := normalizeAdminContentDBPredicateValues(predicate.Values)
	if len(values) == 0 {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(predicate.Operator)) {
	case "in", "eq", "":
		query.Where("EXISTS (SELECT 1 FROM content_translations AS ct_filter WHERE ct_filter.content_id = c.id AND LOWER(COALESCE("+target.Expr+", '')) IN (?))", bun.In(values))
	case "ilike":
		condition, args := adminContentDBLikeExistsCondition("LOWER(COALESCE("+target.Expr+", ''))", values)
		query.Where("EXISTS (SELECT 1 FROM content_translations AS ct_filter WHERE ct_filter.content_id = c.id AND "+condition+")", args...)
	}
	return true
}

func applyAdminContentDBLikeGroup(q *bun.SelectQuery, expr string, exprArgs []any, values []string) *bun.SelectQuery {
	for i, value := range values {
		args := append([]any{}, exprArgs...)
		args = append(args, adminContentDBContainsPattern(value))
		if i == 0 {
			q = q.Where(expr+" LIKE ?"+adminContentDBLikeEscapeClause, args...)
		} else {
			q = q.WhereOr(expr+" LIKE ?"+adminContentDBLikeEscapeClause, args...)
		}
	}
	return q
}

func adminContentDBLikeExistsCondition(expr string, values []string) (string, []any) {
	parts := make([]string, 0, len(values))
	args := make([]any, 0, len(values))
	for _, value := range values {
		parts = append(parts, expr+" LIKE ?"+adminContentDBLikeEscapeClause)
		args = append(args, adminContentDBContainsPattern(value))
	}
	if len(parts) == 0 {
		return "1=1", nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

func normalizeAdminContentDBPredicateValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		out = append(out, normalized)
	}
	return out
}

func adminContentDBContainsPattern(value string) string {
	return "%" + adminContentDBEscapeLikeValue(strings.ToLower(strings.TrimSpace(value))) + "%"
}

func adminContentDBEscapeLikeValue(value string) string {
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch r {
		case '\\', '%', '_':
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isAdminContentDBListModeFilterField(field string) bool {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "group_by", "groupby":
		return true
	default:
		return false
	}
}

func isSafeAdminContentDBJSONKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func adminContentDBJSONTextExpr(query *bun.SelectQuery, source string, key string) string {
	if query != nil && query.Dialect() != nil && query.Dialect().Name() == dialect.SQLite {
		return fmt.Sprintf("json_extract(%s, '$.%s')", source, key)
	}
	return fmt.Sprintf("%s->>'%s'", source, key)
}

func adminContentDBUnsupportedFilter(reason string, metadata map[string]any) error {
	return interfaces.AdminContentFamilyReadUnsupportedError{
		Reason:   strings.TrimSpace(reason),
		Metadata: metadata,
	}
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
		for _, part := range strings.Split(typed, ",") {
			if text := normalize(part); text != "" {
				values = append(values, text)
			}
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

func applyAdminContentDBFamilySort(query *bun.SelectQuery, expr adminContentDBExpressions, sortBy string, desc bool) error {
	if query == nil {
		return nil
	}
	key := strings.TrimSpace(strings.ToLower(sortBy))
	if key == "" {
		key = "title"
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	switch key {
	case "title":
		if len(expr.titleArgs) > 0 {
			query.ColumnExpr("MIN(LOWER("+expr.title+")) AS family_sort_title", expr.titleArgs...)
		} else {
			query.ColumnExpr("MIN(LOWER(" + expr.title + ")) AS family_sort_title")
		}
		query.OrderExpr("family_sort_title " + dir)
	case "created_at":
		query.ColumnExpr("MIN(c.created_at) AS family_sort_created_at")
		query.OrderExpr("family_sort_created_at " + dir)
	case "updated_at":
		query.ColumnExpr("MAX(c.updated_at) AS family_sort_updated_at")
		query.OrderExpr("family_sort_updated_at " + dir)
	default:
		return fmt.Errorf("content: unsupported grouped family sort %q", sortBy)
	}
	query.OrderExpr("family_id ASC")
	return nil
}

func applyAdminContentDBPagination(query *bun.SelectQuery, pageNum, perPage int) {
	if query == nil || pageNum <= 0 || perPage <= 0 {
		return
	}
	offset := (pageNum - 1) * perPage
	query.Offset(offset).Limit(perPage)
}

func (s *adminContentDBReadService) loadAdminContentDBFamilyVariants(ctx context.Context, envID uuid.UUID, contentTypeID string, contentTypeSlug string, familyIDs []uuid.UUID, requestedLocale string, includeData bool, includeMetadata bool, includeBlocks bool, allowMissing bool) ([]interfaces.AdminContentRecord, error) {
	if len(familyIDs) == 0 {
		return nil, nil
	}
	query := s.db.NewSelect().
		TableExpr("contents AS c").
		Join("JOIN content_translations AS ct ON ct.content_id = c.id").
		Join("LEFT JOIN content_types AS ctype ON ctype.id = c.content_type_id").
		Where("ct.family_id IN (?)", bun.In(familyIDs)).
		ColumnExpr("DISTINCT c.id AS id").
		OrderExpr("c.id ASC")
	applyAdminContentDBEnvironment(query, envID)
	applyAdminContentDBContentTypeScope(query, contentTypeID, contentTypeSlug)

	var rows []adminContentDBIDRow
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	selected := make(map[string]struct{}, len(familyIDs))
	for _, familyID := range familyIDs {
		if familyID != uuid.Nil {
			selected[familyID.String()] = struct{}{}
		}
	}
	records := make([]interfaces.AdminContentRecord, 0, len(rows))
	for _, row := range rows {
		record, err := s.base.content.Get(ctx, row.ID, WithTranslations(), WithProjection(ContentProjectionAdmin))
		if err != nil {
			return nil, err
		}
		if record == nil {
			continue
		}
		for _, translation := range record.Translations {
			if translation == nil || translation.FamilyID == nil || *translation.FamilyID == uuid.Nil {
				continue
			}
			if _, ok := selected[translation.FamilyID.String()]; !ok {
				continue
			}
			locale := s.base.localeCode(ctx, translation)
			if locale == "" {
				continue
			}
			item, err := s.base.buildRecord(ctx, record, locale, requestedLocale, allowMissing, includeData, includeMetadata, includeBlocks)
			if err != nil {
				return nil, err
			}
			records = append(records, item)
		}
	}
	return records, nil
}

func adminContentRecordFamilyID(record interfaces.AdminContentRecord) string {
	if record.FamilyID == nil || *record.FamilyID == uuid.Nil {
		return ""
	}
	return record.FamilyID.String()
}

func sortAdminContentFamilyVariants(records []interfaces.AdminContentRecord, requestedLocale string) {
	requestedLocale = sharedi18n.NormalizeLocale(requestedLocale)
	sort.SliceStable(records, func(i, j int) bool {
		leftLocale := sharedi18n.NormalizeLocale(records[i].Locale)
		rightLocale := sharedi18n.NormalizeLocale(records[j].Locale)
		if requestedLocale != "" {
			if leftLocale == requestedLocale && rightLocale != requestedLocale {
				return true
			}
			if rightLocale == requestedLocale && leftLocale != requestedLocale {
				return false
			}
		}
		if leftLocale != rightLocale {
			return leftLocale < rightLocale
		}
		return records[i].ID.String() < records[j].ID.String()
	})
}
