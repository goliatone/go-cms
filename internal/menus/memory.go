package menus

import (
	"context"
	"maps"
	"sync"

	"github.com/google/uuid"
)

type memoryMenuRepository struct {
	mu     sync.RWMutex
	byID   map[uuid.UUID]*Menu
	byCode map[string]uuid.UUID
}

func NewMemoryMenuRepository() MenuRepository {
	return &memoryMenuRepository{
		byID:   make(map[uuid.UUID]*Menu),
		byCode: make(map[string]uuid.UUID),
	}
}

func (m *memoryMenuRepository) Create(_ context.Context, menu *Menu) (*Menu, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneMenu(menu)
	m.byID[cloned.ID] = cloned
	if cloned.Code != "" {
		m.byCode[cloned.Code] = cloned.ID
	}
	return cloneMenu(cloned), nil
}

func (m *memoryMenuRepository) GetByID(_ context.Context, id uuid.UUID) (*Menu, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.byID[id]
	if !ok {
		return nil, &NotFoundError{Resource: "menu", Key: id.String()}
	}
	return cloneMenu(record), nil
}

func (m *memoryMenuRepository) GetByCode(_ context.Context, code string) (*Menu, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.byCode[code]
	if !ok {
		return nil, &NotFoundError{Resource: "menu", Key: code}
	}
	return cloneMenu(m.byID[id]), nil
}

func (m *memoryMenuRepository) List(_ context.Context) ([]*Menu, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]*Menu, 0, len(m.byID))
	for _, record := range m.byID {
		records = append(records, cloneMenu(record))
	}
	return records, nil
}

// NewMemoryMenuItemRepository constructs an in-memory repository for menu items.
func NewMemoryMenuItemRepository() MenuItemRepository {
	return &memoryMenuItemRepository{
		byID:     make(map[uuid.UUID]*MenuItem),
		byMenuID: make(map[uuid.UUID][]uuid.UUID),
		byParent: make(map[uuid.UUID][]uuid.UUID),
	}
}

type memoryMenuItemRepository struct {
	mu       sync.RWMutex
	byID     map[uuid.UUID]*MenuItem
	byMenuID map[uuid.UUID][]uuid.UUID
	byParent map[uuid.UUID][]uuid.UUID
}

func (m *memoryMenuItemRepository) Create(_ context.Context, item *MenuItem) (*MenuItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneMenuItem(item)
	m.byID[cloned.ID] = cloned
	m.byMenuID[cloned.MenuID] = append(m.byMenuID[cloned.MenuID], cloned.ID)
	if cloned.ParentID != nil {
		parentID := *cloned.ParentID
		m.byParent[parentID] = append(m.byParent[parentID], cloned.ID)
	}
	return cloneMenuItem(cloned), nil
}

func (m *memoryMenuItemRepository) GetByID(_ context.Context, id uuid.UUID) (*MenuItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.byID[id]
	if !ok {
		return nil, &NotFoundError{Resource: "menu_item", Key: id.String()}
	}
	return cloneMenuItem(record), nil
}

func (m *memoryMenuItemRepository) ListByMenu(_ context.Context, menuID uuid.UUID) ([]*MenuItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byMenuID[menuID]
	items := make([]*MenuItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, cloneMenuItem(m.byID[id]))
	}
	return items, nil
}

func (m *memoryMenuItemRepository) ListChildren(_ context.Context, parentID uuid.UUID) ([]*MenuItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byParent[parentID]
	children := make([]*MenuItem, 0, len(ids))
	for _, id := range ids {
		children = append(children, cloneMenuItem(m.byID[id]))
	}
	return children, nil
}

// NewMemoryMenuItemTranslationRepository constructs an in-memory repository for menu item translations.
func NewMemoryMenuItemTranslationRepository() MenuItemTranslationRepository {
	return &memoryMenuItemTranslationRepository{
		byID:         make(map[uuid.UUID]*MenuItemTranslation),
		byItem:       make(map[uuid.UUID][]uuid.UUID),
		byItemLocale: make(map[string]uuid.UUID),
	}
}

type memoryMenuItemTranslationRepository struct {
	mu           sync.RWMutex
	byID         map[uuid.UUID]*MenuItemTranslation
	byItem       map[uuid.UUID][]uuid.UUID
	byItemLocale map[string]uuid.UUID
}

func (m *memoryMenuItemTranslationRepository) Create(_ context.Context, translation *MenuItemTranslation) (*MenuItemTranslation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneMenuItemTranslation(translation)
	m.byID[cloned.ID] = cloned
	m.byItem[cloned.MenuItemID] = append(m.byItem[cloned.MenuItemID], cloned.ID)
	key := translationKey(cloned.MenuItemID, cloned.LocaleID)
	m.byItemLocale[key] = cloned.ID

	return cloneMenuItemTranslation(cloned), nil
}

func (m *memoryMenuItemTranslationRepository) GetByMenuItemAndLocale(_ context.Context, menuItemID uuid.UUID, localeID uuid.UUID) (*MenuItemTranslation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.byItemLocale[translationKey(menuItemID, localeID)]
	if !ok {
		return nil, &NotFoundError{Resource: "menu_item_translation", Key: translationKey(menuItemID, localeID)}
	}
	return cloneMenuItemTranslation(m.byID[id]), nil
}

func (m *memoryMenuItemTranslationRepository) ListByMenuItem(_ context.Context, menuItemID uuid.UUID) ([]*MenuItemTranslation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byItem[menuItemID]
	translations := make([]*MenuItemTranslation, 0, len(ids))
	for _, id := range ids {
		translations = append(translations, cloneMenuItemTranslation(m.byID[id]))
	}
	return translations, nil
}

func cloneMenu(src *Menu) *Menu {
	if src == nil {
		return nil
	}
	cloned := *src
	if len(src.Items) > 0 {
		cloned.Items = make([]*MenuItem, len(src.Items))
		for i, item := range src.Items {
			cloned.Items[i] = cloneMenuItem(item)
		}
	}
	return &cloned
}

func cloneMenuItem(src *MenuItem) *MenuItem {
	if src == nil {
		return nil
	}
	cloned := *src
	if src.Target != nil {
		cloned.Target = maps.Clone(src.Target)
	}
	cloned.Menu = nil
	cloned.Parent = nil
	if len(src.Children) > 0 {
		cloned.Children = make([]*MenuItem, len(src.Children))
		for i, child := range src.Children {
			cloned.Children[i] = cloneMenuItem(child)
		}
	}
	if len(src.Translations) > 0 {
		cloned.Translations = make([]*MenuItemTranslation, len(src.Translations))
		for i, tr := range src.Translations {
			cloned.Translations[i] = cloneMenuItemTranslation(tr)
		}
	}
	return &cloned
}

func cloneMenuItemTranslation(src *MenuItemTranslation) *MenuItemTranslation {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.MenuItem = nil
	if src.Locale != nil {
		local := *src.Locale
		cloned.Locale = &local
	}
	return &cloned
}

func translationKey(menuItemID uuid.UUID, localeID uuid.UUID) string {
	return menuItemID.String() + ":" + localeID.String()
}
