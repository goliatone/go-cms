package environments

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"
)

type memoryRepository struct {
	mu    sync.RWMutex
	byID  map[uuid.UUID]*Environment
	byKey map[string]uuid.UUID
}

// NewMemoryRepository constructs an in-memory environment repository.
func NewMemoryRepository() EnvironmentRepository {
	return &memoryRepository{
		byID:  make(map[uuid.UUID]*Environment),
		byKey: make(map[string]uuid.UUID),
	}
}

func (m *memoryRepository) Create(_ context.Context, env *Environment) (*Environment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneEnvironment(env)
	key := normalizeEnvironmentKey(cloned.Key)
	cloned.Key = key
	m.byID[cloned.ID] = cloned
	if key != "" {
		m.byKey[key] = cloned.ID
	}
	return cloneEnvironment(cloned), nil
}

func (m *memoryRepository) Update(_ context.Context, env *Environment) (*Environment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.byID[env.ID]
	if !ok {
		return nil, &NotFoundError{Resource: "environment", Key: env.ID.String()}
	}
	oldKey := normalizeEnvironmentKey(existing.Key)
	cloned := cloneEnvironment(env)
	newKey := normalizeEnvironmentKey(cloned.Key)
	cloned.Key = newKey
	m.byID[cloned.ID] = cloned

	if oldKey != "" && oldKey != newKey {
		delete(m.byKey, oldKey)
	}
	if newKey != "" {
		m.byKey[newKey] = cloned.ID
	}
	return cloneEnvironment(cloned), nil
}

func (m *memoryRepository) GetByID(_ context.Context, id uuid.UUID) (*Environment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.byID[id]
	if !ok {
		return nil, &NotFoundError{Resource: "environment", Key: id.String()}
	}
	return cloneEnvironment(record), nil
}

func (m *memoryRepository) GetByKey(_ context.Context, key string) (*Environment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalized := normalizeEnvironmentKey(key)
	id, ok := m.byKey[normalized]
	if !ok {
		return nil, &NotFoundError{Resource: "environment", Key: normalized}
	}
	return cloneEnvironment(m.byID[id]), nil
}

func (m *memoryRepository) List(_ context.Context) ([]*Environment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]*Environment, 0, len(m.byID))
	for _, record := range m.byID {
		records = append(records, cloneEnvironment(record))
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Key < records[j].Key
	})
	return records, nil
}

func (m *memoryRepository) ListActive(_ context.Context) ([]*Environment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]*Environment, 0, len(m.byID))
	for _, record := range m.byID {
		if record == nil || !record.IsActive {
			continue
		}
		records = append(records, cloneEnvironment(record))
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Key < records[j].Key
	})
	return records, nil
}

func (m *memoryRepository) GetDefault(_ context.Context) (*Environment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, record := range m.byID {
		if record != nil && record.IsDefault {
			return cloneEnvironment(record), nil
		}
	}
	return nil, &NotFoundError{Resource: "environment", Key: "default"}
}

func (m *memoryRepository) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.byID[id]
	if !ok {
		return &NotFoundError{Resource: "environment", Key: id.String()}
	}
	delete(m.byID, id)
	if record != nil {
		key := normalizeEnvironmentKey(record.Key)
		if key != "" {
			delete(m.byKey, key)
		}
	}
	return nil
}
