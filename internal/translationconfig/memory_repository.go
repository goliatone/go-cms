package translationconfig

import (
	"context"
	"sync"
)

// MemoryRepository stores translation settings in-memory.
type MemoryRepository struct {
	mu          sync.RWMutex
	settings    *Settings
	broadcaster *changeBroadcaster
}

// NewMemoryRepository constructs an in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		broadcaster: newChangeBroadcaster(),
	}
}

// Get returns the stored settings or ErrSettingsNotFound.
func (r *MemoryRepository) Get(context.Context) (Settings, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.settings == nil {
		return Settings{}, ErrSettingsNotFound
	}
	return *r.settings, nil
}

// Upsert stores settings, emitting a change event.
func (r *MemoryRepository) Upsert(_ context.Context, settings Settings) (Settings, error) {
	r.mu.Lock()
	created := r.settings == nil
	previous := Settings{}
	if r.settings != nil {
		previous = *r.settings
	}
	copied := settings
	r.settings = &copied
	r.mu.Unlock()

	if !created && previous == settings {
		return settings, nil
	}
	changeType := ChangeUpdated
	if created {
		changeType = ChangeCreated
	}
	r.broadcaster.Broadcast(newChangeEvent(changeType, settings))
	return settings, nil
}

// Delete clears stored settings and emits a change event.
func (r *MemoryRepository) Delete(context.Context) error {
	r.mu.Lock()
	if r.settings == nil {
		r.mu.Unlock()
		return ErrSettingsNotFound
	}
	r.settings = nil
	r.mu.Unlock()

	r.broadcaster.Broadcast(newChangeEvent(ChangeDeleted, Settings{}))
	return nil
}

// Subscribe delivers change events until the context is cancelled.
func (r *MemoryRepository) Subscribe(ctx context.Context) (<-chan ChangeEvent, error) {
	return r.broadcaster.Subscribe(ctx)
}
