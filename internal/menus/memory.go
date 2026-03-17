package menus

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"

	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/google/uuid"
)

type memoryMenuRepository struct {
	mu         sync.RWMutex
	byID       map[uuid.UUID]*Menu
	byCode     map[string]uuid.UUID
	byLocation map[string]uuid.UUID
}

// NewMemoryMenuRepository constructs an in-memory repository for menus
func NewMemoryMenuRepository() MenuRepository {
	return &memoryMenuRepository{
		byID:       make(map[uuid.UUID]*Menu),
		byCode:     make(map[string]uuid.UUID),
		byLocation: make(map[string]uuid.UUID),
	}
}

func (m *memoryMenuRepository) Create(_ context.Context, menu *Menu) (*Menu, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneMenu(menu)
	cloned.EnvironmentID = resolveEnvironmentID(cloned.EnvironmentID, "")
	m.byID[cloned.ID] = cloned
	if cloned.Code != "" {
		m.byCode[menuCodeKey(cloned.EnvironmentID, cloned.Code)] = cloned.ID
	}
	if cloned.Location != "" {
		m.byLocation[menuLocationKey(cloned.EnvironmentID, cloned.Location)] = cloned.ID
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

func (m *memoryMenuRepository) GetByCode(_ context.Context, code string, env ...string) (*Menu, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	envID := resolveEnvironmentID(uuid.Nil, resolveEnvironmentKey(env...))
	id, ok := m.byCode[menuCodeKey(envID, code)]
	if !ok {
		return nil, &NotFoundError{Resource: "menu", Key: code}
	}
	return cloneMenu(m.byID[id]), nil
}

func (m *memoryMenuRepository) GetByLocation(_ context.Context, location string, env ...string) (*Menu, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	envID := resolveEnvironmentID(uuid.Nil, resolveEnvironmentKey(env...))
	id, ok := m.byLocation[menuLocationKey(envID, location)]
	if !ok {
		return nil, &NotFoundError{Resource: "menu", Key: location}
	}
	return cloneMenu(m.byID[id]), nil
}

func (m *memoryMenuRepository) List(_ context.Context, env ...string) ([]*Menu, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	envID := resolveEnvironmentID(uuid.Nil, resolveEnvironmentKey(env...))
	records := make([]*Menu, 0, len(m.byID))
	for _, record := range m.byID {
		if record == nil {
			continue
		}
		if !matchesEnvironment(record.EnvironmentID, envID) {
			continue
		}
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
	oldLocation := existing.Location
	cloned := cloneMenu(menu)
	cloned.EnvironmentID = resolveEnvironmentID(cloned.EnvironmentID, "")

	m.byID[cloned.ID] = cloned

	if oldCode != "" && oldCode != cloned.Code {
		delete(m.byCode, menuCodeKey(resolveEnvironmentID(existing.EnvironmentID, ""), oldCode))
	}
	if cloned.Code != "" {
		m.byCode[menuCodeKey(cloned.EnvironmentID, cloned.Code)] = cloned.ID
	}
	if oldLocation != "" && oldLocation != cloned.Location {
		delete(m.byLocation, menuLocationKey(resolveEnvironmentID(existing.EnvironmentID, ""), oldLocation))
	}
	if cloned.Location != "" {
		m.byLocation[menuLocationKey(cloned.EnvironmentID, cloned.Location)] = cloned.ID
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
		delete(m.byCode, menuCodeKey(resolveEnvironmentID(existing.EnvironmentID, ""), existing.Code))
	}
	if existing.Location != "" {
		delete(m.byLocation, menuLocationKey(resolveEnvironmentID(existing.EnvironmentID, ""), existing.Location))
	}
	return nil
}

func resolveEnvironmentKey(env ...string) string {
	if len(env) == 0 {
		return ""
	}
	return cmsenv.NormalizeKey(env[0])
}

func resolveEnvironmentID(id uuid.UUID, envKey string) uuid.UUID {
	if id != uuid.Nil {
		return id
	}
	key := strings.TrimSpace(envKey)
	if key == "" {
		key = cmsenv.DefaultKey
	}
	if parsed, err := uuid.Parse(key); err == nil {
		return parsed
	}
	key = cmsenv.NormalizeKey(key)
	if key == "" {
		key = cmsenv.DefaultKey
	}
	return cmsenv.IDForKey(key)
}

func matchesEnvironment(recordID, targetID uuid.UUID) bool {
	if targetID == uuid.Nil {
		targetID = resolveEnvironmentID(uuid.Nil, "")
	}
	if recordID == uuid.Nil {
		return targetID == resolveEnvironmentID(uuid.Nil, "")
	}
	return recordID == targetID
}

func menuCodeKey(envID uuid.UUID, code string) string {
	return envID.String() + "|" + strings.TrimSpace(code)
}

func menuLocationKey(envID uuid.UUID, location string) string {
	return envID.String() + "|" + strings.TrimSpace(location)
}

func locationBindingLocationKey(envID uuid.UUID, location string) string {
	return envID.String() + "|" + strings.TrimSpace(location)
}

func locationBindingMenuCodeKey(envID uuid.UUID, code string) string {
	return envID.String() + "|" + strings.TrimSpace(code)
}

func locationBindingProfileKey(envID uuid.UUID, code string) string {
	return envID.String() + "|" + strings.TrimSpace(code)
}

func menuViewProfileCodeKey(envID uuid.UUID, code string) string {
	return envID.String() + "|" + strings.TrimSpace(code)
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

func (m *memoryMenuItemRepository) GetByMenuAndCanonicalKey(_ context.Context, menuID uuid.UUID, key string) (*MenuItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, id := range m.byMenuID[menuID] {
		record := m.byID[id]
		if record == nil || record.CanonicalKey == nil {
			continue
		}
		if *record.CanonicalKey == key {
			return cloneMenuItem(record), nil
		}
	}
	return nil, &NotFoundError{Resource: "menu_item", Key: menuID.String() + ":" + key}
}

func (m *memoryMenuItemRepository) GetByMenuAndExternalCode(_ context.Context, menuID uuid.UUID, code string) (*MenuItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, id := range m.byMenuID[menuID] {
		record := m.byID[id]
		if record == nil {
			continue
		}
		if record.ExternalCode == code && code != "" {
			return cloneMenuItem(record), nil
		}
	}
	return nil, &NotFoundError{Resource: "menu_item", Key: menuID.String() + ":" + code}
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

func (m *memoryMenuItemRepository) BulkUpdateParentLinks(_ context.Context, items []*MenuItem) error {
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

// NewMemoryMenuLocationBindingRepository constructs an in-memory location binding repository.
func NewMemoryMenuLocationBindingRepository() MenuLocationBindingRepository {
	return &memoryMenuLocationBindingRepository{
		byID:         make(map[uuid.UUID]*MenuLocationBinding),
		byLocation:   make(map[string][]uuid.UUID),
		byMenuCode:   make(map[string][]uuid.UUID),
		byProfileKey: make(map[string][]uuid.UUID),
	}
}

type memoryMenuLocationBindingRepository struct {
	mu           sync.RWMutex
	byID         map[uuid.UUID]*MenuLocationBinding
	byLocation   map[string][]uuid.UUID
	byMenuCode   map[string][]uuid.UUID
	byProfileKey map[string][]uuid.UUID
}

func (m *memoryMenuLocationBindingRepository) Create(_ context.Context, binding *MenuLocationBinding) (*MenuLocationBinding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneMenuLocationBinding(binding)
	cloned.EnvironmentID = resolveEnvironmentID(cloned.EnvironmentID, "")
	m.byID[cloned.ID] = cloned
	locKey := locationBindingLocationKey(cloned.EnvironmentID, cloned.Location)
	m.byLocation[locKey] = appendUniqueUUID(m.byLocation[locKey], cloned.ID)
	menuKey := locationBindingMenuCodeKey(cloned.EnvironmentID, cloned.MenuCode)
	m.byMenuCode[menuKey] = appendUniqueUUID(m.byMenuCode[menuKey], cloned.ID)
	if cloned.ViewProfileCode != nil && strings.TrimSpace(*cloned.ViewProfileCode) != "" {
		profileKey := locationBindingProfileKey(cloned.EnvironmentID, *cloned.ViewProfileCode)
		m.byProfileKey[profileKey] = appendUniqueUUID(m.byProfileKey[profileKey], cloned.ID)
	}
	return cloneMenuLocationBinding(cloned), nil
}

func (m *memoryMenuLocationBindingRepository) GetByID(_ context.Context, id uuid.UUID) (*MenuLocationBinding, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.byID[id]
	if !ok {
		return nil, &NotFoundError{Resource: "menu_location_binding", Key: id.String()}
	}
	return cloneMenuLocationBinding(record), nil
}

func (m *memoryMenuLocationBindingRepository) ListByLocation(_ context.Context, location string, env ...string) ([]*MenuLocationBinding, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	envID := resolveEnvironmentID(uuid.Nil, resolveEnvironmentKey(env...))
	ids := m.byLocation[locationBindingLocationKey(envID, location)]
	out := make([]*MenuLocationBinding, 0, len(ids))
	for _, id := range ids {
		if record := m.byID[id]; record != nil {
			out = append(out, cloneMenuLocationBinding(record))
		}
	}
	return out, nil
}

func (m *memoryMenuLocationBindingRepository) List(_ context.Context, env ...string) ([]*MenuLocationBinding, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	envID := resolveEnvironmentID(uuid.Nil, resolveEnvironmentKey(env...))
	out := make([]*MenuLocationBinding, 0, len(m.byID))
	for _, record := range m.byID {
		if record == nil || !matchesEnvironment(record.EnvironmentID, envID) {
			continue
		}
		out = append(out, cloneMenuLocationBinding(record))
	}
	return out, nil
}

func (m *memoryMenuLocationBindingRepository) Update(_ context.Context, binding *MenuLocationBinding) (*MenuLocationBinding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.byID[binding.ID]
	if !ok {
		return nil, &NotFoundError{Resource: "menu_location_binding", Key: binding.ID.String()}
	}
	old := cloneMenuLocationBinding(existing)
	cloned := cloneMenuLocationBinding(binding)
	cloned.EnvironmentID = resolveEnvironmentID(cloned.EnvironmentID, "")
	m.byID[cloned.ID] = cloned
	m.reindexBinding(old, cloned)
	return cloneMenuLocationBinding(cloned), nil
}

func (m *memoryMenuLocationBindingRepository) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.byID[id]
	if !ok {
		return &NotFoundError{Resource: "menu_location_binding", Key: id.String()}
	}
	delete(m.byID, id)
	locKey := locationBindingLocationKey(resolveEnvironmentID(existing.EnvironmentID, ""), existing.Location)
	m.byLocation[locKey] = removeUUID(m.byLocation[locKey], id)
	menuKey := locationBindingMenuCodeKey(resolveEnvironmentID(existing.EnvironmentID, ""), existing.MenuCode)
	m.byMenuCode[menuKey] = removeUUID(m.byMenuCode[menuKey], id)
	if existing.ViewProfileCode != nil && strings.TrimSpace(*existing.ViewProfileCode) != "" {
		profileKey := locationBindingProfileKey(resolveEnvironmentID(existing.EnvironmentID, ""), *existing.ViewProfileCode)
		m.byProfileKey[profileKey] = removeUUID(m.byProfileKey[profileKey], id)
	}
	return nil
}

func (m *memoryMenuLocationBindingRepository) reindexBinding(old, updated *MenuLocationBinding) {
	if old == nil || updated == nil {
		return
	}
	oldEnv := resolveEnvironmentID(old.EnvironmentID, "")
	newEnv := resolveEnvironmentID(updated.EnvironmentID, "")
	if old.Location != updated.Location || oldEnv != newEnv {
		m.byLocation[locationBindingLocationKey(oldEnv, old.Location)] = removeUUID(
			m.byLocation[locationBindingLocationKey(oldEnv, old.Location)],
			old.ID,
		)
	}
	m.byLocation[locationBindingLocationKey(newEnv, updated.Location)] = appendUniqueUUID(
		m.byLocation[locationBindingLocationKey(newEnv, updated.Location)],
		updated.ID,
	)

	if old.MenuCode != updated.MenuCode || oldEnv != newEnv {
		m.byMenuCode[locationBindingMenuCodeKey(oldEnv, old.MenuCode)] = removeUUID(
			m.byMenuCode[locationBindingMenuCodeKey(oldEnv, old.MenuCode)],
			old.ID,
		)
	}
	m.byMenuCode[locationBindingMenuCodeKey(newEnv, updated.MenuCode)] = appendUniqueUUID(
		m.byMenuCode[locationBindingMenuCodeKey(newEnv, updated.MenuCode)],
		updated.ID,
	)

	oldProfile := ""
	if old.ViewProfileCode != nil {
		oldProfile = strings.TrimSpace(*old.ViewProfileCode)
	}
	newProfile := ""
	if updated.ViewProfileCode != nil {
		newProfile = strings.TrimSpace(*updated.ViewProfileCode)
	}
	if oldProfile != "" && (oldProfile != newProfile || oldEnv != newEnv) {
		m.byProfileKey[locationBindingProfileKey(oldEnv, oldProfile)] = removeUUID(
			m.byProfileKey[locationBindingProfileKey(oldEnv, oldProfile)],
			old.ID,
		)
	}
	if newProfile != "" {
		m.byProfileKey[locationBindingProfileKey(newEnv, newProfile)] = appendUniqueUUID(
			m.byProfileKey[locationBindingProfileKey(newEnv, newProfile)],
			updated.ID,
		)
	}
}

// NewMemoryMenuViewProfileRepository constructs an in-memory view profile repository.
func NewMemoryMenuViewProfileRepository() MenuViewProfileRepository {
	return &memoryMenuViewProfileRepository{
		byID:   make(map[uuid.UUID]*MenuViewProfile),
		byCode: make(map[string]uuid.UUID),
	}
}

type memoryMenuViewProfileRepository struct {
	mu     sync.RWMutex
	byID   map[uuid.UUID]*MenuViewProfile
	byCode map[string]uuid.UUID
}

func (m *memoryMenuViewProfileRepository) Create(_ context.Context, profile *MenuViewProfile) (*MenuViewProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneMenuViewProfile(profile)
	cloned.EnvironmentID = resolveEnvironmentID(cloned.EnvironmentID, "")
	m.byID[cloned.ID] = cloned
	m.byCode[menuViewProfileCodeKey(cloned.EnvironmentID, cloned.Code)] = cloned.ID
	return cloneMenuViewProfile(cloned), nil
}

func (m *memoryMenuViewProfileRepository) GetByID(_ context.Context, id uuid.UUID) (*MenuViewProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.byID[id]
	if !ok {
		return nil, &NotFoundError{Resource: "menu_view_profile", Key: id.String()}
	}
	return cloneMenuViewProfile(record), nil
}

func (m *memoryMenuViewProfileRepository) GetByCode(_ context.Context, code string, env ...string) (*MenuViewProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	envID := resolveEnvironmentID(uuid.Nil, resolveEnvironmentKey(env...))
	id, ok := m.byCode[menuViewProfileCodeKey(envID, code)]
	if !ok {
		return nil, &NotFoundError{Resource: "menu_view_profile", Key: code}
	}
	return cloneMenuViewProfile(m.byID[id]), nil
}

func (m *memoryMenuViewProfileRepository) List(_ context.Context, env ...string) ([]*MenuViewProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	envID := resolveEnvironmentID(uuid.Nil, resolveEnvironmentKey(env...))
	out := make([]*MenuViewProfile, 0, len(m.byID))
	for _, record := range m.byID {
		if record == nil || !matchesEnvironment(record.EnvironmentID, envID) {
			continue
		}
		out = append(out, cloneMenuViewProfile(record))
	}
	return out, nil
}

func (m *memoryMenuViewProfileRepository) Update(_ context.Context, profile *MenuViewProfile) (*MenuViewProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.byID[profile.ID]
	if !ok {
		return nil, &NotFoundError{Resource: "menu_view_profile", Key: profile.ID.String()}
	}
	oldCodeKey := menuViewProfileCodeKey(resolveEnvironmentID(existing.EnvironmentID, ""), existing.Code)
	cloned := cloneMenuViewProfile(profile)
	cloned.EnvironmentID = resolveEnvironmentID(cloned.EnvironmentID, "")
	m.byID[cloned.ID] = cloned
	newCodeKey := menuViewProfileCodeKey(cloned.EnvironmentID, cloned.Code)
	if oldCodeKey != newCodeKey {
		delete(m.byCode, oldCodeKey)
	}
	m.byCode[newCodeKey] = cloned.ID
	return cloneMenuViewProfile(cloned), nil
}

func (m *memoryMenuViewProfileRepository) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.byID[id]
	if !ok {
		return &NotFoundError{Resource: "menu_view_profile", Key: id.String()}
	}
	delete(m.byID, id)
	delete(m.byCode, menuViewProfileCodeKey(resolveEnvironmentID(existing.EnvironmentID, ""), existing.Code))
	return nil
}

func cloneMenu(src *Menu) *Menu {
	if src == nil {
		return nil
	}
	cloned := *src
	if src.Description != nil {
		description := *src.Description
		cloned.Description = &description
	}
	if src.Locale != nil {
		locale := *src.Locale
		cloned.Locale = &locale
	}
	if src.FamilyID != nil {
		groupID := *src.FamilyID
		cloned.FamilyID = &groupID
	}
	if src.PublishedAt != nil {
		publishedAt := *src.PublishedAt
		cloned.PublishedAt = &publishedAt
	}
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
	if src.ParentRef != nil {
		ref := *src.ParentRef
		cloned.ParentRef = &ref
	}
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

func cloneMenuLocationBinding(src *MenuLocationBinding) *MenuLocationBinding {
	if src == nil {
		return nil
	}
	cloned := *src
	if src.ViewProfileCode != nil {
		code := *src.ViewProfileCode
		cloned.ViewProfileCode = &code
	}
	if src.Locale != nil {
		locale := *src.Locale
		cloned.Locale = &locale
	}
	if src.PublishedAt != nil {
		publishedAt := *src.PublishedAt
		cloned.PublishedAt = &publishedAt
	}
	return &cloned
}

func cloneMenuViewProfile(src *MenuViewProfile) *MenuViewProfile {
	if src == nil {
		return nil
	}
	cloned := *src
	if src.MaxTopLevel != nil {
		topLevel := *src.MaxTopLevel
		cloned.MaxTopLevel = &topLevel
	}
	if src.MaxDepth != nil {
		maxDepth := *src.MaxDepth
		cloned.MaxDepth = &maxDepth
	}
	if len(src.IncludeItemIDs) > 0 {
		cloned.IncludeItemIDs = slices.Clone(src.IncludeItemIDs)
	}
	if len(src.ExcludeItemIDs) > 0 {
		cloned.ExcludeItemIDs = slices.Clone(src.ExcludeItemIDs)
	}
	if src.PublishedAt != nil {
		publishedAt := *src.PublishedAt
		cloned.PublishedAt = &publishedAt
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
	if slices.Contains(list, id) {
		return list
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
