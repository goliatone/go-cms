package widgets

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// NewMemoryDefinitionRepository constructs an in-memory widget definition repository.
func NewMemoryDefinitionRepository() DefinitionRepository {
	return &memoryDefinitionRepository{
		byID:   make(map[uuid.UUID]*Definition),
		byName: make(map[string]uuid.UUID),
	}
}

type memoryDefinitionRepository struct {
	mu     sync.RWMutex
	byID   map[uuid.UUID]*Definition
	byName map[string]uuid.UUID
}

func (m *memoryDefinitionRepository) Create(_ context.Context, definition *Definition) (*Definition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneDefinition(definition)
	m.byID[cloned.ID] = cloned
	if cloned.Name != "" {
		m.byName[cloned.Name] = cloned.ID
	}
	return cloneDefinition(cloned), nil
}

func (m *memoryDefinitionRepository) GetByID(_ context.Context, id uuid.UUID) (*Definition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.byID[id]
	if !ok {
		return nil, &NotFoundError{Resource: "widget_definition", Key: id.String()}
	}
	return cloneDefinition(record), nil
}

func (m *memoryDefinitionRepository) GetByName(_ context.Context, name string) (*Definition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.byName[name]
	if !ok {
		return nil, &NotFoundError{Resource: "widget_definition", Key: name}
	}
	return cloneDefinition(m.byID[id]), nil
}

func (m *memoryDefinitionRepository) List(_ context.Context) ([]*Definition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]*Definition, 0, len(m.byID))
	for _, record := range m.byID {
		records = append(records, cloneDefinition(record))
	}
	return records, nil
}

// NewMemoryInstanceRepository constructs an in-memory widget instance repository.
func NewMemoryInstanceRepository() InstanceRepository {
	return &memoryInstanceRepository{
		byID:           make(map[uuid.UUID]*Instance),
		byDefinition:   make(map[uuid.UUID][]uuid.UUID),
		byArea:         make(map[string][]uuid.UUID),
		insertionOrder: make([]uuid.UUID, 0),
	}
}

type memoryInstanceRepository struct {
	mu             sync.RWMutex
	byID           map[uuid.UUID]*Instance
	byDefinition   map[uuid.UUID][]uuid.UUID
	byArea         map[string][]uuid.UUID
	insertionOrder []uuid.UUID
}

func (m *memoryInstanceRepository) Create(_ context.Context, instance *Instance) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneInstance(instance)
	m.byID[cloned.ID] = cloned
	m.byDefinition[cloned.DefinitionID] = append(m.byDefinition[cloned.DefinitionID], cloned.ID)
	if cloned.AreaCode != nil {
		area := *cloned.AreaCode
		m.byArea[area] = append(m.byArea[area], cloned.ID)
	}
	m.insertionOrder = append(m.insertionOrder, cloned.ID)
	return cloneInstance(cloned), nil
}

func (m *memoryInstanceRepository) GetByID(_ context.Context, id uuid.UUID) (*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.byID[id]
	if !ok {
		return nil, &NotFoundError{Resource: "widget_instance", Key: id.String()}
	}
	return cloneInstance(record), nil
}

func (m *memoryInstanceRepository) ListByDefinition(_ context.Context, definitionID uuid.UUID) ([]*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byDefinition[definitionID]
	instances := make([]*Instance, 0, len(ids))
	for _, id := range ids {
		instances = append(instances, cloneInstance(m.byID[id]))
	}
	return instances, nil
}

func (m *memoryInstanceRepository) ListByArea(_ context.Context, areaCode string) ([]*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byArea[areaCode]
	instances := make([]*Instance, 0, len(ids))
	for _, id := range ids {
		instances = append(instances, cloneInstance(m.byID[id]))
	}
	return instances, nil
}

func (m *memoryInstanceRepository) ListAll(_ context.Context) ([]*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instances := make([]*Instance, 0, len(m.insertionOrder))
	for _, id := range m.insertionOrder {
		if record, ok := m.byID[id]; ok {
			instances = append(instances, cloneInstance(record))
		}
	}
	return instances, nil
}

func (m *memoryInstanceRepository) Update(_ context.Context, instance *Instance) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.byID[instance.ID]
	if !ok {
		return nil, &NotFoundError{Resource: "widget_instance", Key: instance.ID.String()}
	}

	oldDefinition := existing.DefinitionID
	var oldArea *string
	if existing.AreaCode != nil {
		value := *existing.AreaCode
		oldArea = &value
	}

	cloned := cloneInstance(instance)
	m.byID[cloned.ID] = cloned

	if oldDefinition != cloned.DefinitionID {
		m.byDefinition[oldDefinition] = removeUUID(m.byDefinition[oldDefinition], cloned.ID)
		m.byDefinition[cloned.DefinitionID] = appendUniqueUUID(m.byDefinition[cloned.DefinitionID], cloned.ID)
	}

	switch {
	case oldArea == nil && cloned.AreaCode != nil:
		m.byArea[*cloned.AreaCode] = appendUniqueUUID(m.byArea[*cloned.AreaCode], cloned.ID)
	case oldArea != nil && cloned.AreaCode == nil:
		m.byArea[*oldArea] = removeUUID(m.byArea[*oldArea], cloned.ID)
	case oldArea != nil && cloned.AreaCode != nil && *oldArea != *cloned.AreaCode:
		m.byArea[*oldArea] = removeUUID(m.byArea[*oldArea], cloned.ID)
		m.byArea[*cloned.AreaCode] = appendUniqueUUID(m.byArea[*cloned.AreaCode], cloned.ID)
	}

	return cloneInstance(cloned), nil
}

// NewMemoryTranslationRepository constructs an in-memory widget translation repository.
func NewMemoryTranslationRepository() TranslationRepository {
	return &memoryTranslationRepository{
		byID:             make(map[uuid.UUID]*Translation),
		byInstanceLocale: make(map[string]uuid.UUID),
		byInstance:       make(map[uuid.UUID][]uuid.UUID),
	}
}

type memoryTranslationRepository struct {
	mu               sync.RWMutex
	byID             map[uuid.UUID]*Translation
	byInstanceLocale map[string]uuid.UUID
	byInstance       map[uuid.UUID][]uuid.UUID
}

func (m *memoryTranslationRepository) Create(_ context.Context, translation *Translation) (*Translation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneTranslation(translation)
	m.byID[cloned.ID] = cloned
	key := translationKey(cloned.WidgetInstanceID, cloned.LocaleID)
	m.byInstanceLocale[key] = cloned.ID
	m.byInstance[cloned.WidgetInstanceID] = append(m.byInstance[cloned.WidgetInstanceID], cloned.ID)
	return cloneTranslation(cloned), nil
}

func (m *memoryTranslationRepository) GetByInstanceAndLocale(_ context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.byInstanceLocale[translationKey(instanceID, localeID)]
	if !ok {
		return nil, &NotFoundError{Resource: "widget_translation", Key: translationKey(instanceID, localeID)}
	}
	return cloneTranslation(m.byID[id]), nil
}

func (m *memoryTranslationRepository) ListByInstance(_ context.Context, instanceID uuid.UUID) ([]*Translation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byInstance[instanceID]
	translations := make([]*Translation, 0, len(ids))
	for _, id := range ids {
		translations = append(translations, cloneTranslation(m.byID[id]))
	}
	return translations, nil
}

// NewMemoryAreaDefinitionRepository constructs an in-memory area definition repository.
func NewMemoryAreaDefinitionRepository() AreaDefinitionRepository {
	return &memoryAreaDefinitionRepository{
		byID:   make(map[uuid.UUID]*AreaDefinition),
		byCode: make(map[string]uuid.UUID),
	}
}

type memoryAreaDefinitionRepository struct {
	mu     sync.RWMutex
	byID   map[uuid.UUID]*AreaDefinition
	byCode map[string]uuid.UUID
}

func (m *memoryAreaDefinitionRepository) Create(_ context.Context, definition *AreaDefinition) (*AreaDefinition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneAreaDefinition(definition)
	m.byID[cloned.ID] = cloned
	m.byCode[strings.ToLower(cloned.Code)] = cloned.ID
	return cloneAreaDefinition(cloned), nil
}

func (m *memoryAreaDefinitionRepository) GetByCode(_ context.Context, code string) (*AreaDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.byCode[strings.ToLower(code)]
	if !ok {
		return nil, &NotFoundError{Resource: "widget_area_definition", Key: code}
	}
	return cloneAreaDefinition(m.byID[id]), nil
}

func (m *memoryAreaDefinitionRepository) List(_ context.Context) ([]*AreaDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*AreaDefinition, 0, len(m.byID))
	for _, def := range m.byID {
		out = append(out, cloneAreaDefinition(def))
	}
	slices.SortFunc(out, func(a, b *AreaDefinition) int {
		return strings.Compare(a.Code, b.Code)
	})
	return out, nil
}

// NewMemoryAreaPlacementRepository constructs an in-memory area placement repository.
func NewMemoryAreaPlacementRepository() AreaPlacementRepository {
	return &memoryAreaPlacementRepository{
		byKey: make(map[string][]*AreaPlacement),
		byID:  make(map[uuid.UUID]*AreaPlacement),
	}
}

type memoryAreaPlacementRepository struct {
	mu    sync.RWMutex
	byKey map[string][]*AreaPlacement
	byID  map[uuid.UUID]*AreaPlacement
}

func (m *memoryAreaPlacementRepository) ListByAreaAndLocale(_ context.Context, areaCode string, localeID *uuid.UUID) ([]*AreaPlacement, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := areaLocaleKey(areaCode, localeID)
	records := m.byKey[key]
	out := make([]*AreaPlacement, 0, len(records))
	for _, record := range records {
		out = append(out, cloneAreaPlacement(record))
	}
	slices.SortFunc(out, func(a, b *AreaPlacement) int {
		return a.Position - b.Position
	})
	return out, nil
}

func (m *memoryAreaPlacementRepository) Replace(_ context.Context, areaCode string, localeID *uuid.UUID, placements []*AreaPlacement) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := areaLocaleKey(areaCode, localeID)
	if existing := m.byKey[key]; len(existing) > 0 {
		for _, record := range existing {
			delete(m.byID, record.ID)
		}
	}

	clean := make([]*AreaPlacement, 0, len(placements))
	for idx, placement := range placements {
		cloned := cloneAreaPlacement(placement)
		if cloned.ID == uuid.Nil {
			cloned.ID = uuid.New()
		}
		cloned.AreaCode = areaCode
		cloned.LocaleID = cloneUUIDPtr(localeID)
		cloned.Position = idx
		cloned.Metadata = deepCloneMap(cloned.Metadata)
		m.byID[cloned.ID] = cloned
		clean = append(clean, cloned)
	}

	m.byKey[key] = clean
	return nil
}

func (m *memoryAreaPlacementRepository) DeleteByAreaLocaleInstance(_ context.Context, areaCode string, localeID *uuid.UUID, instanceID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := areaLocaleKey(areaCode, localeID)
	records := m.byKey[key]
	if len(records) == 0 {
		return nil
	}

	next := make([]*AreaPlacement, 0, len(records))
	for _, record := range records {
		if record.InstanceID == instanceID {
			delete(m.byID, record.ID)
			continue
		}
		next = append(next, record)
	}

	for idx, record := range next {
		record.Position = idx
	}

	m.byKey[key] = next
	return nil
}

func (m *memoryTranslationRepository) Update(_ context.Context, translation *Translation) (*Translation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.byID[translation.ID]
	if !ok {
		return nil, &NotFoundError{Resource: "widget_translation", Key: translation.ID.String()}
	}

	oldKey := translationKey(existing.WidgetInstanceID, existing.LocaleID)
	delete(m.byInstanceLocale, oldKey)

	if existing.WidgetInstanceID != translation.WidgetInstanceID {
		m.byInstance[existing.WidgetInstanceID] = removeUUID(m.byInstance[existing.WidgetInstanceID], translation.ID)
	}

	cloned := cloneTranslation(translation)
	m.byID[cloned.ID] = cloned

	newKey := translationKey(cloned.WidgetInstanceID, cloned.LocaleID)
	m.byInstanceLocale[newKey] = cloned.ID
	m.byInstance[cloned.WidgetInstanceID] = appendUniqueUUID(m.byInstance[cloned.WidgetInstanceID], cloned.ID)

	return cloneTranslation(cloned), nil
}

func cloneDefinition(src *Definition) *Definition {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.Schema = deepCloneMap(src.Schema)
	cloned.Defaults = deepCloneMap(src.Defaults)
	if src.Instances != nil {
		cloned.Instances = make([]*Instance, len(src.Instances))
		for i, inst := range src.Instances {
			cloned.Instances[i] = cloneInstance(inst)
		}
	}
	if src.Description != nil {
		value := *src.Description
		cloned.Description = &value
	}
	if src.Category != nil {
		value := *src.Category
		cloned.Category = &value
	}
	if src.Icon != nil {
		value := *src.Icon
		cloned.Icon = &value
	}
	return &cloned
}

func removeUUID(list []uuid.UUID, id uuid.UUID) []uuid.UUID {
	out := list[:0]
	for _, candidate := range list {
		if candidate != id {
			out = append(out, candidate)
		}
	}
	return append([]uuid.UUID(nil), out...)
}

func appendUniqueUUID(list []uuid.UUID, id uuid.UUID) []uuid.UUID {
	for _, candidate := range list {
		if candidate == id {
			return append([]uuid.UUID(nil), list...)
		}
	}
	return append(append([]uuid.UUID(nil), list...), id)
}

func cloneInstance(src *Instance) *Instance {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.Placement = deepCloneMap(src.Placement)
	cloned.Configuration = deepCloneMap(src.Configuration)
	cloned.VisibilityRules = deepCloneMap(src.VisibilityRules)
	if src.AreaCode != nil {
		area := *src.AreaCode
		cloned.AreaCode = &area
	}
	if src.BlockInstanceID != nil {
		blockID := *src.BlockInstanceID
		cloned.BlockInstanceID = &blockID
	}
	if src.Definition != nil {
		cloned.Definition = cloneDefinition(src.Definition)
	}
	if src.Translations != nil {
		cloned.Translations = make([]*Translation, len(src.Translations))
		for i, tr := range src.Translations {
			cloned.Translations[i] = cloneTranslation(tr)
		}
	}
	return &cloned
}

func cloneTranslation(src *Translation) *Translation {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.Content = deepCloneMap(src.Content)
	if src.Instance != nil {
		cloned.Instance = cloneInstance(src.Instance)
	}
	return &cloned
}

func cloneAreaDefinition(src *AreaDefinition) *AreaDefinition {
	if src == nil {
		return nil
	}
	cloned := *src
	if src.Description != nil {
		value := *src.Description
		cloned.Description = &value
	}
	if src.ThemeID != nil {
		value := *src.ThemeID
		cloned.ThemeID = &value
	}
	if src.TemplateID != nil {
		value := *src.TemplateID
		cloned.TemplateID = &value
	}
	return &cloned
}

func cloneAreaPlacement(src *AreaPlacement) *AreaPlacement {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.LocaleID = cloneUUIDPtr(src.LocaleID)
	cloned.Metadata = deepCloneMap(src.Metadata)
	if src.Instance != nil {
		cloned.Instance = cloneInstance(src.Instance)
	}
	return &cloned
}

func deepCloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	cloned := make(map[string]any, len(src))
	for key, value := range src {
		cloned[key] = deepCloneValue(value)
	}
	return cloned
}

func deepCloneSlice(src []any) []any {
	if src == nil {
		return nil
	}
	cloned := make([]any, len(src))
	for i, value := range src {
		cloned[i] = deepCloneValue(value)
	}
	return cloned
}

func deepCloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return deepCloneMap(typed)
	case []any:
		return deepCloneSlice(typed)
	case []string:
		return append([]string(nil), typed...)
	case []int:
		return append([]int(nil), typed...)
	case []map[string]any:
		result := make([]map[string]any, len(typed))
		for i, entry := range typed {
			result[i] = deepCloneMap(entry)
		}
		return result
	default:
		return typed
	}
}

func cloneUUIDPtr(src *uuid.UUID) *uuid.UUID {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}
