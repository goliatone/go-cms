package di

import (
	"context"
	"sync"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/google/uuid"
)

// contentRepositoryProxy routes calls to the current content repository implementation.
type contentRepositoryProxy struct {
	mu   sync.RWMutex
	repo content.ContentRepository
}

func newContentRepositoryProxy(repo content.ContentRepository) *contentRepositoryProxy {
	return &contentRepositoryProxy{repo: repo}
}

func (p *contentRepositoryProxy) swap(repo content.ContentRepository) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if repo != nil {
		p.repo = repo
	}
}

func (p *contentRepositoryProxy) current() content.ContentRepository {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.repo
}

func (p *contentRepositoryProxy) Create(ctx context.Context, record *content.Content) (*content.Content, error) {
	return p.current().Create(ctx, record)
}

func (p *contentRepositoryProxy) GetByID(ctx context.Context, id uuid.UUID) (*content.Content, error) {
	return p.current().GetByID(ctx, id)
}

func (p *contentRepositoryProxy) GetBySlug(ctx context.Context, slug string, contentTypeID uuid.UUID, env ...string) (*content.Content, error) {
	return p.current().GetBySlug(ctx, slug, contentTypeID, env...)
}

func (p *contentRepositoryProxy) List(ctx context.Context, env ...string) ([]*content.Content, error) {
	return p.current().List(ctx, env...)
}

func (p *contentRepositoryProxy) Update(ctx context.Context, record *content.Content) (*content.Content, error) {
	return p.current().Update(ctx, record)
}

func (p *contentRepositoryProxy) CreateTranslation(ctx context.Context, contentID uuid.UUID, translation *content.ContentTranslation) (*content.ContentTranslation, error) {
	return p.current().CreateTranslation(ctx, contentID, translation)
}

func (p *contentRepositoryProxy) ReplaceTranslations(ctx context.Context, contentID uuid.UUID, translations []*content.ContentTranslation) error {
	return p.current().ReplaceTranslations(ctx, contentID, translations)
}

func (p *contentRepositoryProxy) ListTranslations(ctx context.Context, contentID uuid.UUID) ([]*content.ContentTranslation, error) {
	current := p.current()
	reader, ok := current.(content.ContentTranslationReader)
	if !ok {
		return nil, content.ErrContentTranslationLookupUnsupported
	}
	return reader.ListTranslations(ctx, contentID)
}

func (p *contentRepositoryProxy) Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error {
	return p.current().Delete(ctx, id, hardDelete)
}

func (p *contentRepositoryProxy) CreateVersion(ctx context.Context, version *content.ContentVersion) (*content.ContentVersion, error) {
	return p.current().CreateVersion(ctx, version)
}

func (p *contentRepositoryProxy) ListVersions(ctx context.Context, contentID uuid.UUID) ([]*content.ContentVersion, error) {
	return p.current().ListVersions(ctx, contentID)
}

func (p *contentRepositoryProxy) GetVersion(ctx context.Context, contentID uuid.UUID, number int) (*content.ContentVersion, error) {
	return p.current().GetVersion(ctx, contentID, number)
}

func (p *contentRepositoryProxy) GetLatestVersion(ctx context.Context, contentID uuid.UUID) (*content.ContentVersion, error) {
	return p.current().GetLatestVersion(ctx, contentID)
}

func (p *contentRepositoryProxy) UpdateVersion(ctx context.Context, version *content.ContentVersion) (*content.ContentVersion, error) {
	return p.current().UpdateVersion(ctx, version)
}

// contentTypeRepositoryProxy swaps content type repositories on demand.
type contentTypeRepositoryProxy struct {
	mu   sync.RWMutex
	repo content.ContentTypeRepository
}

func newContentTypeRepositoryProxy(repo content.ContentTypeRepository) *contentTypeRepositoryProxy {
	return &contentTypeRepositoryProxy{repo: repo}
}

func (p *contentTypeRepositoryProxy) swap(repo content.ContentTypeRepository) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if repo != nil {
		p.repo = repo
	}
}

func (p *contentTypeRepositoryProxy) current() content.ContentTypeRepository {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.repo
}

func (p *contentTypeRepositoryProxy) GetByID(ctx context.Context, id uuid.UUID) (*content.ContentType, error) {
	return p.current().GetByID(ctx, id)
}

func (p *contentTypeRepositoryProxy) GetBySlug(ctx context.Context, slug string, env ...string) (*content.ContentType, error) {
	return p.current().GetBySlug(ctx, slug, env...)
}

func (p *contentTypeRepositoryProxy) Create(ctx context.Context, record *content.ContentType) (*content.ContentType, error) {
	return p.current().Create(ctx, record)
}

func (p *contentTypeRepositoryProxy) List(ctx context.Context, env ...string) ([]*content.ContentType, error) {
	return p.current().List(ctx, env...)
}

func (p *contentTypeRepositoryProxy) Search(ctx context.Context, query string, env ...string) ([]*content.ContentType, error) {
	return p.current().Search(ctx, query, env...)
}

func (p *contentTypeRepositoryProxy) Update(ctx context.Context, record *content.ContentType) (*content.ContentType, error) {
	return p.current().Update(ctx, record)
}

func (p *contentTypeRepositoryProxy) Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error {
	return p.current().Delete(ctx, id, hardDelete)
}

func (p *contentTypeRepositoryProxy) Put(ct *content.ContentType) error {
	current := p.current()
	if repo, ok := current.(interface {
		Put(*content.ContentType) error
	}); ok && repo != nil {
		return repo.Put(ct)
	}
	return nil
}

// localeRepositoryProxy swaps locale repositories on demand.
type localeRepositoryProxy struct {
	mu   sync.RWMutex
	repo content.LocaleRepository
}

func newLocaleRepositoryProxy(repo content.LocaleRepository) *localeRepositoryProxy {
	return &localeRepositoryProxy{repo: repo}
}

func (p *localeRepositoryProxy) swap(repo content.LocaleRepository) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if repo != nil {
		p.repo = repo
	}
}

func (p *localeRepositoryProxy) current() content.LocaleRepository {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.repo
}

func (p *localeRepositoryProxy) GetByCode(ctx context.Context, code string) (*content.Locale, error) {
	return p.current().GetByCode(ctx, code)
}

func (p *localeRepositoryProxy) GetByID(ctx context.Context, id uuid.UUID) (*content.Locale, error) {
	return p.current().GetByID(ctx, id)
}

func (p *localeRepositoryProxy) Put(locale *content.Locale) {
	current := p.current()
	if repo, ok := current.(interface{ Put(*content.Locale) }); ok && repo != nil {
		repo.Put(locale)
	}
}

// environmentRepositoryProxy routes calls to the current environment repository implementation.
type environmentRepositoryProxy struct {
	mu   sync.RWMutex
	repo environments.EnvironmentRepository
}

func newEnvironmentRepositoryProxy(repo environments.EnvironmentRepository) *environmentRepositoryProxy {
	return &environmentRepositoryProxy{repo: repo}
}

func (p *environmentRepositoryProxy) swap(repo environments.EnvironmentRepository) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if repo != nil {
		p.repo = repo
	}
}

func (p *environmentRepositoryProxy) current() environments.EnvironmentRepository {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.repo
}

func (p *environmentRepositoryProxy) Create(ctx context.Context, env *environments.Environment) (*environments.Environment, error) {
	return p.current().Create(ctx, env)
}

func (p *environmentRepositoryProxy) Update(ctx context.Context, env *environments.Environment) (*environments.Environment, error) {
	return p.current().Update(ctx, env)
}

func (p *environmentRepositoryProxy) GetByID(ctx context.Context, id uuid.UUID) (*environments.Environment, error) {
	return p.current().GetByID(ctx, id)
}

func (p *environmentRepositoryProxy) GetByKey(ctx context.Context, key string) (*environments.Environment, error) {
	return p.current().GetByKey(ctx, key)
}

func (p *environmentRepositoryProxy) List(ctx context.Context) ([]*environments.Environment, error) {
	return p.current().List(ctx)
}

func (p *environmentRepositoryProxy) ListActive(ctx context.Context) ([]*environments.Environment, error) {
	return p.current().ListActive(ctx)
}

func (p *environmentRepositoryProxy) GetDefault(ctx context.Context) (*environments.Environment, error) {
	return p.current().GetDefault(ctx)
}

func (p *environmentRepositoryProxy) Delete(ctx context.Context, id uuid.UUID) error {
	return p.current().Delete(ctx, id)
}

// pageRepositoryProxy routes calls to the current page repository implementation.
type pageRepositoryProxy struct {
	mu   sync.RWMutex
	repo pages.PageRepository
}

func newPageRepositoryProxy(repo pages.PageRepository) *pageRepositoryProxy {
	return &pageRepositoryProxy{repo: repo}
}

func (p *pageRepositoryProxy) swap(repo pages.PageRepository) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if repo != nil {
		p.repo = repo
	}
}

func (p *pageRepositoryProxy) current() pages.PageRepository {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.repo
}

func (p *pageRepositoryProxy) Create(ctx context.Context, record *pages.Page) (*pages.Page, error) {
	return p.current().Create(ctx, record)
}

func (p *pageRepositoryProxy) GetByID(ctx context.Context, id uuid.UUID) (*pages.Page, error) {
	return p.current().GetByID(ctx, id)
}

func (p *pageRepositoryProxy) GetBySlug(ctx context.Context, slug string, env ...string) (*pages.Page, error) {
	return p.current().GetBySlug(ctx, slug, env...)
}

func (p *pageRepositoryProxy) List(ctx context.Context, env ...string) ([]*pages.Page, error) {
	return p.current().List(ctx, env...)
}

func (p *pageRepositoryProxy) Update(ctx context.Context, record *pages.Page) (*pages.Page, error) {
	return p.current().Update(ctx, record)
}

func (p *pageRepositoryProxy) ReplaceTranslations(ctx context.Context, pageID uuid.UUID, translations []*pages.PageTranslation) error {
	return p.current().ReplaceTranslations(ctx, pageID, translations)
}

func (p *pageRepositoryProxy) Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error {
	return p.current().Delete(ctx, id, hardDelete)
}

func (p *pageRepositoryProxy) CreateVersion(ctx context.Context, version *pages.PageVersion) (*pages.PageVersion, error) {
	return p.current().CreateVersion(ctx, version)
}

func (p *pageRepositoryProxy) ListVersions(ctx context.Context, pageID uuid.UUID) ([]*pages.PageVersion, error) {
	return p.current().ListVersions(ctx, pageID)
}

func (p *pageRepositoryProxy) GetVersion(ctx context.Context, pageID uuid.UUID, number int) (*pages.PageVersion, error) {
	return p.current().GetVersion(ctx, pageID, number)
}

func (p *pageRepositoryProxy) GetLatestVersion(ctx context.Context, pageID uuid.UUID) (*pages.PageVersion, error) {
	return p.current().GetLatestVersion(ctx, pageID)
}

func (p *pageRepositoryProxy) UpdateVersion(ctx context.Context, version *pages.PageVersion) (*pages.PageVersion, error) {
	return p.current().UpdateVersion(ctx, version)
}

// blockDefinitionRepositoryProxy routes calls to the current block definition repository.
type blockDefinitionRepositoryProxy struct {
	mu   sync.RWMutex
	repo blocks.DefinitionRepository
}

func newBlockDefinitionRepositoryProxy(repo blocks.DefinitionRepository) *blockDefinitionRepositoryProxy {
	return &blockDefinitionRepositoryProxy{repo: repo}
}

func (p *blockDefinitionRepositoryProxy) swap(repo blocks.DefinitionRepository) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if repo != nil {
		p.repo = repo
	}
}

func (p *blockDefinitionRepositoryProxy) current() blocks.DefinitionRepository {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.repo
}

func (p *blockDefinitionRepositoryProxy) Create(ctx context.Context, definition *blocks.Definition) (*blocks.Definition, error) {
	return p.current().Create(ctx, definition)
}

func (p *blockDefinitionRepositoryProxy) GetByID(ctx context.Context, id uuid.UUID) (*blocks.Definition, error) {
	return p.current().GetByID(ctx, id)
}

func (p *blockDefinitionRepositoryProxy) GetBySlug(ctx context.Context, slug string, env ...string) (*blocks.Definition, error) {
	return p.current().GetBySlug(ctx, slug, env...)
}

func (p *blockDefinitionRepositoryProxy) List(ctx context.Context, env ...string) ([]*blocks.Definition, error) {
	return p.current().List(ctx, env...)
}

func (p *blockDefinitionRepositoryProxy) Update(ctx context.Context, definition *blocks.Definition) (*blocks.Definition, error) {
	return p.current().Update(ctx, definition)
}

func (p *blockDefinitionRepositoryProxy) Delete(ctx context.Context, id uuid.UUID) error {
	return p.current().Delete(ctx, id)
}

// blockDefinitionVersionRepositoryProxy routes calls to the current block definition version repository.
type blockDefinitionVersionRepositoryProxy struct {
	mu   sync.RWMutex
	repo blocks.DefinitionVersionRepository
}

func newBlockDefinitionVersionRepositoryProxy(repo blocks.DefinitionVersionRepository) *blockDefinitionVersionRepositoryProxy {
	return &blockDefinitionVersionRepositoryProxy{repo: repo}
}

func (p *blockDefinitionVersionRepositoryProxy) swap(repo blocks.DefinitionVersionRepository) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if repo != nil {
		p.repo = repo
	}
}

func (p *blockDefinitionVersionRepositoryProxy) current() blocks.DefinitionVersionRepository {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.repo
}

func (p *blockDefinitionVersionRepositoryProxy) Create(ctx context.Context, version *blocks.DefinitionVersion) (*blocks.DefinitionVersion, error) {
	return p.current().Create(ctx, version)
}

func (p *blockDefinitionVersionRepositoryProxy) GetByID(ctx context.Context, id uuid.UUID) (*blocks.DefinitionVersion, error) {
	return p.current().GetByID(ctx, id)
}

func (p *blockDefinitionVersionRepositoryProxy) GetByDefinitionAndVersion(ctx context.Context, definitionID uuid.UUID, version string) (*blocks.DefinitionVersion, error) {
	return p.current().GetByDefinitionAndVersion(ctx, definitionID, version)
}

func (p *blockDefinitionVersionRepositoryProxy) ListByDefinition(ctx context.Context, definitionID uuid.UUID) ([]*blocks.DefinitionVersion, error) {
	return p.current().ListByDefinition(ctx, definitionID)
}

func (p *blockDefinitionVersionRepositoryProxy) Update(ctx context.Context, version *blocks.DefinitionVersion) (*blocks.DefinitionVersion, error) {
	return p.current().Update(ctx, version)
}

// blockInstanceRepositoryProxy routes calls to the current block instance repository.
type blockInstanceRepositoryProxy struct {
	mu   sync.RWMutex
	repo blocks.InstanceRepository
}

func newBlockInstanceRepositoryProxy(repo blocks.InstanceRepository) *blockInstanceRepositoryProxy {
	return &blockInstanceRepositoryProxy{repo: repo}
}

func (p *blockInstanceRepositoryProxy) swap(repo blocks.InstanceRepository) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if repo != nil {
		p.repo = repo
	}
}

func (p *blockInstanceRepositoryProxy) current() blocks.InstanceRepository {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.repo
}

func (p *blockInstanceRepositoryProxy) Create(ctx context.Context, instance *blocks.Instance) (*blocks.Instance, error) {
	return p.current().Create(ctx, instance)
}

func (p *blockInstanceRepositoryProxy) GetByID(ctx context.Context, id uuid.UUID) (*blocks.Instance, error) {
	return p.current().GetByID(ctx, id)
}

func (p *blockInstanceRepositoryProxy) ListByPage(ctx context.Context, pageID uuid.UUID) ([]*blocks.Instance, error) {
	return p.current().ListByPage(ctx, pageID)
}

func (p *blockInstanceRepositoryProxy) ListGlobal(ctx context.Context) ([]*blocks.Instance, error) {
	return p.current().ListGlobal(ctx)
}

func (p *blockInstanceRepositoryProxy) ListByDefinition(ctx context.Context, definitionID uuid.UUID) ([]*blocks.Instance, error) {
	return p.current().ListByDefinition(ctx, definitionID)
}

func (p *blockInstanceRepositoryProxy) Update(ctx context.Context, instance *blocks.Instance) (*blocks.Instance, error) {
	return p.current().Update(ctx, instance)
}

func (p *blockInstanceRepositoryProxy) Delete(ctx context.Context, id uuid.UUID) error {
	return p.current().Delete(ctx, id)
}

// blockTranslationRepositoryProxy routes calls to the current block translation repository.
type blockTranslationRepositoryProxy struct {
	mu   sync.RWMutex
	repo blocks.TranslationRepository
}

func newBlockTranslationRepositoryProxy(repo blocks.TranslationRepository) *blockTranslationRepositoryProxy {
	return &blockTranslationRepositoryProxy{repo: repo}
}

func (p *blockTranslationRepositoryProxy) swap(repo blocks.TranslationRepository) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if repo != nil {
		p.repo = repo
	}
}

func (p *blockTranslationRepositoryProxy) current() blocks.TranslationRepository {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.repo
}

func (p *blockTranslationRepositoryProxy) Create(ctx context.Context, translation *blocks.Translation) (*blocks.Translation, error) {
	return p.current().Create(ctx, translation)
}

func (p *blockTranslationRepositoryProxy) GetByInstanceAndLocale(ctx context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*blocks.Translation, error) {
	return p.current().GetByInstanceAndLocale(ctx, instanceID, localeID)
}

func (p *blockTranslationRepositoryProxy) ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]*blocks.Translation, error) {
	return p.current().ListByInstance(ctx, instanceID)
}

func (p *blockTranslationRepositoryProxy) Update(ctx context.Context, translation *blocks.Translation) (*blocks.Translation, error) {
	return p.current().Update(ctx, translation)
}

func (p *blockTranslationRepositoryProxy) Delete(ctx context.Context, id uuid.UUID) error {
	return p.current().Delete(ctx, id)
}

// blockVersionRepositoryProxy routes calls to the current block instance version repository.
type blockVersionRepositoryProxy struct {
	mu   sync.RWMutex
	repo blocks.InstanceVersionRepository
}

func newBlockVersionRepositoryProxy(repo blocks.InstanceVersionRepository) *blockVersionRepositoryProxy {
	return &blockVersionRepositoryProxy{repo: repo}
}

func (p *blockVersionRepositoryProxy) swap(repo blocks.InstanceVersionRepository) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if repo != nil {
		p.repo = repo
	}
}

func (p *blockVersionRepositoryProxy) current() blocks.InstanceVersionRepository {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.repo
}

func (p *blockVersionRepositoryProxy) Create(ctx context.Context, version *blocks.InstanceVersion) (*blocks.InstanceVersion, error) {
	return p.current().Create(ctx, version)
}

func (p *blockVersionRepositoryProxy) ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]*blocks.InstanceVersion, error) {
	return p.current().ListByInstance(ctx, instanceID)
}

func (p *blockVersionRepositoryProxy) GetVersion(ctx context.Context, instanceID uuid.UUID, number int) (*blocks.InstanceVersion, error) {
	return p.current().GetVersion(ctx, instanceID, number)
}

func (p *blockVersionRepositoryProxy) GetLatest(ctx context.Context, instanceID uuid.UUID) (*blocks.InstanceVersion, error) {
	return p.current().GetLatest(ctx, instanceID)
}

func (p *blockVersionRepositoryProxy) Update(ctx context.Context, version *blocks.InstanceVersion) (*blocks.InstanceVersion, error) {
	return p.current().Update(ctx, version)
}
