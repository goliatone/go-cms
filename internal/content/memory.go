package content

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// MemoryContentRepository is an in-memory implementation for scaffolding and tests.
type MemoryContentRepository struct {
	mu        sync.RWMutex
	contents  map[uuid.UUID]*Content
	slugIndex map[string]uuid.UUID
}

// NewMemoryContentRepository creates an empty in-memory content repository.
func NewMemoryContentRepository() *MemoryContentRepository {
	return &MemoryContentRepository{
		contents:  make(map[uuid.UUID]*Content),
		slugIndex: make(map[string]uuid.UUID),
	}
}

// Create inserts the supplied content.
func (m *MemoryContentRepository) Create(_ context.Context, record *Content) (*Content, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	copied := cloneContent(record)
	m.contents[copied.ID] = copied
	m.slugIndex[copied.Slug] = copied.ID
	return cloneContent(copied), nil
}

// GetByID retrieves content by identifier.
func (m *MemoryContentRepository) GetByID(_ context.Context, id uuid.UUID) (*Content, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, ok := m.contents[id]
	if !ok {
		return nil, &NotFoundError{Resource: "content", Key: id.String()}
	}
	return cloneContent(rec), nil
}

// GetBySlug retrieves content by slug, returning NotFoundError when absent.
func (m *MemoryContentRepository) GetBySlug(_ context.Context, slug string) (*Content, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.slugIndex[slug]
	if !ok {
		return nil, &NotFoundError{Resource: "content", Key: slug}
	}
	return cloneContent(m.contents[id]), nil
}

// List returns all content entries.
func (m *MemoryContentRepository) List(_ context.Context) ([]*Content, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Content, 0, len(m.contents))
	for _, rec := range m.contents {
		out = append(out, cloneContent(rec))
	}
	return out, nil
}

func cloneContent(src *Content) *Content {
	if src == nil {
		return nil
	}

	copied := *src
	if len(src.Translations) > 0 {
		copied.Translations = make([]*ContentTranslation, len(src.Translations))
		for i, tr := range src.Translations {
			if tr == nil {
				continue
			}
			local := *tr
			local.Content = cloneMap(tr.Content)
			copied.Translations[i] = &local
		}
	}
	return &copied
}

// MemoryContentTypeRepository stores content types in-memory.
type MemoryContentTypeRepository struct {
	mu    sync.RWMutex
	types map[uuid.UUID]*ContentType
}

// NewMemoryContentTypeRepository constructs the repository.
func NewMemoryContentTypeRepository() *MemoryContentTypeRepository {
	return &MemoryContentTypeRepository{
		types: make(map[uuid.UUID]*ContentType),
	}
}

// Put inserts or replaces a content type.
func (m *MemoryContentTypeRepository) Put(ct *ContentType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := *ct
	m.types[ct.ID] = &copied
}

// GetByID fetches a content type.
func (m *MemoryContentTypeRepository) GetByID(_ context.Context, id uuid.UUID) (*ContentType, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ct, ok := m.types[id]
	if !ok {
		return nil, &NotFoundError{Resource: "content_type", Key: id.String()}
	}
	copied := *ct
	if ct.Capabilities != nil {
		copied.Capabilities = cloneMap(ct.Capabilities)
	}
	if ct.Schema != nil {
		copied.Schema = cloneMap(ct.Schema)
	}
	return &copied, nil
}

// MemoryLocaleRepository stores locales by code.
type MemoryLocaleRepository struct {
	mu      sync.RWMutex
	locales map[string]*Locale
}

// NewMemoryLocaleRepository constructs the repository.
func NewMemoryLocaleRepository() *MemoryLocaleRepository {
	return &MemoryLocaleRepository{
		locales: make(map[string]*Locale),
	}
}

// Put inserts or replaces a locale.
func (m *MemoryLocaleRepository) Put(locale *Locale) {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := *locale
	m.locales[strings.ToLower(locale.Code)] = &copied
}

// GetByCode resolves a locale by code (case-insensitive).
func (m *MemoryLocaleRepository) GetByCode(_ context.Context, code string) (*Locale, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	loc, ok := m.locales[strings.ToLower(code)]
	if !ok {
		return nil, &NotFoundError{Resource: "locale", Key: code}
	}
	copied := *loc
	return &copied, nil
}
