package generator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	gotheme "github.com/goliatone/go-theme"
	"github.com/google/uuid"
)

var (
	errPagesServiceRequired   = errors.New("generator: pages service is required")
	errContentServiceRequired = errors.New("generator: content service is required")
	errLocaleLookupRequired   = errors.New("generator: locale lookup is required")
)

// BuildContext aggregates the localized page data required to execute a static build.
type BuildContext struct {
	GeneratedAt   time.Time
	DefaultLocale string
	Locales       []LocaleSpec
	Pages         []*PageData
	MenuAliases   map[string]string
	Options       BuildOptions
}

// LocaleSpec captures resolved locale information for a build.
type LocaleSpec struct {
	Code      string
	LocaleID  uuid.UUID
	IsDefault bool
}

// PageData encapsulates resolved dependencies for a page/locale combination.
type PageData struct {
	Page               *pages.Page
	Content            *content.Content
	Locale             LocaleSpec
	Translation        *pages.PageTranslation
	ContentTranslation *content.ContentTranslation
	Blocks             []*blocks.Instance
	Widgets            map[string][]*widgets.ResolvedWidget
	Menus              map[string][]menus.NavigationNode
	Template           *themes.Template
	Theme              *themes.Theme
	ThemeSelection     *gotheme.Selection
	Metadata           DependencyMetadata
}

// DependencyMetadata tracks hashes and timestamps for incremental builds.
type DependencyMetadata struct {
	Sources      map[string]string
	Hash         string
	LastModified time.Time
}

func (s *service) loadContext(ctx context.Context, opts BuildOptions) (*BuildContext, error) {
	if s.deps.Pages == nil {
		return nil, errPagesServiceRequired
	}
	if s.deps.Content == nil {
		return nil, errContentServiceRequired
	}
	if s.deps.Locales == nil {
		return nil, errLocaleLookupRequired
	}

	localeSet, err := s.resolveLocales(ctx, opts)
	if err != nil {
		return nil, err
	}

	pagesToBuild, err := s.loadPages(ctx, opts.PageIDs)
	if err != nil {
		return nil, err
	}

	caches := newBuildCaches(s.cfg.Menus)
	var pageContexts []*PageData

	for _, page := range pagesToBuild {
		if page == nil || !page.IsVisible {
			continue
		}
		contentRecord, err := s.deps.Content.Get(ctx, page.ContentID)
		if err != nil {
			return nil, err
		}

		localized, err := s.buildPageData(ctx, page, contentRecord, localeSet, caches)
		if err != nil {
			return nil, err
		}
		pageContexts = append(pageContexts, localized...)
	}

	context := &BuildContext{
		GeneratedAt:   s.now(),
		DefaultLocale: localeSet.defaultCode,
		Locales:       localeSet.ordered,
		Pages:         pageContexts,
		MenuAliases:   maps.Clone(s.cfg.Menus),
		Options:       opts,
	}
	return context, nil
}

type localeSet struct {
	ordered     []LocaleSpec
	byID        map[uuid.UUID]LocaleSpec
	defaultCode string
	defaultID   uuid.UUID
}

func (s *service) resolveLocales(ctx context.Context, opts BuildOptions) (localeSet, error) {
	defaultLocale := strings.TrimSpace(s.cfg.DefaultLocale)
	if defaultLocale == "" && s.deps.I18N != nil {
		defaultLocale = strings.TrimSpace(s.deps.I18N.DefaultLocale())
	}
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	requestedFromOpts := len(opts.Locales) > 0
	var baseRequested []string
	if requestedFromOpts {
		baseRequested = append([]string{}, opts.Locales...)
	} else if len(s.cfg.Locales) > 0 {
		baseRequested = append([]string{}, s.cfg.Locales...)
	}

	seen := map[string]struct{}{}
	var codes []string

	includeDefault := !requestedFromOpts

	if includeDefault {
		defaultLower := strings.ToLower(defaultLocale)
		if strings.TrimSpace(defaultLocale) != "" {
			seen[defaultLower] = struct{}{}
			codes = append(codes, defaultLocale)
		}
	}

	for _, candidate := range baseRequested {
		normalized := strings.TrimSpace(candidate)
		if normalized == "" {
			continue
		}
		lower := strings.ToLower(normalized)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		codes = append(codes, normalized)
	}

	if len(codes) == 0 {
		codes = append(codes, defaultLocale)
	}

	set := localeSet{
		byID:        make(map[uuid.UUID]LocaleSpec, len(codes)),
		defaultCode: defaultLocale,
	}

	for _, code := range codes {
		record, err := s.deps.Locales.GetByCode(ctx, code)
		if err != nil {
			return localeSet{}, err
		}
		spec := LocaleSpec{
			Code:      record.Code,
			LocaleID:  record.ID,
			IsDefault: strings.EqualFold(record.Code, defaultLocale),
		}
		if spec.IsDefault {
			set.defaultID = record.ID
		}
		set.ordered = append(set.ordered, spec)
		set.byID[record.ID] = spec
	}

	if set.defaultID == uuid.Nil {
		if !includeDefault {
			record, err := s.deps.Locales.GetByCode(ctx, defaultLocale)
			if err != nil {
				return localeSet{}, err
			}
			set.defaultID = record.ID
			return set, nil
		}
		record, err := s.deps.Locales.GetByCode(ctx, defaultLocale)
		if err != nil {
			return localeSet{}, err
		}
		set.defaultID = record.ID
		defaultSpec := LocaleSpec{
			Code:      record.Code,
			LocaleID:  record.ID,
			IsDefault: true,
		}
		if _, ok := set.byID[record.ID]; !ok {
			set.byID[record.ID] = defaultSpec
			set.ordered = append([]LocaleSpec{defaultSpec}, set.ordered...)
		} else if len(set.ordered) > 0 && set.ordered[0].LocaleID != record.ID {
			set.ordered = reorderWithDefaultFirst(set.ordered, record.ID)
		}
	} else if includeDefault && len(set.ordered) > 0 && set.ordered[0].LocaleID != set.defaultID {
		set.ordered = reorderWithDefaultFirst(set.ordered, set.defaultID)
	}

	return set, nil
}

func reorderWithDefaultFirst(locales []LocaleSpec, defaultID uuid.UUID) []LocaleSpec {
	index := -1
	for i, spec := range locales {
		if spec.LocaleID == defaultID {
			index = i
			break
		}
	}
	if index <= 0 {
		return locales
	}
	defaultSpec := locales[index]
	remaining := append([]LocaleSpec{}, locales[:index]...)
	remaining = append(remaining, locales[index+1:]...)
	result := make([]LocaleSpec, 0, len(locales))
	result = append(result, defaultSpec)
	result = append(result, remaining...)
	return result
}

func (s *service) loadPages(ctx context.Context, ids []uuid.UUID) ([]*pages.Page, error) {
	if len(ids) == 0 {
		return s.deps.Pages.List(ctx)
	}

	unique := make(map[uuid.UUID]struct{}, len(ids))
	var result []*pages.Page
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, seen := unique[id]; seen {
			continue
		}
		unique[id] = struct{}{}
		record, err := s.deps.Pages.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if record != nil {
			result = append(result, record)
		}
	}
	return result, nil
}

func (s *service) buildPageData(
	ctx context.Context,
	page *pages.Page,
	contentRecord *content.Content,
	locales localeSet,
	caches *buildCaches,
) ([]*PageData, error) {
	if page == nil || contentRecord == nil {
		return nil, nil
	}

	template, err := caches.template(ctx, s.deps.Themes, page.TemplateID)
	if err != nil {
		return nil, err
	}

	var theme *themes.Theme
	if template != nil && template.ThemeID != uuid.Nil {
		theme, err = caches.theme(ctx, s.deps.Themes, template.ThemeID)
		if err != nil {
			return nil, err
		}
	}

	var selection *gotheme.Selection
	if theme != nil && s.themeSelector != nil {
		selection, err = s.themeSelector.Selection(theme, s.cfg.Theming.DefaultVariant)
		if err != nil {
			return nil, err
		}
	}

	pageTranslations := indexPageTranslations(page.Translations)
	contentTranslations := indexContentTranslations(contentRecord.Translations)

	var localized []*PageData
	for localeID, translation := range pageTranslations {
		localeSpec, ok := locales.byID[localeID]
		if !ok {
			continue
		}
		if strings.TrimSpace(translation.Path) == "" {
			continue
		}

		contentTranslation := contentTranslations[localeID]
		if contentTranslation == nil && locales.defaultID != uuid.Nil {
			contentTranslation = contentTranslations[locales.defaultID]
		}
		if contentTranslation == nil {
			// Without content translation, generating the page is risky; skip the locale.
			continue
		}

		menuSet, err := caches.menus.resolveAll(ctx, s.deps.Menus, localeSpec.Code)
		if err != nil {
			return nil, err
		}

		metadata := computeDependencyMetadata(page, translation, contentRecord, contentTranslation, menuSet, template, theme)

		// Page and content structures are already enriched by the services; reuse them directly.
		localized = append(localized, &PageData{
			Page:               page,
			Content:            contentRecord,
			Locale:             localeSpec,
			Translation:        translation,
			ContentTranslation: contentTranslation,
			Blocks:             page.Blocks,
			Widgets:            page.Widgets,
			Menus:              menuSet,
			Template:           template,
			Theme:              theme,
			ThemeSelection:     selection,
			Metadata:           metadata,
		})
	}

	return localized, nil
}

type buildCaches struct {
	templates map[uuid.UUID]*themes.Template
	themes    map[uuid.UUID]*themes.Theme
	menus     *menuCache
}

func newBuildCaches(menuAliases map[string]string) *buildCaches {
	return &buildCaches{
		templates: map[uuid.UUID]*themes.Template{},
		themes:    map[uuid.UUID]*themes.Theme{},
		menus:     newMenuCache(menuAliases),
	}
}

func (c *buildCaches) template(ctx context.Context, service themes.Service, id uuid.UUID) (*themes.Template, error) {
	if id == uuid.Nil || service == nil {
		return nil, nil
	}
	if tpl, ok := c.templates[id]; ok {
		return tpl, nil
	}
	template, err := service.GetTemplate(ctx, id)
	if err != nil {
		if errors.Is(err, themes.ErrFeatureDisabled) || errors.Is(err, themes.ErrTemplateNotFound) {
			c.templates[id] = nil
			return nil, nil
		}
		return nil, err
	}
	c.templates[id] = template
	return template, nil
}

func (c *buildCaches) theme(ctx context.Context, service themes.Service, id uuid.UUID) (*themes.Theme, error) {
	if id == uuid.Nil || service == nil {
		return nil, nil
	}
	if theme, ok := c.themes[id]; ok {
		return theme, nil
	}
	record, err := service.GetTheme(ctx, id)
	if err != nil {
		if errors.Is(err, themes.ErrFeatureDisabled) || errors.Is(err, themes.ErrThemeNotFound) {
			c.themes[id] = nil
			return nil, nil
		}
		return nil, err
	}
	c.themes[id] = record
	return record, nil
}

type menuCache struct {
	aliases map[string]string
	data    map[string]map[string][]menus.NavigationNode
	mu      sync.Mutex
}

func newMenuCache(aliases map[string]string) *menuCache {
	if len(aliases) == 0 {
		return &menuCache{
			aliases: map[string]string{},
			data:    map[string]map[string][]menus.NavigationNode{},
		}
	}
	clean := make(map[string]string, len(aliases))
	for alias, code := range aliases {
		trimmedAlias := strings.TrimSpace(alias)
		trimmedCode := strings.TrimSpace(code)
		if trimmedAlias == "" || trimmedCode == "" {
			continue
		}
		clean[trimmedAlias] = trimmedCode
	}
	return &menuCache{
		aliases: clean,
		data:    map[string]map[string][]menus.NavigationNode{},
	}
}

func (c *menuCache) resolveAll(ctx context.Context, service menus.Service, locale string) (map[string][]menus.NavigationNode, error) {
	if len(c.aliases) == 0 || service == nil {
		return nil, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if localized, ok := c.data[locale]; ok {
		return cloneMenus(localized), nil
	}

	localized := make(map[string][]menus.NavigationNode, len(c.aliases))
	for alias, code := range c.aliases {
		nodes, err := service.ResolveNavigation(ctx, code, locale)
		if err != nil {
			if errors.Is(err, menus.ErrMenuNotFound) {
				localized[alias] = nil
				continue
			}
			return nil, err
		}
		localized[alias] = cloneNavigationNodes(nodes)
	}
	c.data[locale] = localized
	return cloneMenus(localized), nil
}

func cloneMenus(input map[string][]menus.NavigationNode) map[string][]menus.NavigationNode {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string][]menus.NavigationNode, len(input))
	for alias, nodes := range input {
		cloned[alias] = cloneNavigationNodes(nodes)
	}
	return cloned
}

func cloneNavigationNodes(nodes []menus.NavigationNode) []menus.NavigationNode {
	if len(nodes) == 0 {
		return nil
	}
	cloned := make([]menus.NavigationNode, len(nodes))
	for i, node := range nodes {
		cloned[i] = menus.NavigationNode{
			ID:       node.ID,
			Position: node.Position,
			Label:    node.Label,
			URL:      node.URL,
			Target:   maps.Clone(node.Target),
			Children: cloneNavigationNodes(node.Children),
		}
	}
	return cloned
}

func indexPageTranslations(translations []*pages.PageTranslation) map[uuid.UUID]*pages.PageTranslation {
	result := make(map[uuid.UUID]*pages.PageTranslation, len(translations))
	for _, translation := range translations {
		if translation == nil {
			continue
		}
		result[translation.LocaleID] = translation
	}
	return result
}

func indexContentTranslations(translations []*content.ContentTranslation) map[uuid.UUID]*content.ContentTranslation {
	result := make(map[uuid.UUID]*content.ContentTranslation, len(translations))
	for _, translation := range translations {
		if translation == nil {
			continue
		}
		result[translation.LocaleID] = translation
	}
	return result
}

func computeDependencyMetadata(
	page *pages.Page,
	pageTranslation *pages.PageTranslation,
	contentRecord *content.Content,
	contentTranslation *content.ContentTranslation,
	menus map[string][]menus.NavigationNode,
	template *themes.Template,
	theme *themes.Theme,
) DependencyMetadata {
	sources := map[string]string{
		"page": joinParts(
			page.ID.String(),
			page.Slug,
			page.Status,
			page.UpdatedAt.UTC().Format(time.RFC3339Nano),
			intPointerValue(page.PublishedVersion),
		),
		"page_translation": joinParts(
			pageTranslation.ID.String(),
			pageTranslation.Path,
			pageTranslation.Title,
			pageTranslation.UpdatedAt.UTC().Format(time.RFC3339Nano),
		),
		"content": joinParts(
			contentRecord.ID.String(),
			contentRecord.Slug,
			contentRecord.Status,
			contentRecord.UpdatedAt.UTC().Format(time.RFC3339Nano),
			intPointerValue(contentRecord.PublishedVersion),
		),
	}

	if contentTranslation != nil {
		sources["content_translation"] = joinParts(
			contentTranslation.ID.String(),
			contentTranslation.Title,
			contentTranslation.UpdatedAt.UTC().Format(time.RFC3339Nano),
			hashMap(contentTranslation.Content),
		)
	}

	if template != nil {
		sources["template"] = joinParts(template.ID.String(), template.Name, template.UpdatedAt.UTC().Format(time.RFC3339Nano))
	}
	if theme != nil {
		sources["theme"] = joinParts(theme.ID.String(), theme.Name, theme.Version)
	}

	if len(page.Blocks) > 0 {
		sources["blocks"] = hashBlocks(page.Blocks)
	}
	if len(page.Widgets) > 0 {
		sources["widgets"] = hashWidgets(page.Widgets)
	}
	if len(menus) > 0 {
		sources["menus"] = hashMenus(menus)
	}

	hash := hashSources(sources)
	lastModified := maxTime(
		page.UpdatedAt,
		pageTranslation.UpdatedAt,
		contentRecord.UpdatedAt,
		translationUpdatedAt(contentTranslation),
	)

	return DependencyMetadata{
		Sources:      sources,
		Hash:         hash,
		LastModified: lastModified,
	}
}

func joinParts(parts ...string) string {
	return strings.Join(parts, "|")
}

func intPointerValue(value *int) string {
	if value == nil {
		return "nil"
	}
	return strconvIt(*value)
}

func translationUpdatedAt(tr *content.ContentTranslation) time.Time {
	if tr == nil {
		return time.Time{}
	}
	return tr.UpdatedAt
}

func hashMap(input map[string]any) string {
	if len(input) == 0 {
		return ""
	}
	normalized := normalizeMap(input)
	bytes, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func normalizeMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make(map[string]any, len(input))
	for _, key := range keys {
		result[key] = normalizeValue(input[key])
	}
	return result
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return normalizeMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, elem := range typed {
			out[i] = normalizeValue(elem)
		}
		return out
	default:
		return typed
	}
}

func hashBlocks(instances []*blocks.Instance) string {
	if len(instances) == 0 {
		return ""
	}
	values := make([]string, 0, len(instances))
	for _, instance := range instances {
		if instance == nil {
			continue
		}
		values = append(values, joinParts(
			instance.ID.String(),
			instance.Region,
			strconvIt(instance.Position),
			instance.UpdatedAt.UTC().Format(time.RFC3339Nano),
			strconvIt(instance.CurrentVersion),
			intPointerValue(instance.PublishedVersion),
		))
	}
	sort.Strings(values)
	return hashStrings(values)
}

func hashWidgets(widgets map[string][]*widgets.ResolvedWidget) string {
	if len(widgets) == 0 {
		return ""
	}
	var values []string
	for area, entries := range widgets {
		for _, entry := range entries {
			if entry == nil || entry.Instance == nil || entry.Placement == nil {
				continue
			}
			values = append(values, joinParts(
				area,
				entry.Instance.ID.String(),
				entry.Instance.UpdatedAt.UTC().Format(time.RFC3339Nano),
				entry.Placement.ID.String(),
				entry.Placement.UpdatedAt.UTC().Format(time.RFC3339Nano),
				strconvIt(entry.Placement.Position),
			))
		}
	}
	sort.Strings(values)
	return hashStrings(values)
}

func hashMenus(menus map[string][]menus.NavigationNode) string {
	if len(menus) == 0 {
		return ""
	}
	entries := make([]string, 0, len(menus))
	for alias, nodes := range menus {
		entries = append(entries, joinParts(alias, hashNavigationNodes(nodes)))
	}
	sort.Strings(entries)
	return hashStrings(entries)
}

func hashNavigationNodes(nodes []menus.NavigationNode) string {
	if len(nodes) == 0 {
		return ""
	}
	values := make([]string, 0, len(nodes))
	for _, node := range nodes {
		values = append(values, joinParts(
			node.ID.String(),
			node.Label,
			node.URL,
			hashNavigationNodes(node.Children),
		))
	}
	sort.Strings(values)
	return hashStrings(values)
}

func hashStrings(values []string) string {
	if len(values) == 0 {
		return ""
	}
	hasher := sha256.New()
	for _, value := range values {
		hasher.Write([]byte(value))
		hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func hashSources(sources map[string]string) string {
	if len(sources) == 0 {
		return ""
	}
	keys := make([]string, 0, len(sources))
	for key := range sources {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	hasher := sha256.New()
	for _, key := range keys {
		hasher.Write([]byte(key))
		hasher.Write([]byte("="))
		hasher.Write([]byte(sources[key]))
		hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func maxTime(times ...time.Time) time.Time {
	var max time.Time
	for _, t := range times {
		if t.After(max) {
			max = t
		}
	}
	return max
}

func strconvIt(value int) string {
	return strconv.FormatInt(int64(value), 10)
}
