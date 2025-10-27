package pages

import (
	"context"
	"sync"
	"time"

	"github.com/goliatone/go-cms/internal/media"
	"github.com/google/uuid"
)

// MemoryPageRepository is an in-memory page store for scaffolding/tests.
type MemoryPageRepository struct {
	mu        sync.RWMutex
	pages     map[uuid.UUID]*Page
	slugIndex map[string]uuid.UUID
	versions  map[uuid.UUID][]*PageVersion
}

// NewMemoryPageRepository constructs the repository.
func NewMemoryPageRepository() *MemoryPageRepository {
	return &MemoryPageRepository{
		pages:     make(map[uuid.UUID]*Page),
		slugIndex: make(map[string]uuid.UUID),
		versions:  make(map[uuid.UUID][]*PageVersion),
	}
}

// Create inserts the supplied page.
func (m *MemoryPageRepository) Create(_ context.Context, record *Page) (*Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := clonePage(record)
	if len(copied.Versions) > 0 {
		m.versions[copied.ID] = clonePageVersions(copied.Versions)
	} else {
		m.versions[copied.ID] = nil
	}
	m.pages[copied.ID] = copied
	m.slugIndex[copied.Slug] = copied.ID
	return m.attachVersions(clonePage(copied)), nil
}

// GetByID retrieves a page by identifier.
func (m *MemoryPageRepository) GetByID(_ context.Context, id uuid.UUID) (*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	page, ok := m.pages[id]
	if !ok {
		return nil, &PageNotFoundError{Key: id.String()}
	}
	return m.attachVersions(clonePage(page)), nil
}

// GetBySlug retrieves a page by slug.
func (m *MemoryPageRepository) GetBySlug(_ context.Context, slug string) (*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.slugIndex[slug]
	if !ok {
		return nil, &PageNotFoundError{Key: slug}
	}
	return m.attachVersions(clonePage(m.pages[id])), nil
}

// List returns every page.
func (m *MemoryPageRepository) List(_ context.Context) ([]*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Page, 0, len(m.pages))
	for _, record := range m.pages {
		out = append(out, m.attachVersions(clonePage(record)))
	}
	return out, nil
}

// Update persists metadata changes for a page.
func (m *MemoryPageRepository) Update(_ context.Context, record *Page) (*Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.pages[record.ID]
	if !ok {
		return nil, &PageNotFoundError{Key: record.ID.String()}
	}

	updated := clonePage(current)
	updated.CurrentVersion = record.CurrentVersion
	updated.PublishedVersion = cloneIntPointer(record.PublishedVersion)
	updated.PublishAt = cloneTimePointer(record.PublishAt)
	updated.UnpublishAt = cloneTimePointer(record.UnpublishAt)
	updated.PublishedAt = cloneTimePointer(record.PublishedAt)
	updated.PublishedBy = cloneUUIDPointer(record.PublishedBy)
	updated.Status = record.Status
	updated.UpdatedAt = record.UpdatedAt
	updated.UpdatedBy = record.UpdatedBy

	m.pages[record.ID] = updated
	return m.attachVersions(clonePage(updated)), nil
}

// CreateVersion appends a version to the supplied page.
func (m *MemoryPageRepository) CreateVersion(_ context.Context, version *PageVersion) (*PageVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := clonePageVersion(version)
	existing := append([]*PageVersion{}, m.versions[cloned.PageID]...)
	existing = append(existing, cloned)
	m.versions[cloned.PageID] = existing
	return clonePageVersion(cloned), nil
}

// ListVersions returns all recorded versions for a page.
func (m *MemoryPageRepository) ListVersions(_ context.Context, pageID uuid.UUID) ([]*PageVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queue := m.versions[pageID]
	return clonePageVersions(queue), nil
}

// GetVersion retrieves a specific version number for a page.
func (m *MemoryPageRepository) GetVersion(_ context.Context, pageID uuid.UUID, number int) (*PageVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, version := range m.versions[pageID] {
		if version.Version == number {
			return clonePageVersion(version), nil
		}
	}
	return nil, &PageVersionNotFoundError{PageID: pageID, Version: number}
}

// GetLatestVersion retrieves the most recent version of a page.
func (m *MemoryPageRepository) GetLatestVersion(_ context.Context, pageID uuid.UUID) (*PageVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[pageID]
	if len(versions) == 0 {
		return nil, &PageVersionNotFoundError{PageID: pageID}
	}
	return clonePageVersion(versions[len(versions)-1]), nil
}

// UpdateVersion mutates stored metadata for a page version.
func (m *MemoryPageRepository) UpdateVersion(_ context.Context, version *PageVersion) (*PageVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue := m.versions[version.PageID]
	for idx, existing := range queue {
		if existing == nil {
			continue
		}
		if existing.ID == version.ID {
			queue[idx] = clonePageVersion(version)
			m.versions[version.PageID] = queue
			return clonePageVersion(queue[idx]), nil
		}
	}
	return nil, &PageVersionNotFoundError{PageID: version.PageID, Version: version.Version}
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
			local.MediaBindings = media.CloneBindingSet(tr.MediaBindings)
			local.ResolvedMedia = media.CloneAttachments(tr.ResolvedMedia)
			copied.Translations[i] = &local
		}
	}
	if len(src.Versions) > 0 {
		copied.Versions = clonePageVersions(src.Versions)
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

func (m *MemoryPageRepository) attachVersions(page *Page) *Page {
	if page == nil {
		return nil
	}
	page.Versions = clonePageVersions(m.versions[page.ID])
	return page
}

func clonePageVersions(src []*PageVersion) []*PageVersion {
	if len(src) == 0 {
		return nil
	}
	out := make([]*PageVersion, len(src))
	for i, version := range src {
		out[i] = clonePageVersion(version)
	}
	return out
}

func clonePageVersion(src *PageVersion) *PageVersion {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.Snapshot = clonePageVersionSnapshot(src.Snapshot)
	cloned.PublishedAt = cloneTimePointer(src.PublishedAt)
	cloned.PublishedBy = cloneUUIDPointer(src.PublishedBy)
	return &cloned
}

func clonePageVersionSnapshot(src PageVersionSnapshot) PageVersionSnapshot {
	target := PageVersionSnapshot{
		Metadata: cloneMap(src.Metadata),
	}
	if len(src.Regions) > 0 {
		target.Regions = make(map[string][]PageBlockPlacement, len(src.Regions))
		for region, placements := range src.Regions {
			target.Regions[region] = cloneBlockPlacements(placements)
		}
	}
	if len(src.Blocks) > 0 {
		target.Blocks = cloneBlockPlacements(src.Blocks)
	}
	if len(src.Widgets) > 0 {
		target.Widgets = make(map[string][]WidgetPlacementSnapshot, len(src.Widgets))
		for area, placements := range src.Widgets {
			target.Widgets[area] = cloneWidgetPlacements(placements)
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

func cloneBlockPlacements(src []PageBlockPlacement) []PageBlockPlacement {
	if len(src) == 0 {
		return nil
	}
	out := make([]PageBlockPlacement, len(src))
	for i, placement := range src {
		out[i] = cloneBlockPlacement(placement)
	}
	return out
}

func cloneBlockPlacement(src PageBlockPlacement) PageBlockPlacement {
	cloned := src
	cloned.Snapshot = cloneMap(src.Snapshot)
	return cloned
}

func cloneWidgetPlacements(src []WidgetPlacementSnapshot) []WidgetPlacementSnapshot {
	if len(src) == 0 {
		return nil
	}
	out := make([]WidgetPlacementSnapshot, len(src))
	for i, placement := range src {
		out[i] = cloneWidgetPlacement(placement)
	}
	return out
}

func cloneWidgetPlacement(src WidgetPlacementSnapshot) WidgetPlacementSnapshot {
	cloned := src
	cloned.Configuration = cloneMap(src.Configuration)
	return cloned
}
