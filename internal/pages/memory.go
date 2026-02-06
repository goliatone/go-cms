package pages

import (
	"context"
	"strings"
	"sync"
	"time"

	cmsenv "github.com/goliatone/go-cms/internal/environments"
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
	copied.EnvironmentID = resolveEnvironmentID(copied.EnvironmentID, "")
	if len(copied.Versions) > 0 {
		m.versions[copied.ID] = clonePageVersions(copied.Versions)
	} else {
		m.versions[copied.ID] = nil
	}
	m.pages[copied.ID] = copied
	m.slugIndex[pageSlugKey(copied.EnvironmentID, copied.Slug)] = copied.ID
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
func (m *MemoryPageRepository) GetBySlug(_ context.Context, slug string, env ...string) (*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	envID := resolveEnvironmentID(uuid.Nil, resolveEnvironmentKey(env...))
	id, ok := m.slugIndex[pageSlugKey(envID, slug)]
	if !ok {
		return nil, &PageNotFoundError{Key: slug}
	}
	return m.attachVersions(clonePage(m.pages[id])), nil
}

// List returns every page.
func (m *MemoryPageRepository) List(_ context.Context, env ...string) ([]*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	envID := resolveEnvironmentID(uuid.Nil, resolveEnvironmentKey(env...))
	out := make([]*Page, 0, len(m.pages))
	for _, record := range m.pages {
		if record == nil {
			continue
		}
		if !matchesEnvironment(record.EnvironmentID, envID) {
			continue
		}
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
	if strings.TrimSpace(record.PrimaryLocale) != "" {
		updated.PrimaryLocale = record.PrimaryLocale
	}
	updated.UpdatedAt = record.UpdatedAt
	updated.UpdatedBy = record.UpdatedBy
	updated.TemplateID = record.TemplateID
	updated.ParentID = record.ParentID
	if len(record.Translations) > 0 {
		updated.Translations = clonePageTranslations(record.Translations)
	}

	m.pages[record.ID] = updated
	return m.attachVersions(clonePage(updated)), nil
}

// ReplaceTranslations swaps the translations associated with a page.
func (m *MemoryPageRepository) ReplaceTranslations(_ context.Context, pageID uuid.UUID, translations []*PageTranslation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.pages[pageID]
	if !ok {
		return &PageNotFoundError{Key: pageID.String()}
	}
	record.Translations = clonePageTranslations(translations)
	return nil
}

// ListTranslations returns stored translations for a page.
func (m *MemoryPageRepository) ListTranslations(_ context.Context, pageID uuid.UUID) ([]*PageTranslation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.pages[pageID]
	if !ok {
		return nil, &PageNotFoundError{Key: pageID.String()}
	}
	return clonePageTranslations(record.Translations), nil
}

// Delete removes the page and associated versions when hard delete is requested.
func (m *MemoryPageRepository) Delete(_ context.Context, id uuid.UUID, hardDelete bool) error {
	if !hardDelete {
		return ErrPageSoftDeleteUnsupported
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.pages[id]
	if !ok {
		return &PageNotFoundError{Key: id.String()}
	}

	delete(m.pages, id)
	if slug := record.Slug; slug != "" {
		delete(m.slugIndex, pageSlugKey(resolveEnvironmentID(record.EnvironmentID, ""), slug))
	}
	delete(m.versions, id)
	return nil
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

func resolveEnvironmentKey(env ...string) string {
	if len(env) == 0 {
		return ""
	}
	return cmsenv.NormalizeKey(env[0])
}

func resolveEnvironmentID(id uuid.UUID, envKey string) uuid.UUID {
	if id != uuid.Nil {
		return id
	}
	key := strings.TrimSpace(envKey)
	if key == "" {
		key = cmsenv.DefaultKey
	}
	if parsed, err := uuid.Parse(key); err == nil {
		return parsed
	}
	key = cmsenv.NormalizeKey(key)
	if key == "" {
		key = cmsenv.DefaultKey
	}
	return cmsenv.IDForKey(key)
}

func matchesEnvironment(recordID, targetID uuid.UUID) bool {
	if targetID == uuid.Nil {
		targetID = resolveEnvironmentID(uuid.Nil, "")
	}
	if recordID == uuid.Nil {
		return targetID == resolveEnvironmentID(uuid.Nil, "")
	}
	return recordID == targetID
}

func pageSlugKey(envID uuid.UUID, slug string) string {
	return envID.String() + "|" + strings.TrimSpace(slug)
}

func clonePage(src *Page) *Page {
	if src == nil {
		return nil
	}
	copied := *src
	copied.Translations = clonePageTranslations(src.Translations)
	if len(src.Versions) > 0 {
		copied.Versions = clonePageVersions(src.Versions)
	}
	return &copied
}

func clonePageTranslations(src []*PageTranslation) []*PageTranslation {
	if len(src) == 0 {
		return nil
	}
	out := make([]*PageTranslation, len(src))
	for i, tr := range src {
		if tr == nil {
			continue
		}
		local := *tr
		local.MediaBindings = media.CloneBindingSet(tr.MediaBindings)
		local.ResolvedMedia = media.CloneAttachments(tr.ResolvedMedia)
		out[i] = &local
	}
	return out
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
