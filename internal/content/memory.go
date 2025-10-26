package content

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// MemoryContentRepository is an "in memory" implementation for scaffolding and tests.
type MemoryContentRepository struct {
	mu        sync.RWMutex
	contents  map[uuid.UUID]*Content
	slugIndex map[string]uuid.UUID
	versions  map[uuid.UUID][]*ContentVersion
}

// NewMemoryContentRepository creates an empty "in memory" content repository.
func NewMemoryContentRepository() *MemoryContentRepository {
	return &MemoryContentRepository{
		contents:  make(map[uuid.UUID]*Content),
		slugIndex: make(map[string]uuid.UUID),
		versions:  make(map[uuid.UUID][]*ContentVersion),
	}
}

// Create inserts the supplied content.
func (m *MemoryContentRepository) Create(_ context.Context, record *Content) (*Content, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	copied := cloneContent(record)
	if len(copied.Versions) > 0 {
		m.versions[copied.ID] = cloneContentVersions(copied.Versions)
	} else {
		m.versions[copied.ID] = nil
	}
	m.contents[copied.ID] = copied
	m.slugIndex[copied.Slug] = copied.ID
	return m.attachVersions(cloneContent(copied)), nil
}

// GetByID retrieves content by identifier.
func (m *MemoryContentRepository) GetByID(_ context.Context, id uuid.UUID) (*Content, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, ok := m.contents[id]
	if !ok {
		return nil, &NotFoundError{Resource: "content", Key: id.String()}
	}
	return m.attachVersions(cloneContent(rec)), nil
}

// GetBySlug retrieves content by slug, returning NotFoundError when absent.
func (m *MemoryContentRepository) GetBySlug(_ context.Context, slug string) (*Content, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.slugIndex[slug]
	if !ok {
		return nil, &NotFoundError{Resource: "content", Key: slug}
	}
	return m.attachVersions(cloneContent(m.contents[id])), nil
}

// List returns all content entries.
func (m *MemoryContentRepository) List(_ context.Context) ([]*Content, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Content, 0, len(m.contents))
	for _, rec := range m.contents {
		out = append(out, m.attachVersions(cloneContent(rec)))
	}
	return out, nil
}

// CreateVersion appends a new version snapshot for the supplied content entity.
func (m *MemoryContentRepository) CreateVersion(_ context.Context, version *ContentVersion) (*ContentVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := cloneContentVersion(version)
	existing := append([]*ContentVersion{}, m.versions[cloned.ContentID]...)
	existing = append(existing, cloned)
	m.versions[cloned.ContentID] = existing
	return cloneContentVersion(cloned), nil
}

// ListVersions returns every stored version for a content entity.
func (m *MemoryContentRepository) ListVersions(_ context.Context, contentID uuid.UUID) ([]*ContentVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queue := m.versions[contentID]
	return cloneContentVersions(queue), nil
}

// GetVersion retrieves a specific content version by number.
func (m *MemoryContentRepository) GetVersion(_ context.Context, contentID uuid.UUID, number int) (*ContentVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, version := range m.versions[contentID] {
		if version.Version == number {
			return cloneContentVersion(version), nil
		}
	}
	return nil, &NotFoundError{Resource: "content_version", Key: contentID.String()}
}

// GetLatestVersion retrieves the most recent version for a content entity.
func (m *MemoryContentRepository) GetLatestVersion(_ context.Context, contentID uuid.UUID) (*ContentVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queue := m.versions[contentID]
	if len(queue) == 0 {
		return nil, &NotFoundError{Resource: "content_version", Key: contentID.String()}
	}
	last := queue[len(queue)-1]
	return cloneContentVersion(last), nil
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
	if len(src.Versions) > 0 {
		copied.Versions = cloneContentVersions(src.Versions)
	}
	return &copied
}

func (m *MemoryContentRepository) attachVersions(content *Content) *Content {
	if content == nil {
		return nil
	}
	content.Versions = cloneContentVersions(m.versions[content.ID])
	return content
}

func cloneContentVersions(src []*ContentVersion) []*ContentVersion {
	if len(src) == 0 {
		return nil
	}
	out := make([]*ContentVersion, len(src))
	for i, record := range src {
		out[i] = cloneContentVersion(record)
	}
	return out
}

func cloneContentVersion(src *ContentVersion) *ContentVersion {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.Snapshot = cloneContentVersionSnapshot(src.Snapshot)
	return &cloned
}

func cloneContentVersionSnapshot(src ContentVersionSnapshot) ContentVersionSnapshot {
	target := ContentVersionSnapshot{
		Fields:   cloneMap(src.Fields),
		Metadata: cloneMap(src.Metadata),
	}
	if len(src.Translations) > 0 {
		target.Translations = make([]ContentVersionTranslationSnapshot, len(src.Translations))
		for i, tr := range src.Translations {
			target.Translations[i] = ContentVersionTranslationSnapshot{
				Locale:  tr.Locale,
				Title:   tr.Title,
				Summary: tr.Summary,
				Content: cloneMap(tr.Content),
			}
		}
	}
	return target
}

// MemoryContentTypeRepository stores content types "in memory".
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
