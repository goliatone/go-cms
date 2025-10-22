package pages

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// MemoryPageRepository is an in-memory page store for scaffolding/tests.
type MemoryPageRepository struct {
	mu        sync.RWMutex
	pages     map[uuid.UUID]*Page
	slugIndex map[string]uuid.UUID
}

// NewMemoryPageRepository constructs the repository.
func NewMemoryPageRepository() *MemoryPageRepository {
	return &MemoryPageRepository{
		pages:     make(map[uuid.UUID]*Page),
		slugIndex: make(map[string]uuid.UUID),
	}
}

// Create inserts the supplied page.
func (m *MemoryPageRepository) Create(_ context.Context, record *Page) (*Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := clonePage(record)
	m.pages[copied.ID] = copied
	m.slugIndex[copied.Slug] = copied.ID
	return clonePage(copied), nil
}

// GetByID retrieves a page by identifier.
func (m *MemoryPageRepository) GetByID(_ context.Context, id uuid.UUID) (*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	page, ok := m.pages[id]
	if !ok {
		return nil, &PageNotFoundError{Key: id.String()}
	}
	return clonePage(page), nil
}

// GetBySlug retrieves a page by slug.
func (m *MemoryPageRepository) GetBySlug(_ context.Context, slug string) (*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.slugIndex[slug]
	if !ok {
		return nil, &PageNotFoundError{Key: slug}
	}
	return clonePage(m.pages[id]), nil
}

// List returns every page.
func (m *MemoryPageRepository) List(_ context.Context) ([]*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Page, 0, len(m.pages))
	for _, record := range m.pages {
		out = append(out, clonePage(record))
	}
	return out, nil
}

func clonePage(src *Page) *Page {
	if src == nil {
		return nil
	}
	copied := *src
	if len(src.Translations) > 0 {
		copied.Translations = make([]*PageTranslation, len(src.Translations))
		for i, tr := range src.Translations {
			if tr == nil {
				continue
			}
			local := *tr
			copied.Translations[i] = &local
		}
	}
	if len(src.Versions) > 0 {
		copied.Versions = make([]*PageVersion, len(src.Versions))
		for i, v := range src.Versions {
			if v == nil {
				continue
			}
			local := *v
			if v.Snapshot != nil {
				local.Snapshot = cloneMap(v.Snapshot)
			}
			copied.Versions[i] = &local
		}
	}
	return &copied
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
