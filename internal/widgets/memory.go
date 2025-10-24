package widgets

import (
	"context"
	"maps"
	"sync"

	"github.com/google/uuid"
)

// NewMemoryDefinitionRepository constructs an "in memory" widget definition repository.
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

func cloneDefinition(src *Definition) *Definition {
	if src == nil {
		return nil
	}
	cloned := *src
	if src.Schema != nil {
		cloned.Schema = maps.Clone(src.Schema)
	}
	if src.Defaults != nil {
		cloned.Defaults = maps.Clone(src.Defaults)
	}
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

func cloneInstance(src *Instance) *Instance {
	if src == nil {
		return nil
	}
	cloned := *src
	if src.Placement != nil {
		cloned.Placement = maps.Clone(src.Placement)
	}
	if src.Configuration != nil {
		cloned.Configuration = maps.Clone(src.Configuration)
	}
	if src.VisibilityRules != nil {
		cloned.VisibilityRules = maps.Clone(src.VisibilityRules)
	}
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
	if src.Content != nil {
		cloned.Content = maps.Clone(src.Content)
	}
	if src.Instance != nil {
		cloned.Instance = cloneInstance(src.Instance)
	}
	return &cloned
}
