package menus

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/google/uuid"
)

type memoryMenuRepository struct {
	mu     sync.RWMutex
	byID   map[uuid.UUID]*Menu
	byCode map[string]uuid.UUID
}

// NewMemoryMenuRepository constructs an in-memory repository for menus
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

func (m *memoryMenuRepository) Update(_ context.Context, menu *Menu) (*Menu, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.byID[menu.ID]
	if !ok {
		return nil, &NotFoundError{Resource: "menu", Key: menu.ID.String()}
	}

	oldCode := existing.Code
	cloned := cloneMenu(menu)

	m.byID[cloned.ID] = cloned

	if oldCode != "" && oldCode != cloned.Code {
		delete(m.byCode, oldCode)
	}
	if cloned.Code != "" {
		m.byCode[cloned.Code] = cloned.ID
	}

	return cloneMenu(cloned), nil
}

func (m *memoryMenuRepository) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.byID[id]
	if !ok {
		return &NotFoundError{Resource: "menu", Key: id.String()}
	}
	delete(m.byID, id)
	if existing.Code != "" {
		delete(m.byCode, existing.Code)
	}
	return nil
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

func (m *memoryMenuItemRepository) Update(_ context.Context, item *MenuItem) (*MenuItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.byID[item.ID]
	if !ok {
		return nil, &NotFoundError{Resource: "menu_item", Key: item.ID.String()}
	}

	oldMenuID := existing.MenuID
	var oldParentID *uuid.UUID
	if existing.ParentID != nil {
		tmp := *existing.ParentID
		oldParentID = &tmp
	}

	cloned := cloneMenuItem(item)
	m.byID[cloned.ID] = cloned

	// Update menu index if needed.
	if oldMenuID != cloned.MenuID {
		m.byMenuID[oldMenuID] = removeUUID(m.byMenuID[oldMenuID], cloned.ID)
		m.byMenuID[cloned.MenuID] = appendUniqueUUID(m.byMenuID[cloned.MenuID], cloned.ID)
	}

	// Update parent index if changed.
	if !uuidPtrEqual(oldParentID, cloned.ParentID) {
		if oldParentID != nil {
			m.byParent[*oldParentID] = removeUUID(m.byParent[*oldParentID], cloned.ID)
		}
		if cloned.ParentID != nil {
			m.byParent[*cloned.ParentID] = appendUniqueUUID(m.byParent[*cloned.ParentID], cloned.ID)
		}
	}

	return cloneMenuItem(cloned), nil
}

func (m *memoryMenuItemRepository) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.byID[id]
	if !ok {
		return &NotFoundError{Resource: "menu_item", Key: id.String()}
	}

	delete(m.byID, id)
	m.byMenuID[item.MenuID] = removeUUID(m.byMenuID[item.MenuID], id)
	if item.ParentID != nil {
		m.byParent[*item.ParentID] = removeUUID(m.byParent[*item.ParentID], id)
	}
	return nil
}

func (m *memoryMenuItemRepository) BulkUpdateHierarchy(_ context.Context, items []*MenuItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, record := range items {
		existing, ok := m.byID[record.ID]
		if !ok {
			return &NotFoundError{Resource: "menu_item", Key: record.ID.String()}
		}
		var oldParent *uuid.UUID
		if existing.ParentID != nil {
			tmp := *existing.ParentID
			oldParent = &tmp
		}

		cloned := cloneMenuItem(record)
		m.byID[cloned.ID] = cloned

		if !uuidPtrEqual(oldParent, cloned.ParentID) {
			if oldParent != nil {
				m.byParent[*oldParent] = removeUUID(m.byParent[*oldParent], cloned.ID)
			}
			if cloned.ParentID != nil {
				m.byParent[*cloned.ParentID] = appendUniqueUUID(m.byParent[*cloned.ParentID], cloned.ID)
			}
		}
	}

	return nil
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

func (m *memoryMenuItemTranslationRepository) Update(_ context.Context, translation *MenuItemTranslation) (*MenuItemTranslation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.byID[translation.ID]
	if !ok {
		return nil, &NotFoundError{Resource: "menu_item_translation", Key: translation.ID.String()}
	}

	oldKey := translationKey(existing.MenuItemID, existing.LocaleID)

	cloned := cloneMenuItemTranslation(translation)
	m.byID[cloned.ID] = cloned

	// Update locale index if changed.
	if oldKey != translationKey(cloned.MenuItemID, cloned.LocaleID) {
		delete(m.byItemLocale, oldKey)
		m.byItemLocale[translationKey(cloned.MenuItemID, cloned.LocaleID)] = cloned.ID
		// also adjust byItem slices if menu item changed
		if existing.MenuItemID != cloned.MenuItemID {
			m.byItem[existing.MenuItemID] = removeUUID(m.byItem[existing.MenuItemID], cloned.ID)
			m.byItem[cloned.MenuItemID] = appendUniqueUUID(m.byItem[cloned.MenuItemID], cloned.ID)
		}
	}

	return cloneMenuItemTranslation(cloned), nil
}

func (m *memoryMenuItemTranslationRepository) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	translation, ok := m.byID[id]
	if !ok {
		return &NotFoundError{Resource: "menu_item_translation", Key: id.String()}
	}
	delete(m.byID, id)
	m.byItem[translation.MenuItemID] = removeUUID(m.byItem[translation.MenuItemID], id)
	delete(m.byItemLocale, translationKey(translation.MenuItemID, translation.LocaleID))
	return nil
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
	if src.Badge != nil {
		cloned.Badge = maps.Clone(src.Badge)
	}
	if src.Metadata != nil {
		cloned.Metadata = maps.Clone(src.Metadata)
	}
	if src.Styles != nil {
		cloned.Styles = maps.Clone(src.Styles)
	}
	if len(src.Permissions) > 0 {
		cloned.Permissions = slices.Clone(src.Permissions)
	}
	if len(src.Classes) > 0 {
		cloned.Classes = slices.Clone(src.Classes)
	}
	if src.CanonicalKey != nil {
		key := *src.CanonicalKey
		cloned.CanonicalKey = &key
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

func removeUUID(list []uuid.UUID, id uuid.UUID) []uuid.UUID {
	if len(list) == 0 {
		return list
	}
	out := make([]uuid.UUID, 0, len(list))
	for _, item := range list {
		if item != id {
			out = append(out, item)
		}
	}
	return out
}

func appendUniqueUUID(list []uuid.UUID, id uuid.UUID) []uuid.UUID {
	for _, item := range list {
		if item == id {
			return list
		}
	}
	return append(list, id)
}

func uuidPtrEqual(a, b *uuid.UUID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
