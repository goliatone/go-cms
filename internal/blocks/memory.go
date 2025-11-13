package blocks

import (
	"context"
	"maps"
	"sync"
	"time"

	"github.com/goliatone/go-cms/internal/media"
	"github.com/google/uuid"
)

// NewMemoryDefinitionRepository constructs an "in memory" definition repository.
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
		return nil, &NotFoundError{Resource: "block_definition", Key: id.String()}
	}
	return cloneDefinition(record), nil
}

func (m *memoryDefinitionRepository) GetByName(_ context.Context, name string) (*Definition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.byName[name]
	if !ok {
		return nil, &NotFoundError{Resource: "block_definition", Key: name}
	}
	return cloneDefinition(m.byID[id]), nil
}

func (m *memoryDefinitionRepository) List(_ context.Context) ([]*Definition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	defs := make([]*Definition, 0, len(m.byID))
	for _, def := range m.byID {
		defs = append(defs, cloneDefinition(def))
	}
	return defs, nil
}

func (m *memoryDefinitionRepository) Update(_ context.Context, definition *Definition) (*Definition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.byID[definition.ID]; !ok {
		return nil, &NotFoundError{Resource: "block_definition", Key: definition.ID.String()}
	}

	cloned := cloneDefinition(definition)
	m.byID[cloned.ID] = cloned
	if cloned.Name != "" {
		m.byName[cloned.Name] = cloned.ID
	}
	return cloneDefinition(cloned), nil
}

func (m *memoryDefinitionRepository) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.byID[id]
	if !ok {
		return &NotFoundError{Resource: "block_definition", Key: id.String()}
	}
	delete(m.byID, id)
	if record.Name != "" {
		delete(m.byName, record.Name)
	}
	return nil
}

// NewMemoryInstanceRepository constructs an "in memory" instance repository.
func NewMemoryInstanceRepository() InstanceRepository {
	return &memoryInstanceRepository{
		byID:      make(map[uuid.UUID]*Instance),
		byPageID:  make(map[uuid.UUID][]uuid.UUID),
		globalIDs: make([]uuid.UUID, 0),
	}
}

type memoryInstanceRepository struct {
	mu        sync.RWMutex
	byID      map[uuid.UUID]*Instance
	byPageID  map[uuid.UUID][]uuid.UUID
	globalIDs []uuid.UUID
}

func (m *memoryInstanceRepository) Create(_ context.Context, instance *Instance) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneInstance(instance)
	m.byID[cloned.ID] = cloned
	if cloned.PageID != nil {
		idList := append([]uuid.UUID{}, m.byPageID[*cloned.PageID]...)
		idList = append(idList, cloned.ID)
		m.byPageID[*cloned.PageID] = idList
	}
	if cloned.IsGlobal {
		m.globalIDs = append(m.globalIDs, cloned.ID)
	}
	return cloneInstance(cloned), nil
}

func (m *memoryInstanceRepository) GetByID(_ context.Context, id uuid.UUID) (*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.byID[id]
	if !ok {
		return nil, &NotFoundError{Resource: "block_instance", Key: id.String()}
	}
	return cloneInstance(record), nil
}

func (m *memoryInstanceRepository) ListByPage(_ context.Context, pageID uuid.UUID) ([]*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byPageID[pageID]
	instances := make([]*Instance, 0, len(ids))
	for _, id := range ids {
		instances = append(instances, cloneInstance(m.byID[id]))
	}
	return instances, nil
}

func (m *memoryInstanceRepository) ListGlobal(_ context.Context) ([]*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instances := make([]*Instance, 0, len(m.globalIDs))
	for _, id := range m.globalIDs {
		instances = append(instances, cloneInstance(m.byID[id]))
	}
	return instances, nil
}

func (m *memoryInstanceRepository) ListByDefinition(_ context.Context, definitionID uuid.UUID) ([]*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Instance, 0)
	for _, instance := range m.byID {
		if instance.DefinitionID == definitionID {
			out = append(out, cloneInstance(instance))
		}
	}
	return out, nil
}

func (m *memoryInstanceRepository) Update(_ context.Context, instance *Instance) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.byID[instance.ID]
	if !ok {
		return nil, &NotFoundError{Resource: "block_instance", Key: instance.ID.String()}
	}

	updated := cloneInstance(current)
	if instance.PageID != nil {
		pageID := *instance.PageID
		updated.PageID = &pageID
	} else {
		updated.PageID = nil
	}
	if current.PageID != nil && (updated.PageID == nil || *current.PageID != *updated.PageID) {
		m.byPageID[*current.PageID] = removeUUID(m.byPageID[*current.PageID], instance.ID)
	}
	if updated.PageID != nil {
		m.byPageID[*updated.PageID] = appendUniqueUUID(m.byPageID[*updated.PageID], instance.ID)
	}

	updated.Region = instance.Region
	updated.Position = instance.Position
	updated.Configuration = maps.Clone(instance.Configuration)
	updated.IsGlobal = instance.IsGlobal
	if updated.IsGlobal {
		m.globalIDs = appendUniqueUUID(m.globalIDs, instance.ID)
	} else {
		m.globalIDs = removeUUID(m.globalIDs, instance.ID)
	}
	updated.CurrentVersion = instance.CurrentVersion
	updated.PublishedVersion = cloneIntPointer(instance.PublishedVersion)
	updated.PublishedAt = cloneTimePointer(instance.PublishedAt)
	updated.PublishedBy = cloneUUIDPointer(instance.PublishedBy)
	updated.UpdatedAt = instance.UpdatedAt
	updated.UpdatedBy = instance.UpdatedBy

	m.byID[instance.ID] = updated
	return cloneInstance(updated), nil
}

func (m *memoryInstanceRepository) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, ok := m.byID[id]
	if !ok {
		return &NotFoundError{Resource: "block_instance", Key: id.String()}
	}

	if instance.PageID != nil {
		pageID := *instance.PageID
		m.byPageID[pageID] = removeUUID(m.byPageID[pageID], id)
	}
	m.globalIDs = removeUUID(m.globalIDs, id)
	delete(m.byID, id)
	return nil
}

// NewMemoryInstanceVersionRepository constructs an "in memory" instance version repository.
func NewMemoryInstanceVersionRepository() InstanceVersionRepository {
	return &memoryInstanceVersionRepository{
		byInstance: make(map[uuid.UUID][]*InstanceVersion),
	}
}

type memoryInstanceVersionRepository struct {
	mu         sync.RWMutex
	byInstance map[uuid.UUID][]*InstanceVersion
}

func (m *memoryInstanceVersionRepository) Create(_ context.Context, version *InstanceVersion) (*InstanceVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneInstanceVersion(version)
	queue := append([]*InstanceVersion{}, m.byInstance[cloned.BlockInstanceID]...)
	queue = append(queue, cloned)
	m.byInstance[cloned.BlockInstanceID] = queue
	return cloneInstanceVersion(cloned), nil
}

func (m *memoryInstanceVersionRepository) ListByInstance(_ context.Context, instanceID uuid.UUID) ([]*InstanceVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return cloneInstanceVersions(m.byInstance[instanceID]), nil
}

func (m *memoryInstanceVersionRepository) GetVersion(_ context.Context, instanceID uuid.UUID, number int) (*InstanceVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, version := range m.byInstance[instanceID] {
		if version.Version == number {
			return cloneInstanceVersion(version), nil
		}
	}
	return nil, &NotFoundError{Resource: "block_version", Key: versionKey(instanceID, number)}
}

func (m *memoryInstanceVersionRepository) GetLatest(_ context.Context, instanceID uuid.UUID) (*InstanceVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queue := m.byInstance[instanceID]
	if len(queue) == 0 {
		return nil, &NotFoundError{Resource: "block_version", Key: instanceID.String()}
	}
	return cloneInstanceVersion(queue[len(queue)-1]), nil
}

func (m *memoryInstanceVersionRepository) Update(_ context.Context, version *InstanceVersion) (*InstanceVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue := m.byInstance[version.BlockInstanceID]
	for idx, existing := range queue {
		if existing == nil {
			continue
		}
		if existing.ID == version.ID {
			queue[idx] = cloneInstanceVersion(version)
			m.byInstance[version.BlockInstanceID] = queue
			return cloneInstanceVersion(queue[idx]), nil
		}
	}
	return nil, &NotFoundError{Resource: "block_version", Key: versionKey(version.BlockInstanceID, version.Version)}
}

// NewMemoryTranslationRepository constructs an "in memory" translation repository.
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
	key := translationKey(cloned.BlockInstanceID, cloned.LocaleID)
	m.byInstanceLocale[key] = cloned.ID
	m.byInstance[cloned.BlockInstanceID] = append(m.byInstance[cloned.BlockInstanceID], cloned.ID)
	return cloneTranslation(cloned), nil
}

func (m *memoryTranslationRepository) GetByInstanceAndLocale(_ context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.byInstanceLocale[translationKey(instanceID, localeID)]
	if !ok {
		return nil, &NotFoundError{Resource: "block_translation", Key: translationKey(instanceID, localeID)}
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

func (m *memoryTranslationRepository) Update(_ context.Context, translation *Translation) (*Translation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.byID[translation.ID]
	if !ok {
		return nil, &NotFoundError{Resource: "block_translation", Key: translation.ID.String()}
	}

	oldKey := translationKey(existing.BlockInstanceID, existing.LocaleID)

	cloned := cloneTranslation(translation)
	m.byID[cloned.ID] = cloned

	newKey := translationKey(cloned.BlockInstanceID, cloned.LocaleID)
	if oldKey != newKey {
		delete(m.byInstanceLocale, oldKey)
		m.byInstanceLocale[newKey] = cloned.ID
		m.byInstance[existing.BlockInstanceID] = removeUUID(m.byInstance[existing.BlockInstanceID], cloned.ID)
		m.byInstance[cloned.BlockInstanceID] = appendUniqueUUID(m.byInstance[cloned.BlockInstanceID], cloned.ID)
	}

	return cloneTranslation(cloned), nil
}

func (m *memoryTranslationRepository) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	translation, ok := m.byID[id]
	if !ok {
		return &NotFoundError{Resource: "block_translation", Key: id.String()}
	}
	delete(m.byID, id)

	key := translationKey(translation.BlockInstanceID, translation.LocaleID)
	delete(m.byInstanceLocale, key)
	m.byInstance[translation.BlockInstanceID] = removeUUID(m.byInstance[translation.BlockInstanceID], id)
	return nil
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
	return &cloned
}

func cloneInstance(src *Instance) *Instance {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.PublishedVersion = cloneIntPointer(src.PublishedVersion)
	cloned.PublishedAt = cloneTimePointer(src.PublishedAt)
	cloned.PublishedBy = cloneUUIDPointer(src.PublishedBy)
	if src.Configuration != nil {
		cloned.Configuration = maps.Clone(src.Configuration)
	}
	if src.Translations != nil {
		cloned.Translations = make([]*Translation, len(src.Translations))
		for i, tr := range src.Translations {
			cloned.Translations[i] = cloneTranslation(tr)
		}
	}
	if src.Versions != nil {
		cloned.Versions = cloneInstanceVersions(src.Versions)
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
	if src.AttributeOverride != nil {
		cloned.AttributeOverride = maps.Clone(src.AttributeOverride)
	}
	cloned.MediaBindings = media.CloneBindingSet(src.MediaBindings)
	cloned.ResolvedMedia = media.CloneAttachments(src.ResolvedMedia)
	return &cloned
}

func translationKey(instanceID uuid.UUID, localeID uuid.UUID) string {
	return instanceID.String() + ":" + localeID.String()
}

func cloneInstanceVersions(src []*InstanceVersion) []*InstanceVersion {
	if len(src) == 0 {
		return nil
	}
	out := make([]*InstanceVersion, len(src))
	for i, version := range src {
		out[i] = cloneInstanceVersion(version)
	}
	return out
}

func cloneInstanceVersion(src *InstanceVersion) *InstanceVersion {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.Snapshot = cloneBlockVersionSnapshot(src.Snapshot)
	cloned.PublishedAt = cloneTimePointer(src.PublishedAt)
	cloned.PublishedBy = cloneUUIDPointer(src.PublishedBy)
	return &cloned
}

func cloneBlockVersionSnapshot(src BlockVersionSnapshot) BlockVersionSnapshot {
	target := BlockVersionSnapshot{
		Configuration: maps.Clone(src.Configuration),
		Metadata:      maps.Clone(src.Metadata),
	}
	if len(src.Translations) > 0 {
		target.Translations = make([]BlockVersionTranslationSnapshot, len(src.Translations))
		for i, tr := range src.Translations {
			target.Translations[i] = BlockVersionTranslationSnapshot{
				Locale:             tr.Locale,
				Content:            maps.Clone(tr.Content),
				AttributeOverrides: maps.Clone(tr.AttributeOverrides),
			}
		}
	}
	target.Media = media.CloneBindingSet(src.Media)
	return target
}

func cloneIntPointer(src *int) *int {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}

func cloneTimePointer(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}

func cloneUUIDPointer(src *uuid.UUID) *uuid.UUID {
	if src == nil {
		return nil
	}
	value := *src
	return &value
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
