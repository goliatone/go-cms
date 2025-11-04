package menus

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/google/uuid"
)

// Service describes menu management capabilities.
type Service interface {
	CreateMenu(ctx context.Context, input CreateMenuInput) (*Menu, error)
	GetMenu(ctx context.Context, id uuid.UUID) (*Menu, error)
	GetMenuByCode(ctx context.Context, code string) (*Menu, error)

	AddMenuItem(ctx context.Context, input AddMenuItemInput) (*MenuItem, error)
	UpdateMenuItem(ctx context.Context, input UpdateMenuItemInput) (*MenuItem, error)
	ReorderMenuItems(ctx context.Context, input ReorderMenuItemsInput) ([]*MenuItem, error)

	AddMenuItemTranslation(ctx context.Context, input AddMenuItemTranslationInput) (*MenuItemTranslation, error)
	ResolveNavigation(ctx context.Context, menuCode string, locale string) ([]NavigationNode, error)
	InvalidateCache(ctx context.Context) error
}

// CreateMenuInput captures the information required to register a menu.
type CreateMenuInput struct {
	Code        string
	Description *string
	CreatedBy   uuid.UUID
	UpdatedBy   uuid.UUID
}

// AddMenuItemInput captures the data required to register a new menu item.
type AddMenuItemInput struct {
	MenuID    uuid.UUID
	ParentID  *uuid.UUID
	Position  int
	Target    map[string]any
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID

	Translations             []MenuItemTranslationInput
	AllowMissingTranslations bool
}

// UpdateMenuItemInput captures mutable fields for a menu item.
type UpdateMenuItemInput struct {
	ItemID    uuid.UUID
	Target    map[string]any
	Position  *int
	ParentID  *uuid.UUID
	UpdatedBy uuid.UUID
}

// ReorderMenuItemsInput defines a new hierarchical ordering for menu items.
type ReorderMenuItemsInput struct {
	MenuID uuid.UUID
	Items  []ItemOrder
}

// ItemOrder describes the desired parent/position for a menu item.
type ItemOrder struct {
	ItemID   uuid.UUID
	ParentID *uuid.UUID
	Position int
}

// MenuItemTranslationInput describes localized metadata for a menu item.
type MenuItemTranslationInput struct {
	Locale      string
	Label       string
	URLOverride *string
}

// AddMenuItemTranslationInput adds or updates localized metadata for an item.
type AddMenuItemTranslationInput struct {
	ItemID      uuid.UUID
	Locale      string
	Label       string
	URLOverride *string
}

var (
	ErrMenuCodeRequired         = errors.New("menus: code is required")
	ErrMenuCodeInvalid          = errors.New("menus: code must contain only letters, numbers, hyphen, or underscore")
	ErrMenuCodeExists           = errors.New("menus: code already exists")
	ErrMenuNotFound             = errors.New("menus: menu not found")
	ErrMenuItemNotFound         = errors.New("menus: menu item not found")
	ErrMenuItemParentInvalid    = errors.New("menus: parent menu item invalid")
	ErrMenuItemCycle            = errors.New("menus: hierarchy creates a cycle")
	ErrMenuItemPosition         = errors.New("menus: position must be zero or positive")
	ErrMenuItemTargetMissing    = errors.New("menus: target type is required")
	ErrMenuItemTranslations     = errors.New("menus: at least one translation is required")
	ErrMenuItemDuplicateLocale  = errors.New("menus: duplicate translation locale provided")
	ErrUnknownLocale            = errors.New("menus: locale is unknown")
	ErrTranslationExists        = errors.New("menus: translation already exists for locale")
	ErrTranslationLabelRequired = errors.New("menus: translation label is required")
	ErrMenuItemPageNotFound     = errors.New("menus: page target not found")
	ErrMenuItemPageSlugRequired = errors.New("menus: page target requires slug")
)

// LocaleRepository resolves locales by code.
type LocaleRepository interface {
	GetByCode(ctx context.Context, code string) (*content.Locale, error)
}

// PageRepository looks up pages for menu target validation and navigation resolution.
type PageRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*pages.Page, error)
	GetBySlug(ctx context.Context, slug string) (*pages.Page, error)
}

// NavigationNode represents a localized menu item ready for presentation.
type NavigationNode struct {
	ID       uuid.UUID        `json:"id"`
	Label    string           `json:"label"`
	URL      string           `json:"url"`
	Target   map[string]any   `json:"target,omitempty"`
	Children []NavigationNode `json:"children,omitempty"`
}

// IDGenerator produces unique identifiers.
type IDGenerator func() uuid.UUID

// ServiceOption configures menu service behaviour.
type ServiceOption func(*service)

// WithClock overrides the internal time source.
func WithClock(clock func() time.Time) ServiceOption {
	return func(s *service) {
		if clock != nil {
			s.now = clock
		}
	}
}

// WithIDGenerator overrides the ID generator.
func WithIDGenerator(generator IDGenerator) ServiceOption {
	return func(s *service) {
		if generator != nil {
			s.id = generator
		}
	}
}

// WithPageRepository wires the page repository used for target validation and URL resolution.
func WithPageRepository(repo PageRepository) ServiceOption {
	return func(s *service) {
		s.pageRepo = repo
	}
}

// WithRequireTranslations controls whether menu items must include translations.
func WithRequireTranslations(required bool) ServiceOption {
	return func(s *service) {
		s.requireTranslations = required
	}
}

// WithTranslationsEnabled toggles translation handling.
func WithTranslationsEnabled(enabled bool) ServiceOption {
	return func(s *service) {
		s.translationsEnabled = enabled
	}
}

type service struct {
	menus               MenuRepository
	items               MenuItemRepository
	translations        MenuItemTranslationRepository
	locales             LocaleRepository
	pageRepo            PageRepository
	now                 func() time.Time
	id                  IDGenerator
	urlResolver         URLResolver
	requireTranslations bool
	translationsEnabled bool
}

type cacheInvalidator interface {
	InvalidateCache(ctx context.Context) error
}

// NewService constructs a menu service instance.
func NewService(menuRepo MenuRepository, itemRepo MenuItemRepository, trRepo MenuItemTranslationRepository, localeRepo LocaleRepository, opts ...ServiceOption) Service {
	s := &service{
		menus:               menuRepo,
		items:               itemRepo,
		translations:        trRepo,
		locales:             localeRepo,
		now:                 time.Now,
		id:                  uuid.New,
		requireTranslations: true,
		translationsEnabled: true,
	}

	s.urlResolver = &defaultURLResolver{service: s}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *service) translationsRequired() bool {
	return s.translationsEnabled && s.requireTranslations
}

// CreateMenu registers a new menu ensuring code uniqueness.
func (s *service) CreateMenu(ctx context.Context, input CreateMenuInput) (*Menu, error) {
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return nil, ErrMenuCodeRequired
	}
	if !isValidCode(code) {
		return nil, ErrMenuCodeInvalid
	}

	if _, err := s.menus.GetByCode(ctx, code); err == nil {
		return nil, ErrMenuCodeExists
	} else if err != nil {
		var notFound *NotFoundError
		if !errors.As(err, &notFound) {
			return nil, err
		}
	}

	now := s.now()
	menu := &Menu{
		ID:          s.id(),
		Code:        code,
		Description: input.Description,
		CreatedBy:   input.CreatedBy,
		UpdatedBy:   input.UpdatedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return s.menus.Create(ctx, menu)
}

// GetMenu retrieves a menu by ID including its hierarchical items.
func (s *service) GetMenu(ctx context.Context, id uuid.UUID) (*Menu, error) {
	menu, err := s.menus.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrMenuNotFound) {
			return nil, err
		}
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}
	return s.hydrateMenu(ctx, menu)
}

// GetMenuByCode retrieves a menu using its code.
func (s *service) GetMenuByCode(ctx context.Context, code string) (*Menu, error) {
	menu, err := s.menus.GetByCode(ctx, strings.TrimSpace(code))
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}
	return s.hydrateMenu(ctx, menu)
}

// AddMenuItem registers a new menu item at the specified position.
func (s *service) AddMenuItem(ctx context.Context, input AddMenuItemInput) (*MenuItem, error) {
	menu, err := s.menus.GetByID(ctx, input.MenuID)
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}

	if s.translationsRequired() && len(input.Translations) == 0 && !input.AllowMissingTranslations {
		return nil, ErrMenuItemTranslations
	}

	if input.Position < 0 {
		return nil, ErrMenuItemPosition
	}

	target, err := s.sanitizeTarget(ctx, input.Target)
	if err != nil {
		return nil, err
	}

	var parentID *uuid.UUID
	if input.ParentID != nil {
		parentID = new(uuid.UUID)
		*parentID = *input.ParentID
		parent, err := s.items.GetByID(ctx, *input.ParentID)
		if err != nil {
			var notFound *NotFoundError
			if errors.As(err, &notFound) {
				return nil, ErrMenuItemParentInvalid
			}
			return nil, err
		}
		if parent.MenuID != menu.ID {
			return nil, ErrMenuItemParentInvalid
		}
	}

	// Re-index siblings to make room.
	siblings, err := s.fetchSiblings(ctx, menu.ID, parentID)
	if err != nil {
		return nil, err
	}
	insertAt := input.Position
	if insertAt > len(siblings) {
		insertAt = len(siblings)
	}
	if err := s.shiftSiblings(ctx, siblings, insertAt); err != nil {
		return nil, err
	}

	now := s.now()
	item := &MenuItem{
		ID:        s.id(),
		MenuID:    menu.ID,
		ParentID:  parentID,
		Position:  insertAt,
		Target:    target,
		CreatedBy: input.CreatedBy,
		UpdatedBy: input.UpdatedBy,
		CreatedAt: now,
		UpdatedAt: now,
	}

	created, err := s.items.Create(ctx, item)
	if err != nil {
		return nil, err
	}

	trs, err := s.attachTranslations(ctx, created.ID, input.Translations)
	if err != nil {
		return nil, err
	}
	created.Translations = trs
	return created, nil
}

// UpdateMenuItem mutates supported fields on an existing item.
func (s *service) UpdateMenuItem(ctx context.Context, input UpdateMenuItemInput) (*MenuItem, error) {
	if input.ItemID == uuid.Nil {
		return nil, ErrMenuItemNotFound
	}

	item, err := s.items.GetByID(ctx, input.ItemID)
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuItemNotFound
		}
		return nil, err
	}

	if input.Target != nil {
		target, err := s.sanitizeTarget(ctx, input.Target)
		if err != nil {
			return nil, err
		}
		item.Target = target
	}

	if input.Position != nil {
		if *input.Position < 0 {
			return nil, ErrMenuItemPosition
		}
		siblings, err := s.fetchSiblings(ctx, item.MenuID, item.ParentID)
		if err != nil {
			return nil, err
		}
		desired := *input.Position
		if desired > len(siblings) {
			desired = len(siblings)
		}
		if err := s.repositionItem(ctx, item, siblings, desired); err != nil {
			return nil, err
		}
		item.Position = desired
	}

	if input.ParentID != nil {
		parentID := input.ParentID
		if parentID != nil && *parentID != uuid.Nil {
			parent, err := s.items.GetByID(ctx, *parentID)
			if err != nil {
				var notFound *NotFoundError
				if errors.As(err, &notFound) {
					return nil, ErrMenuItemParentInvalid
				}
				return nil, err
			}
			if parent.MenuID != item.MenuID {
				return nil, ErrMenuItemParentInvalid
			}
		}
		item.ParentID = parentID
	}

	item.UpdatedBy = input.UpdatedBy
	item.UpdatedAt = s.now()
	updated, err := s.items.Update(ctx, item)
	if err != nil {
		return nil, err
	}

	translations, err := s.translations.ListByMenuItem(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	updated.Translations = translations
	return updated, nil
}

// ReorderMenuItems overwrites the hierarchy/positions for a menu's items.
func (s *service) ReorderMenuItems(ctx context.Context, input ReorderMenuItemsInput) ([]*MenuItem, error) {
	if input.MenuID == uuid.Nil {
		return nil, ErrMenuNotFound
	}

	if _, err := s.menus.GetByID(ctx, input.MenuID); err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}

	items, err := s.items.ListByMenu(ctx, input.MenuID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}

	if len(input.Items) != len(items) {
		return nil, fmt.Errorf("menus: reorder requires %d items, got %d", len(items), len(input.Items))
	}

	itemIndex := make(map[uuid.UUID]*MenuItem, len(items))
	for _, item := range items {
		itemIndex[item.ID] = item
	}

	parentMap := make(map[uuid.UUID]*uuid.UUID, len(items))
	positionMap := make(map[string][]ItemOrder)
	seen := make(map[uuid.UUID]struct{}, len(items))

	for _, entry := range input.Items {
		if entry.ItemID == uuid.Nil {
			return nil, ErrMenuItemNotFound
		}
		if entry.Position < 0 {
			return nil, ErrMenuItemPosition
		}
		if _, ok := itemIndex[entry.ItemID]; !ok {
			return nil, ErrMenuItemNotFound
		}
		if entry.ParentID != nil {
			if *entry.ParentID == entry.ItemID {
				return nil, ErrMenuItemCycle
			}
			parent, ok := itemIndex[*entry.ParentID]
			if !ok || parent.MenuID != input.MenuID {
				return nil, ErrMenuItemParentInvalid
			}
		}

		parentMap[entry.ItemID] = entry.ParentID
		parentKey := parentKey(entry.ParentID)
		positionMap[parentKey] = append(positionMap[parentKey], entry)
		if _, dup := seen[entry.ItemID]; dup {
			return nil, fmt.Errorf("menus: duplicate item %s in reorder request", entry.ItemID)
		}
		seen[entry.ItemID] = struct{}{}
	}

	if hasCycle(parentMap) {
		return nil, ErrMenuItemCycle
	}

	// Apply ordering per parent
	for key, entries := range positionMap {
		slices.SortFunc(entries, func(a, b ItemOrder) int {
			return a.Position - b.Position
		})
		for idx, entry := range entries {
			item := itemIndex[entry.ItemID]
			item.ParentID = normalizeUUIDPtr(entry.ParentID)
			item.Position = idx
			item.UpdatedAt = s.now()
			if err := s.persistMenuItem(ctx, item); err != nil {
				return nil, err
			}
		}
		// Update map after reorder
		positionMap[key] = entries
	}

	// Return items ordered by parent and position for convenience.
	result := slices.Clone(items)
	slices.SortFunc(result, func(a, b *MenuItem) int {
		if parentKey(a.ParentID) == parentKey(b.ParentID) {
			return a.Position - b.Position
		}
		return strings.Compare(parentKey(a.ParentID), parentKey(b.ParentID))
	})

	return result, nil
}

// AddMenuItemTranslation registers a localized label for a menu item.
func (s *service) AddMenuItemTranslation(ctx context.Context, input AddMenuItemTranslationInput) (*MenuItemTranslation, error) {
	if input.ItemID == uuid.Nil {
		return nil, ErrMenuItemNotFound
	}
	if strings.TrimSpace(input.Label) == "" {
		return nil, ErrTranslationLabelRequired
	}

	item, err := s.items.GetByID(ctx, input.ItemID)
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuItemNotFound
		}
		return nil, err
	}

	locale, err := s.lookupLocale(ctx, input.Locale)
	if err != nil {
		return nil, err
	}

	if existing, err := s.translations.GetByMenuItemAndLocale(ctx, item.ID, locale.ID); err == nil && existing != nil {
		return nil, ErrTranslationExists
	}

	now := s.now()
	translation := &MenuItemTranslation{
		ID:          s.id(),
		MenuItemID:  item.ID,
		LocaleID:    locale.ID,
		Label:       strings.TrimSpace(input.Label),
		URLOverride: input.URLOverride,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := s.translations.Create(ctx, translation)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// ResolveNavigation builds a localized navigation tree for the requested menu.
func (s *service) ResolveNavigation(ctx context.Context, menuCode string, locale string) ([]NavigationNode, error) {
	menu, err := s.GetMenuByCode(ctx, strings.TrimSpace(menuCode))
	if err != nil {
		return nil, err
	}
	if menu == nil || len(menu.Items) == 0 {
		return nil, nil
	}

	var localeID uuid.UUID
	if trimmed := strings.TrimSpace(locale); trimmed != "" {
		if loc, err := s.lookupLocale(ctx, trimmed); err == nil {
			localeID = loc.ID
		}
	}

	nodes := make([]NavigationNode, 0, len(menu.Items))
	for _, item := range menu.Items {
		node, err := s.buildNavigationNode(ctx, menu.Code, item, localeID, locale)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (s *service) InvalidateCache(ctx context.Context) error {
	var errs []error

	if invalidator, ok := s.menus.(cacheInvalidator); ok {
		if err := invalidator.InvalidateCache(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if invalidator, ok := s.items.(cacheInvalidator); ok {
		if err := invalidator.InvalidateCache(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if invalidator, ok := s.translations.(cacheInvalidator); ok {
		if err := invalidator.InvalidateCache(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *service) buildNavigationNode(ctx context.Context, menuCode string, item *MenuItem, localeID uuid.UUID, locale string) (NavigationNode, error) {
	node := NavigationNode{
		ID: item.ID,
	}
	if item.Target != nil {
		node.Target = maps.Clone(item.Target)
	}

	primary, fallback := selectMenuTranslation(item.Translations, localeID)
	translation := primary
	if translation == nil {
		translation = fallback
	}

	if translation != nil {
		node.Label = translation.Label
		if translation.URLOverride != nil {
			if url := strings.TrimSpace(*translation.URLOverride); url != "" {
				node.URL = url
			}
		}
	}

	if node.URL == "" {
		node.URL = s.resolveNodeURL(ctx, menuCode, item, localeID, locale)
	}

	if node.Label == "" {
		if slug, ok := extractSlug(item.Target); ok && slug != "" {
			node.Label = slug
		} else if translation != nil {
			node.Label = translation.Label
		} else if targetType, ok := item.Target["type"].(string); ok {
			node.Label = targetType
		} else {
			node.Label = item.ID.String()
		}
	}

	if len(item.Children) > 0 {
		children := make([]NavigationNode, 0, len(item.Children))
		for _, child := range item.Children {
			childNode, err := s.buildNavigationNode(ctx, menuCode, child, localeID, locale)
			if err != nil {
				return NavigationNode{}, err
			}
			children = append(children, childNode)
		}
		node.Children = children
	}

	return node, nil
}

func (s *service) hydrateMenu(ctx context.Context, menu *Menu) (*Menu, error) {
	items, err := s.items.ListByMenu(ctx, menu.ID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		menu.Items = nil
		return menu, nil
	}

	trMap := make(map[uuid.UUID][]*MenuItemTranslation, len(items))
	for _, item := range items {
		list, err := s.translations.ListByMenuItem(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		trMap[item.ID] = list
	}

	menu.Items = buildHierarchy(items, trMap)
	return menu, nil
}

func (s *service) resolveNodeURL(ctx context.Context, menuCode string, item *MenuItem, localeID uuid.UUID, locale string) string {
	if item == nil {
		return ""
	}
	if s.urlResolver != nil {
		url, err := s.urlResolver.Resolve(ctx, ResolveRequest{
			MenuCode: menuCode,
			Item:     item,
			Locale:   locale,
			LocaleID: localeID,
		})
		if err == nil {
			if trimmed := strings.TrimSpace(url); trimmed != "" {
				return trimmed
			}
		}
	}
	url, err := s.resolveURLForTarget(ctx, item.Target, localeID)
	if err != nil {
		return ""
	}
	return url
}

func (s *service) attachTranslations(ctx context.Context, itemID uuid.UUID, inputs []MenuItemTranslationInput) ([]*MenuItemTranslation, error) {
	seen := make(map[string]struct{}, len(inputs))
	translations := make([]*MenuItemTranslation, 0, len(inputs))
	now := s.now()

	for _, tr := range inputs {
		localeCode := strings.TrimSpace(tr.Locale)
		if localeCode == "" {
			return nil, ErrUnknownLocale
		}
		if _, exists := seen[localeCode]; exists {
			return nil, ErrMenuItemDuplicateLocale
		}
		seen[localeCode] = struct{}{}

		locale, err := s.lookupLocale(ctx, localeCode)
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(tr.Label) == "" {
			return nil, ErrTranslationLabelRequired
		}

		if existing, err := s.translations.GetByMenuItemAndLocale(ctx, itemID, locale.ID); err == nil && existing != nil {
			return nil, ErrTranslationExists
		}

		record := &MenuItemTranslation{
			ID:          s.id(),
			MenuItemID:  itemID,
			LocaleID:    locale.ID,
			Label:       strings.TrimSpace(tr.Label),
			URLOverride: tr.URLOverride,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		created, err := s.translations.Create(ctx, record)
		if err != nil {
			return nil, err
		}
		translations = append(translations, created)
	}

	return translations, nil
}

func (s *service) lookupLocale(ctx context.Context, code string) (*content.Locale, error) {
	if strings.TrimSpace(code) == "" {
		return nil, ErrUnknownLocale
	}
	locale, err := s.locales.GetByCode(ctx, strings.TrimSpace(code))
	if err != nil {
		return nil, ErrUnknownLocale
	}
	return locale, nil
}

func (s *service) fetchSiblings(ctx context.Context, menuID uuid.UUID, parentID *uuid.UUID) ([]*MenuItem, error) {
	items, err := s.items.ListByMenu(ctx, menuID)
	if err != nil {
		return nil, err
	}
	out := make([]*MenuItem, 0, len(items))
	for _, item := range items {
		if uuidPtrEqual(item.ParentID, parentID) {
			out = append(out, item)
		}
	}
	slices.SortFunc(out, func(a, b *MenuItem) int {
		return a.Position - b.Position
	})
	return out, nil
}

func (s *service) shiftSiblings(ctx context.Context, siblings []*MenuItem, start int) error {
	for i := len(siblings) - 1; i >= start; i-- {
		sibling := siblings[i]
		sibling.Position++
		sibling.UpdatedAt = s.now()
		if _, err := s.items.Update(ctx, sibling); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) repositionItem(ctx context.Context, item *MenuItem, siblings []*MenuItem, desired int) error {
	currentIdx := -1
	for idx, sib := range siblings {
		if sib.ID == item.ID {
			currentIdx = idx
			break
		}
	}
	if currentIdx == -1 {
		// Item may have been moved or parent changed; just ensure positions consistent.
		siblings = append(siblings, item)
	}

	if desired == currentIdx {
		return nil
	}

	// Remove item and re-insert.
	var remaining []*MenuItem
	for _, sib := range siblings {
		if sib.ID != item.ID {
			remaining = append(remaining, sib)
		}
	}

	if desired > len(remaining) {
		desired = len(remaining)
	}

	updatedOrder := append([]*MenuItem{}, remaining[:desired]...)
	updatedOrder = append(updatedOrder, item)
	updatedOrder = append(updatedOrder, remaining[desired:]...)

	for idx, sib := range updatedOrder {
		if sib.Position != idx {
			sib.Position = idx
			sib.UpdatedAt = s.now()
			if _, err := s.items.Update(ctx, sib); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *service) persistMenuItem(ctx context.Context, item *MenuItem) error {
	_, err := s.items.Update(ctx, item)
	return err
}

func buildHierarchy(items []*MenuItem, translations map[uuid.UUID][]*MenuItemTranslation) []*MenuItem {
	byID := make(map[uuid.UUID]*MenuItem, len(items))
	children := make(map[string][]*MenuItem, len(items))

	for _, item := range items {
		clone := *item
		if item.Target != nil {
			clone.Target = maps.Clone(item.Target)
		}
		clone.Children = nil
		clone.Translations = translations[item.ID]
		byID[item.ID] = &clone
		parentKey := parentKey(item.ParentID)
		children[parentKey] = append(children[parentKey], &clone)
	}

	for _, item := range byID {
		key := parentKey(&item.ID)
		if kids, ok := children[key]; ok {
			slices.SortFunc(kids, func(a, b *MenuItem) int {
				return a.Position - b.Position
			})
			item.Children = kids
		}
	}

	rootKey := parentKey(nil)
	root := children[rootKey]
	slices.SortFunc(root, func(a, b *MenuItem) int {
		return a.Position - b.Position
	})
	return root
}

func parentKey(id *uuid.UUID) string {
	if id == nil {
		return "root"
	}
	return id.String()
}

func hasCycle(parents map[uuid.UUID]*uuid.UUID) bool {
	visited := make(map[uuid.UUID]int, len(parents))

	var visit func(uuid.UUID) bool
	visit = func(id uuid.UUID) bool {
		state := visited[id]
		if state == 1 {
			return true
		}
		if state == 2 {
			return false
		}
		visited[id] = 1
		if parent := parents[id]; parent != nil {
			if visit(*parent) {
				return true
			}
		}
		visited[id] = 2
		return false
	}

	for id := range parents {
		if visit(id) {
			return true
		}
	}
	return false
}

func (s *service) sanitizeTarget(ctx context.Context, raw map[string]any) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, ErrMenuItemTargetMissing
	}

	normalized := maps.Clone(raw)
	typVal, ok := normalized["type"]
	if !ok {
		return nil, ErrMenuItemTargetMissing
	}

	typ := strings.ToLower(strings.TrimSpace(fmt.Sprint(typVal)))
	if typ == "" {
		return nil, ErrMenuItemTargetMissing
	}
	normalized["type"] = typ

	switch typ {
	case "page":
		return s.sanitizePageTarget(ctx, normalized)
	default:
		if urlVal, ok := normalized["url"]; ok {
			if url := strings.TrimSpace(fmt.Sprint(urlVal)); url != "" {
				normalized["url"] = url
			} else {
				delete(normalized, "url")
			}
		}
		return normalized, nil
	}
}

func (s *service) sanitizePageTarget(ctx context.Context, target map[string]any) (map[string]any, error) {
	cloned := maps.Clone(target)

	slug, _ := extractSlug(cloned)
	if slug != "" {
		cloned["slug"] = slug
	} else {
		delete(cloned, "slug")
	}

	var (
		pageID uuid.UUID
		hasID  bool
	)
	if rawID, ok := cloned["page_id"]; ok {
		id, okID, err := parseUUIDValue(rawID)
		if err != nil {
			return nil, err
		}
		if okID {
			pageID = id
			hasID = true
		} else {
			delete(cloned, "page_id")
		}
	} else {
		delete(cloned, "page_id")
	}

	if slug == "" && !hasID {
		return nil, ErrMenuItemPageSlugRequired
	}

	if s.pageRepo != nil {
		var (
			page *pages.Page
			err  error
		)
		if hasID {
			page, err = s.pageRepo.GetByID(ctx, pageID)
		} else {
			page, err = s.pageRepo.GetBySlug(ctx, slug)
		}
		if err != nil {
			var notFound *pages.PageNotFoundError
			if errors.As(err, &notFound) {
				return nil, ErrMenuItemPageNotFound
			}
			return nil, err
		}
		if page != nil {
			pageID = page.ID
			if slug == "" {
				slug = strings.TrimSpace(page.Slug)
				cloned["slug"] = slug
			}
		}
	}

	if pageID != uuid.Nil {
		cloned["page_id"] = pageID.String()
	}
	if slug != "" {
		cloned["slug"] = slug
	}

	return cloned, nil
}

func (s *service) resolveURLForTarget(ctx context.Context, target map[string]any, localeID uuid.UUID) (string, error) {
	if len(target) == 0 {
		return "", nil
	}
	typVal, ok := target["type"]
	if !ok {
		return "", nil
	}
	typ := strings.ToLower(strings.TrimSpace(fmt.Sprint(typVal)))
	switch typ {
	case "page":
		return s.resolvePageURL(ctx, target, localeID)
	default:
		if raw, ok := target["url"]; ok {
			return strings.TrimSpace(fmt.Sprint(raw)), nil
		}
	}
	return "", nil
}

func (s *service) resolvePageURL(ctx context.Context, target map[string]any, localeID uuid.UUID) (string, error) {
	slug, _ := extractSlug(target)

	var pageID uuid.UUID
	if raw, ok := target["page_id"]; ok {
		if id, hasID, err := parseUUIDValue(raw); err == nil && hasID {
			pageID = id
		}
	}

	if s.pageRepo == nil {
		if slug != "" {
			return ensureLeadingSlash(slug), nil
		}
		return "", nil
	}

	var (
		page *pages.Page
		err  error
	)
	if pageID != uuid.Nil {
		page, err = s.pageRepo.GetByID(ctx, pageID)
	} else if slug != "" {
		page, err = s.pageRepo.GetBySlug(ctx, slug)
	} else {
		return "", ErrMenuItemPageSlugRequired
	}
	if err != nil {
		var notFound *pages.PageNotFoundError
		if errors.As(err, &notFound) {
			return "", ErrMenuItemPageNotFound
		}
		return "", err
	}
	if page == nil {
		return "", ErrMenuItemPageNotFound
	}

	if slug == "" {
		slug = strings.TrimSpace(page.Slug)
	}

	path := findPagePath(page, localeID)
	if path == "" {
		if slug != "" {
			return ensureLeadingSlash(slug), nil
		}
		return "", nil
	}

	return ensureLeadingSlash(path), nil
}

func findPagePath(page *pages.Page, localeID uuid.UUID) string {
	if page == nil {
		return ""
	}
	if localeID != uuid.Nil {
		for _, tr := range page.Translations {
			if tr.LocaleID == localeID && tr.Path != "" {
				return tr.Path
			}
		}
	}
	for _, tr := range page.Translations {
		if tr.Path != "" {
			return tr.Path
		}
	}
	return ""
}

func selectMenuTranslation(translations []*MenuItemTranslation, localeID uuid.UUID) (*MenuItemTranslation, *MenuItemTranslation) {
	var fallback *MenuItemTranslation
	for _, tr := range translations {
		if fallback == nil {
			fallback = tr
		}
		if localeID != uuid.Nil && tr.LocaleID == localeID {
			return tr, fallback
		}
	}
	return nil, fallback
}

func extractSlug(target map[string]any) (string, bool) {
	if target == nil {
		return "", false
	}
	raw, ok := target["slug"]
	if !ok {
		return "", false
	}
	return strings.TrimSpace(fmt.Sprint(raw)), true
}

func parseUUIDValue(value any) (uuid.UUID, bool, error) {
	switch v := value.(type) {
	case uuid.UUID:
		if v == uuid.Nil {
			return uuid.UUID{}, false, nil
		}
		return v, true, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return uuid.UUID{}, false, nil
		}
		id, err := uuid.Parse(trimmed)
		if err != nil {
			return uuid.UUID{}, false, err
		}
		return id, true, nil
	default:
		return uuid.UUID{}, false, fmt.Errorf("menus: unsupported identifier type %T", value)
	}
}

func ensureLeadingSlash(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func normalizeUUIDPtr(id *uuid.UUID) *uuid.UUID {
	if id == nil {
		return nil
	}
	copy := *id
	return &copy
}

func isValidCode(code string) bool {
	for _, r := range code {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' ||
			r == '_' {
			continue
		}
		return false
	}
	return true
}
