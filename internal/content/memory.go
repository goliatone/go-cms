package content

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrContentTypeSlugRequired = errors.New("content type: slug is required")
	ErrContentTypeSlugExists   = errors.New("content type: slug already exists")
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

// Update persists metadata changes for content records.
func (m *MemoryContentRepository) Update(_ context.Context, record *Content) (*Content, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.contents[record.ID]
	if !ok {
		return nil, &NotFoundError{Resource: "content", Key: record.ID.String()}
	}

	updated := cloneContent(current)
	updated.CurrentVersion = record.CurrentVersion
	updated.PublishedVersion = cloneIntPointer(record.PublishedVersion)
	updated.PublishAt = cloneTimePointer(record.PublishAt)
	updated.UnpublishAt = cloneTimePointer(record.UnpublishAt)
	updated.PublishedAt = cloneTimePointer(record.PublishedAt)
	updated.PublishedBy = cloneUUIDPointer(record.PublishedBy)
	updated.Status = record.Status
	updated.UpdatedAt = record.UpdatedAt
	updated.UpdatedBy = record.UpdatedBy
	if len(record.Translations) > 0 {
		updated.Translations = cloneContentTranslations(record.Translations)
	}

	m.contents[record.ID] = updated
	return m.attachVersions(cloneContent(updated)), nil
}

// ReplaceTranslations swaps the translations associated with a content record.
func (m *MemoryContentRepository) ReplaceTranslations(_ context.Context, contentID uuid.UUID, translations []*ContentTranslation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.contents[contentID]
	if !ok {
		return &NotFoundError{Resource: "content", Key: contentID.String()}
	}
	record.Translations = cloneContentTranslations(translations)
	return nil
}

// ListTranslations returns stored translations for a content entry.
func (m *MemoryContentRepository) ListTranslations(_ context.Context, contentID uuid.UUID) ([]*ContentTranslation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.contents[contentID]
	if !ok {
		return nil, &NotFoundError{Resource: "content", Key: contentID.String()}
	}
	return cloneContentTranslations(record.Translations), nil
}

// Delete removes the content record and its associated versions when hard delete is requested.
func (m *MemoryContentRepository) Delete(_ context.Context, id uuid.UUID, hardDelete bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.contents[id]
	if !ok {
		return &NotFoundError{Resource: "content", Key: id.String()}
	}
	if !hardDelete {
		return errors.New("memory content repository: soft delete not supported")
	}

	delete(m.contents, id)
	if slug := strings.TrimSpace(record.Slug); slug != "" {
		delete(m.slugIndex, slug)
	}
	delete(m.versions, id)

	return nil
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

// UpdateVersion mutates metadata for a stored content version.
func (m *MemoryContentRepository) UpdateVersion(_ context.Context, version *ContentVersion) (*ContentVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue := m.versions[version.ContentID]
	for idx, existing := range queue {
		if existing == nil {
			continue
		}
		if existing.ID == version.ID {
			queue[idx] = cloneContentVersion(version)
			m.versions[version.ContentID] = queue
			return cloneContentVersion(queue[idx]), nil
		}
	}
	return nil, &NotFoundError{Resource: "content_version", Key: version.ContentID.String()}
}

func cloneContent(src *Content) *Content {
	if src == nil {
		return nil
	}

	copied := *src
	copied.Translations = cloneContentTranslations(src.Translations)
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

func cloneContentTranslations(src []*ContentTranslation) []*ContentTranslation {
	if len(src) == 0 {
		return nil
	}
	out := make([]*ContentTranslation, len(src))
	for i, tr := range src {
		if tr == nil {
			continue
		}
		cloned := *tr
		cloned.Content = cloneMap(tr.Content)
		out[i] = &cloned
	}
	return out
}

func cloneContentVersion(src *ContentVersion) *ContentVersion {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.Snapshot = cloneContentVersionSnapshot(src.Snapshot)
	cloned.PublishedAt = cloneTimePointer(src.PublishedAt)
	cloned.PublishedBy = cloneUUIDPointer(src.PublishedBy)
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

// MemoryContentTypeRepository stores content types "in memory".
type MemoryContentTypeRepository struct {
	mu        sync.RWMutex
	types     map[uuid.UUID]*ContentType
	slugIndex map[string]uuid.UUID
}

// NewMemoryContentTypeRepository constructs the repository.
func NewMemoryContentTypeRepository() *MemoryContentTypeRepository {
	return &MemoryContentTypeRepository{
		types:     make(map[uuid.UUID]*ContentType),
		slugIndex: make(map[string]uuid.UUID),
	}
}

// Create inserts a content type record.
func (m *MemoryContentTypeRepository) Create(ctx context.Context, ct *ContentType) (*ContentType, error) {
	if err := m.Put(ct); err != nil {
		return nil, err
	}
	return m.GetByID(ctx, ct.ID)
}

// Put inserts or replaces a content type.
func (m *MemoryContentTypeRepository) Put(ct *ContentType) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ct == nil {
		return ErrContentTypeSlugRequired
	}

	slug := strings.TrimSpace(DeriveContentTypeSlug(ct))
	if slug == "" {
		return ErrContentTypeSlugRequired
	}

	if existingID, ok := m.slugIndex[slug]; ok && existingID != ct.ID {
		return ErrContentTypeSlugExists
	}

	if existing, ok := m.types[ct.ID]; ok && existing != nil {
		oldSlug := strings.TrimSpace(existing.Slug)
		if oldSlug != "" && oldSlug != slug {
			delete(m.slugIndex, oldSlug)
		}
	}

	copied := *ct
	copied.Slug = slug
	m.types[ct.ID] = &copied
	m.slugIndex[slug] = ct.ID
	return nil
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

// GetBySlug fetches a content type by slug.
func (m *MemoryContentTypeRepository) GetBySlug(_ context.Context, slug string) (*ContentType, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := strings.TrimSpace(slug)
	if key == "" {
		return nil, &NotFoundError{Resource: "content_type", Key: slug}
	}

	id, ok := m.slugIndex[key]
	if !ok {
		return nil, &NotFoundError{Resource: "content_type", Key: slug}
	}

	ct, ok := m.types[id]
	if !ok {
		return nil, &NotFoundError{Resource: "content_type", Key: slug}
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

// List returns all content types.
func (m *MemoryContentTypeRepository) List(_ context.Context) ([]*ContentType, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]*ContentType, 0, len(m.types))
	for _, ct := range m.types {
		if ct == nil {
			continue
		}
		copied := *ct
		if ct.Capabilities != nil {
			copied.Capabilities = cloneMap(ct.Capabilities)
		}
		if ct.Schema != nil {
			copied.Schema = cloneMap(ct.Schema)
		}
		records = append(records, &copied)
	}

	sort.Slice(records, func(i, j int) bool {
		return strings.ToLower(records[i].Name) < strings.ToLower(records[j].Name)
	})
	return records, nil
}

// Search returns content types whose name or slug contains the query.
func (m *MemoryContentTypeRepository) Search(ctx context.Context, query string) ([]*ContentType, error) {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return m.List(ctx)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]*ContentType, 0)
	for _, ct := range m.types {
		if ct == nil {
			continue
		}
		name := strings.ToLower(ct.Name)
		slug := strings.ToLower(ct.Slug)
		if strings.Contains(name, query) || strings.Contains(slug, query) {
			copied := *ct
			if ct.Capabilities != nil {
				copied.Capabilities = cloneMap(ct.Capabilities)
			}
			if ct.Schema != nil {
				copied.Schema = cloneMap(ct.Schema)
			}
			records = append(records, &copied)
		}
	}

	sort.Slice(records, func(i, j int) bool {
		return strings.ToLower(records[i].Name) < strings.ToLower(records[j].Name)
	})
	return records, nil
}

// Update updates an existing content type.
func (m *MemoryContentTypeRepository) Update(ctx context.Context, ct *ContentType) (*ContentType, error) {
	m.mu.RLock()
	_, ok := m.types[ct.ID]
	m.mu.RUnlock()
	if !ok {
		return nil, &NotFoundError{Resource: "content_type", Key: ct.ID.String()}
	}
	if err := m.Put(ct); err != nil {
		return nil, err
	}
	return m.GetByID(ctx, ct.ID)
}

// Delete removes a content type.
func (m *MemoryContentTypeRepository) Delete(_ context.Context, id uuid.UUID, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ct, ok := m.types[id]
	if !ok {
		return &NotFoundError{Resource: "content_type", Key: id.String()}
	}
	if ct != nil {
		slug := strings.TrimSpace(ct.Slug)
		if slug != "" {
			delete(m.slugIndex, slug)
		}
	}
	delete(m.types, id)
	return nil
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

// Count returns the number of locales stored in memory.
func (m *MemoryLocaleRepository) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.locales)
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
