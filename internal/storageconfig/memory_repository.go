package storageconfig

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/goliatone/go-cms/pkg/storage"
)

// MemoryRepository stores profiles in-memory for tests and lightweight deployments.
type MemoryRepository struct {
	mu          sync.RWMutex
	profiles    map[string]storage.Profile
	broadcaster *changeBroadcaster
}

// NewMemoryRepository constructs an empty in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		profiles:    make(map[string]storage.Profile),
		broadcaster: newChangeBroadcaster(),
	}
}

// List returns the stored profiles ordered by name.
func (r *MemoryRepository) List(context.Context) ([]storage.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]storage.Profile, 0, len(r.profiles))
	for _, profile := range r.profiles {
		out = append(out, cloneProfile(profile))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Get retrieves a profile by name.
func (r *MemoryRepository) Get(_ context.Context, name string) (*storage.Profile, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, ErrProfileNameRequired
	}

	r.mu.RLock()
	profile, ok := r.profiles[trimmed]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrProfileNotFound
	}
	cloned := cloneProfile(profile)
	return &cloned, nil
}

// Upsert creates or updates a profile.
func (r *MemoryRepository) Upsert(_ context.Context, profile storage.Profile) (*storage.Profile, error) {
	name := strings.TrimSpace(profile.Name)
	if name == "" {
		return nil, ErrProfileNameRequired
	}
	profile.Name = name
	stored := cloneProfile(profile)

	r.mu.Lock()
	_, exists := r.profiles[name]
	r.profiles[name] = stored
	r.mu.Unlock()

	eventType := ChangeCreated
	if exists {
		eventType = ChangeUpdated
	}
	r.broadcaster.Broadcast(newChangeEvent(eventType, stored))

	result := cloneProfile(stored)
	return &result, nil
}

// Delete removes a profile by name.
func (r *MemoryRepository) Delete(_ context.Context, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ErrProfileNameRequired
	}

	r.mu.Lock()
	profile, ok := r.profiles[trimmed]
	if !ok {
		r.mu.Unlock()
		return ErrProfileNotFound
	}
	delete(r.profiles, trimmed)
	r.mu.Unlock()

	r.broadcaster.Broadcast(newChangeEvent(ChangeDeleted, profile))
	return nil
}

// Subscribe delivers change events until the context is cancelled.
func (r *MemoryRepository) Subscribe(ctx context.Context) (<-chan ChangeEvent, error) {
	return r.broadcaster.Subscribe(ctx)
}
