package menus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/identity"
	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/translationconfig"
	"github.com/goliatone/go-cms/pkg/activity"
	"github.com/google/uuid"
)

// Service describes menu management capabilities.
type Service interface {
	CreateMenu(ctx context.Context, input CreateMenuInput) (*Menu, error)
	GetOrCreateMenu(ctx context.Context, input CreateMenuInput) (*Menu, error)
	UpsertMenu(ctx context.Context, input UpsertMenuInput) (*Menu, error)
	MenuByCode(ctx context.Context, code string, locale string, opts MenuQueryOptions, env ...string) (*ResolvedMenu, error)
	MenuByLocation(ctx context.Context, location string, locale string, opts MenuQueryOptions, env ...string) (*ResolvedMenu, error)
	ResolveLocationBinding(ctx context.Context, location, locale string, opts MenuQueryOptions, env ...string) (*MenuLocationBinding, error)
	ApplyViewProfile(ctx context.Context, menu *Menu, profileCode string, opts MenuQueryOptions, env ...string) (*Menu, error)
	UpsertMenuLocationBinding(ctx context.Context, input UpsertMenuLocationBindingInput) (*MenuLocationBinding, error)
	UpsertMenuViewProfile(ctx context.Context, input UpsertMenuViewProfileInput) (*MenuViewProfile, error)
	GetMenu(ctx context.Context, id uuid.UUID) (*Menu, error)
	GetMenuByCode(ctx context.Context, code string, env ...string) (*Menu, error)
	GetMenuByLocation(ctx context.Context, location string, env ...string) (*Menu, error)
	DeleteMenu(ctx context.Context, req DeleteMenuRequest) error
	ResetMenuByCode(ctx context.Context, code string, actor uuid.UUID, force bool, env ...string) error

	AddMenuItem(ctx context.Context, input AddMenuItemInput) (*MenuItem, error)
	UpsertMenuItem(ctx context.Context, input UpsertMenuItemInput) (*MenuItem, error)
	UpdateMenuItem(ctx context.Context, input UpdateMenuItemInput) (*MenuItem, error)
	DeleteMenuItem(ctx context.Context, req DeleteMenuItemRequest) error
	BulkReorderMenuItems(ctx context.Context, input BulkReorderMenuItemsInput) ([]*MenuItem, error)
	ReconcileMenu(ctx context.Context, req ReconcileMenuRequest) (*ReconcileResult, error)

	AddMenuItemTranslation(ctx context.Context, input AddMenuItemTranslationInput) (*MenuItemTranslation, error)
	UpsertMenuItemTranslation(ctx context.Context, input UpsertMenuItemTranslationInput) (*MenuItemTranslation, error)
	GetMenuItemByExternalCode(ctx context.Context, menuCode string, externalCode string, env ...string) (*MenuItem, error)
	ResolveNavigation(ctx context.Context, menuCode string, locale string, env ...string) ([]NavigationNode, error)
	ResolveNavigationByLocation(ctx context.Context, location string, locale string, env ...string) ([]NavigationNode, error)
	InvalidateCache(ctx context.Context) error
}

// CreateMenuInput captures the information required to register a menu.
type CreateMenuInput struct {
	Code           string
	Location       string
	Description    *string
	Status         string
	Locale         *string
	TranslationID  *uuid.UUID
	CreatedBy      uuid.UUID
	UpdatedBy      uuid.UUID
	EnvironmentKey string
}

// UpsertMenuInput captures the information required to create or update a menu by code.
type UpsertMenuInput struct {
	Code           string
	Location       string
	Description    *string
	Status         string
	Locale         *string
	TranslationID  *uuid.UUID
	Actor          uuid.UUID
	EnvironmentKey string
}

type UpsertMenuLocationBindingInput struct {
	ID              *uuid.UUID
	Location        string
	MenuCode        string
	ViewProfileCode *string
	Locale          *string
	Priority        int
	Status          string
	Actor           uuid.UUID
	EnvironmentKey  string
}

type UpsertMenuViewProfileInput struct {
	ID             *uuid.UUID
	Code           string
	Name           string
	Mode           string
	MaxTopLevel    *int
	MaxDepth       *int
	IncludeItemIDs []string
	ExcludeItemIDs []string
	Status         string
	Actor          uuid.UUID
	EnvironmentKey string
}

// AddMenuItemInput captures the data required to register a new menu item.
type AddMenuItemInput struct {
	ID       *uuid.UUID
	MenuID   uuid.UUID
	ParentID *uuid.UUID
	// ParentCode allows callers to reference parents using string codes when UUIDs are not available.
	ParentCode   string
	ExternalCode string
	CanonicalKey string
	// Position is a 0-based insertion index among siblings.
	// Values past the end are clamped to append.
	Position    int
	Type        string
	Target      map[string]any
	Icon        string
	Badge       map[string]any
	Permissions []string
	Classes     []string
	Styles      map[string]string
	Collapsible bool
	Collapsed   bool
	Metadata    map[string]any
	CreatedBy   uuid.UUID
	UpdatedBy   uuid.UUID

	Translations             []MenuItemTranslationInput
	AllowMissingTranslations bool
}

// UpsertMenuItemInput upserts a menu item by canonical identity.
// It supports resolving the target menu by code and deferring parent links when enabled.
type UpsertMenuItemInput struct {
	MenuID          *uuid.UUID
	MenuCode        string
	MenuDescription *string
	EnvironmentKey  string

	// ExternalCode is a stable human-friendly identifier used for both dedupe and parent linking.
	ExternalCode string
	// CanonicalKey optionally overrides the dedupe key; when empty, it is derived from ExternalCode or the target.
	CanonicalKey string

	ParentID   *uuid.UUID
	ParentCode string
	// Position is a 0-based insertion index among siblings.
	// When nil, new items default to append (clamped to sibling length).
	Position *int

	Type        string
	Target      map[string]any
	Icon        string
	Badge       map[string]any
	Permissions []string
	Classes     []string
	Styles      map[string]string
	Collapsible bool
	Collapsed   bool
	Metadata    map[string]any

	Actor uuid.UUID

	Translations             []MenuItemTranslationInput
	AllowMissingTranslations bool
}

// UpdateMenuItemInput captures mutable fields for a menu item.
type UpdateMenuItemInput struct {
	ItemID      uuid.UUID
	Type        *string
	Target      map[string]any
	Icon        *string
	Badge       map[string]any
	Permissions []string
	Classes     []string
	Styles      map[string]string
	Collapsible *bool
	Collapsed   *bool
	Metadata    map[string]any
	// Position is a 0-based insertion index among siblings.
	// Values past the end are clamped to append. Nil leaves the current position unchanged.
	Position  *int
	ParentID  *uuid.UUID
	UpdatedBy uuid.UUID
}

// ReorderMenuItemsInput defines a new hierarchical ordering for menu items.
// BulkReorderMenuItemsInput defines a new hierarchical ordering for menu items.
type BulkReorderMenuItemsInput struct {
	MenuID uuid.UUID
	Items  []ItemOrder
	// UpdatedBy records the actor requesting the reorder for auditing purposes.
	UpdatedBy uuid.UUID
	// Version optionally captures optimistic-lock metadata for future use.
	Version *int
}

// ReorderMenuItemsInput is retained for backward compatibility but mirrors BulkReorderMenuItemsInput.
// Deprecated: use BulkReorderMenuItemsInput instead.
type ReorderMenuItemsInput = BulkReorderMenuItemsInput

// ItemOrder describes the desired parent/position for a menu item.
type ItemOrder struct {
	ItemID   uuid.UUID
	ParentID *uuid.UUID
	Position int
}

// MenuItemTranslationInput describes localized metadata for a menu item.
type MenuItemTranslationInput struct {
	Locale        string
	Label         string
	LabelKey      string
	GroupTitle    string
	GroupTitleKey string
	URLOverride   *string
}

// AddMenuItemTranslationInput adds or updates localized metadata for an item.
type AddMenuItemTranslationInput struct {
	ItemID        uuid.UUID
	Locale        string
	Label         string
	LabelKey      string
	GroupTitle    string
	GroupTitleKey string
	URLOverride   *string
}

// UpsertMenuItemTranslationInput adds or updates localized metadata for an item.
type UpsertMenuItemTranslationInput = AddMenuItemTranslationInput

// DeleteMenuRequest captures the data required to remove a menu.
type DeleteMenuRequest struct {
	MenuID    uuid.UUID
	DeletedBy uuid.UUID
	// Force bypasses guard rails such as theme bindings when true.
	Force bool
}

type ResetMenuCounts struct {
	ItemsDeleted        int
	TranslationsDeleted int
}

// DeleteMenuItemRequest captures the data required to remove a menu item.
type DeleteMenuItemRequest struct {
	ItemID          uuid.UUID
	DeletedBy       uuid.UUID
	CascadeChildren bool
}

// ReconcileMenuRequest triggers a parent-link reconciliation pass for a menu.
type ReconcileMenuRequest struct {
	MenuID    uuid.UUID
	UpdatedBy uuid.UUID
}

// ReconcileResult reports how many items were linked during reconciliation.
type ReconcileResult struct {
	Resolved  int
	Remaining int
}

const (
	MenuViewModeFull          = "full"
	MenuViewModeTopLevelLimit = "top_level_limit"
	MenuViewModeMaxDepth      = "max_depth"
	MenuViewModeIncludeIDs    = "include_ids"
	MenuViewModeExcludeIDs    = "exclude_ids"
	MenuViewModeComposed      = "composed"

	MenuBindingPolicySingle        = "single"
	MenuBindingPolicyPriorityMulti = "priority_multi"

	MenuContributionDuplicateByURL    = content.NavigationDuplicateByURL
	MenuContributionDuplicateByTarget = content.NavigationDuplicateByTarget
	MenuContributionDuplicateNone     = content.NavigationDuplicateNone
)

type MenuQueryOptions struct {
	IncludeDrafts               bool
	PreviewToken                string
	Status                      string
	ViewProfile                 string
	BindingPolicy               string
	IncludeContributions        *bool
	ContributionMergeMode       string
	ContributionDuplicatePolicy string
}

type ResolvedMenuPreview struct {
	IncludeDrafts       bool   `json:"include_drafts"`
	PreviewTokenPresent bool   `json:"preview_token_present"`
	MenuStatus          string `json:"menu_status,omitempty"`
	BindingStatus       string `json:"binding_status,omitempty"`
	ViewProfileStatus   string `json:"view_profile_status,omitempty"`
}

type ResolvedMenu struct {
	Location          string                  `json:"location"`
	RequestedLocale   string                  `json:"requested_locale,omitempty"`
	ResolvedLocale    string                  `json:"resolved_locale,omitempty"`
	Menu              *Menu                   `json:"menu,omitempty"`
	Menus             []*Menu                 `json:"menus,omitempty"`
	Binding           *MenuLocationBinding    `json:"binding,omitempty"`
	Bindings          []*MenuLocationBinding  `json:"bindings,omitempty"`
	ViewProfile       *MenuViewProfile        `json:"view_profile,omitempty"`
	Items             []NavigationNode        `json:"items,omitempty"`
	ContentMembership []ContentMenuMembership `json:"content_membership,omitempty"`
	Preview           ResolvedMenuPreview     `json:"preview"`
	FamilyID          *uuid.UUID              `json:"family_id,omitempty"`
}

var (
	ErrMenuCodeRequired                    = errors.New("menus: code is required")
	ErrMenuCodeInvalid                     = errors.New("menus: code must contain only letters, numbers, hyphen, or underscore")
	ErrMenuCodeExists                      = errors.New("menus: code already exists")
	ErrMenuNotFound                        = errors.New("menus: menu not found")
	ErrMenuInUse                           = errors.New("menus: menu is assigned to an active theme")
	ErrMenuItemNotFound                    = errors.New("menus: menu item not found")
	ErrMenuItemParentInvalid               = errors.New("menus: parent menu item invalid")
	ErrMenuItemCycle                       = errors.New("menus: hierarchy creates a cycle")
	ErrMenuItemPosition                    = errors.New("menus: position must be zero or positive")
	ErrMenuItemTargetMissing               = errors.New("menus: target type is required")
	ErrMenuItemTranslations                = errors.New("menus: at least one translation is required")
	ErrMenuItemDuplicateLocale             = errors.New("menus: duplicate translation locale provided")
	ErrMenuItemHasChildren                 = errors.New("menus: menu item has children; enable cascade to delete")
	ErrUnknownLocale                       = errors.New("menus: locale is unknown")
	ErrTranslationExists                   = errors.New("menus: translation already exists for locale")
	ErrTranslationLabelRequired            = errors.New("menus: translation label is required")
	ErrMenuItemPageNotFound                = errors.New("menus: page target not found")
	ErrMenuItemPageSlugRequired            = errors.New("menus: page target requires slug")
	ErrMenuItemTypeInvalid                 = errors.New("menus: menu item type is invalid")
	ErrMenuItemParentUnsupported           = errors.New("menus: parent cannot accept children")
	ErrMenuItemSeparatorFields             = errors.New("menus: separators cannot have targets, children, labels, icons, or badges")
	ErrMenuItemGroupFields                 = errors.New("menus: groups cannot define targets, icons, or badges")
	ErrMenuItemCollapsibleWithoutChildren  = errors.New("menus: collapsible menus require children")
	ErrMenuItemCollapsedWithoutCollapsible = errors.New("menus: collapsed menus must be marked collapsible")
	ErrMenuItemTranslationTextRequired     = errors.New("menus: translation requires label or translation key")
	ErrMenuItemDepthExceeded               = errors.New("menus: hierarchy exceeds maximum depth")
	ErrMenuLocationBindingNotFound         = errors.New("menus: menu location binding not found")
	ErrMenuViewProfileNotFound             = errors.New("menus: menu view profile not found")
	ErrMenuStatusInvalid                   = errors.New("menus: menu status is invalid")
	ErrMenuViewModeInvalid                 = errors.New("menus: menu view profile mode is invalid")
)

// LocaleRepository resolves locales by code.
type LocaleRepository interface {
	GetByCode(ctx context.Context, code string) (*content.Locale, error)
}

// PageRepository looks up pages for menu target validation and navigation resolution.
type PageRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*pages.Page, error)
	GetBySlug(ctx context.Context, slug string, env ...string) (*pages.Page, error)
}

// ContentRepository reads content entries used for content-driven menu contributions.
type ContentRepository interface {
	List(ctx context.Context, env ...content.ContentListOption) ([]*content.Content, error)
}

// ContentTypeRepository reads content type metadata used for navigation capabilities.
type ContentTypeRepository interface {
	List(ctx context.Context, env ...string) ([]*content.ContentType, error)
}

// NavigationNode represents a localized menu item ready for presentation.
type NavigationNode struct {
	ID                 uuid.UUID         `json:"id"`
	Position           int               `json:"position"`
	Type               string            `json:"type,omitempty"`
	Label              string            `json:"label,omitempty"`
	LabelKey           string            `json:"label_key,omitempty"`
	GroupTitle         string            `json:"group_title,omitempty"`
	GroupTitleKey      string            `json:"group_title_key,omitempty"`
	URL                string            `json:"url"`
	Target             map[string]any    `json:"target,omitempty"`
	Icon               string            `json:"icon,omitempty"`
	Badge              map[string]any    `json:"badge,omitempty"`
	Permissions        []string          `json:"permissions,omitempty"`
	Classes            []string          `json:"classes,omitempty"`
	Styles             map[string]string `json:"styles,omitempty"`
	Collapsible        bool              `json:"collapsible,omitempty"`
	Collapsed          bool              `json:"collapsed,omitempty"`
	Metadata           map[string]any    `json:"metadata,omitempty"`
	Contribution       bool              `json:"contribution,omitempty"`
	ContributionOrigin string            `json:"contribution_origin,omitempty"`
	Children           []NavigationNode  `json:"children,omitempty"`
}

// ContentMenuMembership reports computed content contribution membership for a location.
type ContentMenuMembership struct {
	ContentID       uuid.UUID `json:"content_id"`
	ContentTypeID   uuid.UUID `json:"content_type_id"`
	ContentTypeSlug string    `json:"content_type_slug,omitempty"`
	ContentSlug     string    `json:"content_slug,omitempty"`
	Location        string    `json:"location"`
	Visible         bool      `json:"visible"`
	Origin          string    `json:"origin,omitempty"`
	VisibilityState string    `json:"visibility_state,omitempty"`
	MergeMode       string    `json:"merge_mode,omitempty"`
	DuplicatePolicy string    `json:"duplicate_policy,omitempty"`
	URL             string    `json:"url,omitempty"`
	Label           string    `json:"label,omitempty"`
	SortOrder       *int      `json:"sort_order,omitempty"`
}

// MenuUsageResolver reports whether menus are currently bound to active themes/locations.
type MenuUsageResolver interface {
	ResolveMenuUsage(ctx context.Context, menuID uuid.UUID) ([]MenuUsageBinding, error)
}

// MenuUsageBinding describes an active menu reference inside a theme/location pair.
type MenuUsageBinding struct {
	ThemeID      uuid.UUID
	ThemeName    string
	LocationCode string
}

// MenuInUseError surfaces guard-rail failures for menu deletions.
type MenuInUseError struct {
	MenuID   uuid.UUID
	Bindings []MenuUsageBinding
}

func (e *MenuInUseError) Error() string {
	if len(e.Bindings) == 0 {
		return fmt.Sprintf("menu %s is in use", e.MenuID)
	}

	parts := make([]string, 0, len(e.Bindings))
	for _, binding := range e.Bindings {
		if binding.ThemeName != "" && binding.LocationCode != "" {
			parts = append(parts, fmt.Sprintf("%s:%s", binding.ThemeName, binding.LocationCode))
		} else if binding.ThemeName != "" {
			parts = append(parts, binding.ThemeName)
		} else {
			parts = append(parts, binding.LocationCode)
		}
	}
	return fmt.Sprintf("menu %s is in use (%s)", e.MenuID, strings.Join(parts, ", "))
}

func (e *MenuInUseError) Unwrap() error {
	return ErrMenuInUse
}

// IDGenerator produces unique identifiers.
// It receives the normalized AddMenuItemInput so callers can derive deterministic IDs from payload fields.
type IDGenerator func(AddMenuItemInput) uuid.UUID

// MenuIDDeriver produces deterministic menu UUIDs from the stable menu code.
type MenuIDDeriver func(code string) uuid.UUID

// ParentResolver maps caller-provided parent codes (non-UUID) into UUIDs before validation.
type ParentResolver func(ctx context.Context, code string, input AddMenuItemInput) (*uuid.UUID, error)

// ServiceOption configures menu service behaviour.
type ServiceOption func(*service)

// WithClock overrides the internal time source.
func WithClock(clock func() time.Time) ServiceOption {
	return func(s *service) {
		if clock != nil {
			s.now = clock
		}
	}
}

// WithIDGenerator overrides the ID generator.
func WithIDGenerator(generator IDGenerator) ServiceOption {
	return func(s *service) {
		if generator != nil {
			s.id = generator
		}
	}
}

// WithParentResolver wires a resolver used to translate non-UUID parent references into UUIDs.
func WithParentResolver(resolver ParentResolver) ServiceOption {
	return func(s *service) {
		if resolver != nil {
			s.parentResolver = resolver
		}
	}
}

// WithPageRepository wires the page repository used for target validation and URL resolution.
func WithPageRepository(repo PageRepository) ServiceOption {
	return func(s *service) {
		s.pageRepo = repo
	}
}

// WithContentRepository wires the content repository used for content-driven contributions.
func WithContentRepository(repo ContentRepository) ServiceOption {
	return func(s *service) {
		s.contentRepo = repo
	}
}

// WithContentTypeRepository wires the content-type repository used for contribution capability lookup.
func WithContentTypeRepository(repo ContentTypeRepository) ServiceOption {
	return func(s *service) {
		s.contentTypeRepo = repo
	}
}

// WithRequireTranslations controls whether menu items must include translations.
func WithRequireTranslations(required bool) ServiceOption {
	return func(s *service) {
		s.requireTranslations = required
	}
}

// WithTranslationsEnabled toggles translation handling.
func WithTranslationsEnabled(enabled bool) ServiceOption {
	return func(s *service) {
		s.translationsEnabled = enabled
	}
}

// WithTranslationState wires a shared, runtime-configurable translation state.
func WithTranslationState(state *translationconfig.State) ServiceOption {
	return func(s *service) {
		s.translationState = state
	}
}

// WithActivityEmitter wires the activity emitter used for activity records.
func WithActivityEmitter(emitter *activity.Emitter) ServiceOption {
	return func(s *service) {
		if emitter != nil {
			s.activity = emitter
		}
	}
}

// WithMenuUsageResolver injects a dependency that reports active menu bindings.
func WithMenuUsageResolver(resolver MenuUsageResolver) ServiceOption {
	return func(s *service) {
		if resolver != nil {
			s.usageResolver = resolver
		}
	}
}

// WithMenuLocationBindingRepository wires persistent location bindings used by runtime reads.
func WithMenuLocationBindingRepository(repo MenuLocationBindingRepository) ServiceOption {
	return func(s *service) {
		if repo != nil {
			s.bindings = repo
		}
	}
}

// WithMenuViewProfileRepository wires persistent view profiles used for menu projections.
func WithMenuViewProfileRepository(repo MenuViewProfileRepository) ServiceOption {
	return func(s *service) {
		if repo != nil {
			s.viewProfiles = repo
		}
	}
}

func WithAuditRecorder(recorder jobs.AuditRecorder) ServiceOption {
	return func(s *service) {
		if recorder != nil {
			s.audit = recorder
		}
	}
}

// WithRecordIDGenerator overrides the ID generator used for non-menu-item records (menus, translations).
// Menu item IDs remain governed by WithIDGenerator.
func WithRecordIDGenerator(generator func() uuid.UUID) ServiceOption {
	return func(s *service) {
		if generator != nil {
			s.newID = generator
		}
	}
}

// WithEnvironmentService wires the environment service for env resolution.
func WithEnvironmentService(envSvc cmsenv.Service) ServiceOption {
	return func(s *service) {
		if envSvc != nil {
			s.envSvc = envSvc
		}
	}
}

// WithDefaultEnvironmentKey overrides the default environment key.
func WithDefaultEnvironmentKey(key string) ServiceOption {
	return func(s *service) {
		if strings.TrimSpace(key) != "" {
			s.defaultEnvKey = key
		}
	}
}

// WithRequireExplicitEnvironment enforces explicit environment selection.
func WithRequireExplicitEnvironment(required bool) ServiceOption {
	return func(s *service) {
		s.requireExplicitEnv = required
	}
}

// WithRequireActiveEnvironment blocks operations on inactive environments.
func WithRequireActiveEnvironment(required bool) ServiceOption {
	return func(s *service) {
		s.requireActiveEnv = required
	}
}

// WithMaxMenuDepth controls menu hierarchy guard rails. Zero or negative disables depth checks.
func WithMaxMenuDepth(depth int) ServiceOption {
	return func(s *service) {
		s.maxDepth = depth
	}
}

// WithBindingDuplicatePolicy configures how duplicate location bindings are handled.
func WithBindingDuplicatePolicy(policy string) ServiceOption {
	return func(s *service) {
		s.duplicateBindingPolicy = normalizeBindingPolicy(policy)
	}
}

// WithMenuIDDeriver overrides menu ID derivation so callers can generate stable UUIDs from menu codes.
func WithMenuIDDeriver(deriver MenuIDDeriver) ServiceOption {
	return func(s *service) {
		if deriver != nil {
			s.menuIDDeriver = deriver
		}
	}
}

// WithForgivingMenuBootstrap enables order-independent menu seeding:
// missing parents are deferred, collapsible flags are tolerated until children exist,
// and reconciliation runs automatically after writes and before navigation resolution.
func WithForgivingMenuBootstrap(enabled bool) ServiceOption {
	return func(s *service) {
		s.forgivingBootstrap = enabled
		if enabled {
			s.reconcileOnResolve = true
		}
	}
}

// WithReconcileOnResolveNavigation controls whether ResolveNavigation runs reconciliation first.
func WithReconcileOnResolveNavigation(enabled bool) ServiceOption {
	return func(s *service) {
		s.reconcileOnResolve = enabled
	}
}

type service struct {
	menus                  MenuRepository
	items                  MenuItemRepository
	translations           MenuItemTranslationRepository
	bindings               MenuLocationBindingRepository
	viewProfiles           MenuViewProfileRepository
	locales                LocaleRepository
	pageRepo               PageRepository
	contentRepo            ContentRepository
	contentTypeRepo        ContentTypeRepository
	usageResolver          MenuUsageResolver
	parentResolver         ParentResolver
	audit                  jobs.AuditRecorder
	now                    func() time.Time
	id                     IDGenerator
	newID                  func() uuid.UUID
	urlResolver            URLResolver
	requireTranslations    bool
	translationsEnabled    bool
	translationState       *translationconfig.State
	activity               *activity.Emitter
	forgivingBootstrap     bool
	reconcileOnResolve     bool
	menuIDDeriver          MenuIDDeriver
	envSvc                 cmsenv.Service
	defaultEnvKey          string
	requireExplicitEnv     bool
	requireActiveEnv       bool
	maxDepth               int
	duplicateBindingPolicy string
}

type cacheInvalidator interface {
	InvalidateCache(ctx context.Context) error
}

// NewService constructs a menu service instance.
func NewService(menuRepo MenuRepository, itemRepo MenuItemRepository, trRepo MenuItemTranslationRepository, localeRepo LocaleRepository, opts ...ServiceOption) Service {
	s := &service{
		menus:                  menuRepo,
		items:                  itemRepo,
		translations:           trRepo,
		bindings:               NewMemoryMenuLocationBindingRepository(),
		viewProfiles:           NewMemoryMenuViewProfileRepository(),
		locales:                localeRepo,
		now:                    time.Now,
		id:                     func(AddMenuItemInput) uuid.UUID { return uuid.New() },
		newID:                  uuid.New,
		requireTranslations:    true,
		translationsEnabled:    true,
		activity:               activity.NewEmitter(nil, activity.Config{}),
		defaultEnvKey:          cmsenv.DefaultKey,
		maxDepth:               16,
		duplicateBindingPolicy: MenuBindingPolicySingle,
	}

	s.urlResolver = &defaultURLResolver{service: s}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *service) emitActivity(ctx context.Context, actor uuid.UUID, verb, objectType string, objectID uuid.UUID, meta map[string]any) {
	if s.activity == nil || !s.activity.Enabled() || objectID == uuid.Nil {
		return
	}
	if meta != nil {
		if _, ok := meta["environment_key"]; !ok {
			if raw, ok := meta["environment_id"].(string); ok && strings.TrimSpace(raw) != "" {
				if parsed, err := uuid.Parse(raw); err == nil {
					if key := s.environmentKeyForID(ctx, parsed); key != "" {
						meta["environment_key"] = key
					}
				}
			}
		}
	}
	event := activity.Event{
		Verb:       verb,
		ActorID:    actor.String(),
		ObjectType: objectType,
		ObjectID:   objectID.String(),
		Metadata:   meta,
	}
	_ = s.activity.Emit(ctx, event)
}

func (s *service) resolveEnvironment(ctx context.Context, key string) (uuid.UUID, string, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" && s.requireExplicitEnv {
		return uuid.Nil, "", cmsenv.ErrEnvironmentKeyRequired
	}
	if trimmed != "" {
		if parsed, err := uuid.Parse(trimmed); err == nil {
			return parsed, trimmed, nil
		}
	}
	normalized, err := cmsenv.ResolveKey(trimmed, s.defaultEnvKey, s.requireExplicitEnv)
	if err != nil {
		return uuid.Nil, "", err
	}
	if s.envSvc == nil {
		return cmsenv.IDForKey(normalized), normalized, nil
	}
	env, err := s.envSvc.GetEnvironmentByKey(ctx, normalized)
	if err != nil {
		return uuid.Nil, "", err
	}
	if s.requireActiveEnv && !env.IsActive {
		return uuid.Nil, "", cmsenv.ErrEnvironmentNotFound
	}
	return env.ID, env.Key, nil
}

func (s *service) ensureEnvironmentActive(ctx context.Context, envID uuid.UUID) error {
	if !s.requireActiveEnv || s.envSvc == nil || envID == uuid.Nil {
		return nil
	}
	env, err := s.envSvc.GetEnvironment(ctx, envID)
	if err != nil {
		return err
	}
	if !env.IsActive {
		return cmsenv.ErrEnvironmentNotFound
	}
	return nil
}

func (s *service) environmentKeyForID(ctx context.Context, envID uuid.UUID) string {
	if envID == uuid.Nil {
		return ""
	}
	if s.envSvc != nil {
		env, err := s.envSvc.GetEnvironment(ctx, envID)
		if err == nil && env != nil {
			return env.Key
		}
	}
	defaultKey := strings.TrimSpace(s.defaultEnvKey)
	if defaultKey == "" {
		defaultKey = cmsenv.DefaultKey
	}
	if envID == cmsenv.IDForKey(defaultKey) {
		return cmsenv.NormalizeKey(defaultKey)
	}
	if envID == cmsenv.IDForKey(cmsenv.DefaultKey) {
		return cmsenv.DefaultKey
	}
	return ""
}

func (s *service) isDefaultEnvironmentKey(key string) bool {
	normalized := cmsenv.NormalizeKey(key)
	if normalized == "" {
		return true
	}
	defaultKey := cmsenv.NormalizeKey(s.defaultEnvKey)
	if defaultKey == "" {
		defaultKey = cmsenv.DefaultKey
	}
	return normalized == defaultKey
}

func (s *service) menuIdentityKey(code, envKey string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return ""
	}
	if s.isDefaultEnvironmentKey(envKey) {
		return trimmed
	}
	return cmsenv.NormalizeKey(envKey) + ":" + trimmed
}

func (s *service) menuIDForCode(code, envKey string) uuid.UUID {
	key := s.menuIdentityKey(code, envKey)
	if key == "" {
		return s.nextID()
	}
	if s.menuIDDeriver != nil {
		return s.menuIDDeriver(key)
	}
	if s.forgivingBootstrap {
		return s.deterministicUUID("go-cms:menu:" + key)
	}
	return s.nextID()
}

func pickEnvironmentKey(env ...string) string {
	if len(env) == 0 {
		return ""
	}
	return strings.TrimSpace(env[0])
}

func (s *service) translationsRequired() bool {
	enabled := s.translationsEnabled
	required := s.requireTranslations
	if s.translationState != nil {
		enabled = s.translationState.Enabled()
		required = s.translationState.RequireTranslations()
	}
	return enabled && required
}

// CreateMenu registers a new menu ensuring code uniqueness.
func (s *service) CreateMenu(ctx context.Context, input CreateMenuInput) (*Menu, error) {
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return nil, ErrMenuCodeRequired
	}
	if !isValidCode(code) {
		return nil, ErrMenuCodeInvalid
	}

	envID, envKey, err := s.resolveEnvironment(ctx, input.EnvironmentKey)
	if err != nil {
		return nil, err
	}

	if _, err := s.menus.GetByCode(ctx, code, envID.String()); err == nil {
		return nil, ErrMenuCodeExists
	} else if err != nil {
		var notFound *NotFoundError
		if !errors.As(err, &notFound) {
			return nil, err
		}
	}

	now := s.now()
	menuID := s.menuIDForCode(code, envKey)
	status, err := normalizeMenuStatus(input.Status)
	if err != nil {
		return nil, err
	}
	var publishedAt *time.Time
	if status == MenuStatusPublished {
		published := now
		publishedAt = &published
	}
	menu := &Menu{
		ID:            menuID,
		Code:          code,
		Location:      strings.TrimSpace(input.Location),
		Description:   input.Description,
		Status:        status,
		Locale:        normalizeLocalePointer(input.Locale),
		FamilyID:      cloneUUIDPointer(input.TranslationID),
		PublishedAt:   publishedAt,
		CreatedBy:     input.CreatedBy,
		UpdatedBy:     input.UpdatedBy,
		CreatedAt:     now,
		UpdatedAt:     now,
		EnvironmentID: envID,
	}

	created, err := s.menus.Create(ctx, menu)
	if err != nil {
		return nil, err
	}
	meta := map[string]any{
		"code": created.Code,
	}
	if created.EnvironmentID != uuid.Nil {
		meta["environment_id"] = created.EnvironmentID.String()
	}
	s.emitActivity(ctx, pickActor(input.CreatedBy, input.UpdatedBy), "create", "menu", created.ID, meta)
	return created, nil
}

// GetOrCreateMenu returns an existing menu for the provided code or creates it when missing.
func (s *service) GetOrCreateMenu(ctx context.Context, input CreateMenuInput) (*Menu, error) {
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return nil, ErrMenuCodeRequired
	}
	if !isValidCode(code) {
		return nil, ErrMenuCodeInvalid
	}

	envID, envKey, err := s.resolveEnvironment(ctx, input.EnvironmentKey)
	if err != nil {
		return nil, err
	}

	existing, err := s.menus.GetByCode(ctx, code, envID.String())
	if err == nil {
		changed := false
		location := strings.TrimSpace(input.Location)
		if location != "" && existing.Location != location {
			existing.Location = location
			changed = true
		}
		if input.Description != nil || existing.Description != nil {
			existing.Description = input.Description
			changed = true
		}
		if input.Locale != nil {
			existing.Locale = normalizeLocalePointer(input.Locale)
			changed = true
		}
		if input.TranslationID != nil {
			existing.FamilyID = cloneUUIDPointer(input.TranslationID)
			changed = true
		}
		if status, statusErr := normalizeMenuStatus(input.Status); statusErr == nil {
			if existing.Status != status {
				existing.Status = status
				if status == MenuStatusPublished {
					published := s.now()
					existing.PublishedAt = &published
				}
				changed = true
			}
		}
		if changed {
			existing.UpdatedBy = input.UpdatedBy
			existing.UpdatedAt = s.now()
			if _, updateErr := s.menus.Update(ctx, existing); updateErr == nil {
				return existing, nil
			}
		}
		return existing, nil
	}
	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		return nil, err
	}

	now := s.now()
	menuID := s.menuIDForCode(code, envKey)
	status, err := normalizeMenuStatus(input.Status)
	if err != nil {
		return nil, err
	}
	var publishedAt *time.Time
	if status == MenuStatusPublished {
		published := now
		publishedAt = &published
	}
	menu := &Menu{
		ID:            menuID,
		Code:          code,
		Location:      strings.TrimSpace(input.Location),
		Description:   input.Description,
		Status:        status,
		Locale:        normalizeLocalePointer(input.Locale),
		FamilyID:      cloneUUIDPointer(input.TranslationID),
		PublishedAt:   publishedAt,
		CreatedBy:     input.CreatedBy,
		UpdatedBy:     input.UpdatedBy,
		CreatedAt:     now,
		UpdatedAt:     now,
		EnvironmentID: envID,
	}

	created, err := s.menus.Create(ctx, menu)
	if err == nil {
		meta := map[string]any{
			"code": created.Code,
		}
		if created.EnvironmentID != uuid.Nil {
			meta["environment_id"] = created.EnvironmentID.String()
		}
		s.emitActivity(ctx, pickActor(input.CreatedBy, input.UpdatedBy), "create", "menu", created.ID, meta)
		return created, nil
	}

	// If the create failed because another caller created the menu concurrently, return the winner.
	existing, getErr := s.menus.GetByCode(ctx, code, envID.String())
	if getErr == nil {
		return existing, nil
	}

	return nil, err
}

// UpsertMenu creates or updates a menu by code.
func (s *service) UpsertMenu(ctx context.Context, input UpsertMenuInput) (*Menu, error) {
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return nil, ErrMenuCodeRequired
	}
	if !isValidCode(code) {
		return nil, ErrMenuCodeInvalid
	}

	envID, _, err := s.resolveEnvironment(ctx, input.EnvironmentKey)
	if err != nil {
		return nil, err
	}

	existing, err := s.menus.GetByCode(ctx, code, envID.String())
	if err != nil {
		var notFound *NotFoundError
		if !errors.As(err, &notFound) {
			return nil, err
		}
		created, err := s.CreateMenu(ctx, CreateMenuInput{
			Code:           code,
			Location:       input.Location,
			Description:    input.Description,
			Status:         input.Status,
			Locale:         input.Locale,
			TranslationID:  input.TranslationID,
			CreatedBy:      input.Actor,
			UpdatedBy:      input.Actor,
			EnvironmentKey: input.EnvironmentKey,
		})
		if err != nil {
			return nil, err
		}
		return created, nil
	}

	// Update description if provided (can be nil to clear).
	existing.Description = input.Description
	if location := strings.TrimSpace(input.Location); location != "" {
		existing.Location = location
	}
	if input.Locale != nil {
		existing.Locale = normalizeLocalePointer(input.Locale)
	}
	if input.TranslationID != nil {
		existing.FamilyID = cloneUUIDPointer(input.TranslationID)
	}
	if status, statusErr := normalizeMenuStatus(input.Status); statusErr != nil {
		return nil, statusErr
	} else if status != "" {
		existing.Status = status
		if status == MenuStatusPublished {
			published := s.now()
			existing.PublishedAt = &published
		}
	}
	existing.UpdatedBy = input.Actor
	existing.UpdatedAt = s.now()
	updated, err := s.menus.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	meta := map[string]any{
		"code": updated.Code,
	}
	if updated.EnvironmentID != uuid.Nil {
		meta["environment_id"] = updated.EnvironmentID.String()
	}
	s.emitActivity(ctx, input.Actor, "update", "menu", updated.ID, meta)
	return updated, nil
}

func (s *service) UpsertMenuLocationBinding(ctx context.Context, input UpsertMenuLocationBindingInput) (*MenuLocationBinding, error) {
	location := strings.TrimSpace(input.Location)
	if location == "" {
		return nil, ErrMenuLocationBindingNotFound
	}
	menuCode := strings.TrimSpace(input.MenuCode)
	if menuCode == "" {
		return nil, ErrMenuCodeRequired
	}
	if input.Priority < 0 {
		return nil, ErrMenuItemPosition
	}

	envID, _, err := s.resolveEnvironment(ctx, input.EnvironmentKey)
	if err != nil {
		return nil, err
	}
	status, err := normalizeMenuStatus(input.Status)
	if err != nil {
		return nil, err
	}

	// Ensure bound menu exists.
	if _, err := s.menus.GetByCode(ctx, menuCode, envID.String()); err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}

	now := s.now()
	var record *MenuLocationBinding
	exists := false
	if input.ID != nil && *input.ID != uuid.Nil {
		existing, err := s.bindings.GetByID(ctx, *input.ID)
		if err == nil {
			record = existing
			exists = true
		}
	}
	if record == nil {
		existing, err := s.bindings.ListByLocation(ctx, location, envID.String())
		if err != nil {
			return nil, err
		}
		locale := normalizeLocalePointer(input.Locale)
		policy := s.effectiveBindingPolicy(MenuQueryOptions{})
		for _, candidate := range existing {
			if candidate == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(candidate.MenuCode), menuCode) &&
				localePointerEqual(candidate.Locale, locale) &&
				stringPointerEqual(candidate.ViewProfileCode, normalizeCodePointer(input.ViewProfileCode)) {
				record = candidate
				exists = true
				break
			}
			if policy == MenuBindingPolicySingle && localePointerEqual(candidate.Locale, locale) {
				record = candidate
				exists = true
				break
			}
		}
	}
	if record == nil {
		record = &MenuLocationBinding{
			ID:            s.nextID(),
			CreatedBy:     input.Actor,
			CreatedAt:     now,
			Location:      location,
			EnvironmentID: envID,
		}
	}

	record.Location = location
	record.MenuCode = menuCode
	record.ViewProfileCode = normalizeCodePointer(input.ViewProfileCode)
	record.Locale = normalizeLocalePointer(input.Locale)
	record.Priority = input.Priority
	record.Status = status
	record.UpdatedBy = input.Actor
	record.UpdatedAt = now
	record.EnvironmentID = envID
	if status == MenuStatusPublished {
		published := now
		record.PublishedAt = &published
	}

	if exists {
		updated, err := s.bindings.Update(ctx, record)
		if err != nil {
			return nil, err
		}
		return updated, nil
	}
	return s.bindings.Create(ctx, record)
}

func (s *service) UpsertMenuViewProfile(ctx context.Context, input UpsertMenuViewProfileInput) (*MenuViewProfile, error) {
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return nil, ErrMenuCodeRequired
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = code
	}
	mode, err := normalizeMenuViewMode(input.Mode)
	if err != nil {
		return nil, err
	}
	status, err := normalizeMenuStatus(input.Status)
	if err != nil {
		return nil, err
	}
	envID, _, err := s.resolveEnvironment(ctx, input.EnvironmentKey)
	if err != nil {
		return nil, err
	}

	now := s.now()
	var record *MenuViewProfile
	exists := false
	if input.ID != nil && *input.ID != uuid.Nil {
		existing, err := s.viewProfiles.GetByID(ctx, *input.ID)
		if err == nil {
			record = existing
			exists = true
		}
	}
	if record == nil {
		existing, err := s.viewProfiles.GetByCode(ctx, code, envID.String())
		if err == nil {
			record = existing
			exists = true
		}
	}
	if record == nil {
		record = &MenuViewProfile{
			ID:            s.nextID(),
			CreatedBy:     input.Actor,
			CreatedAt:     now,
			EnvironmentID: envID,
		}
	}

	record.Code = code
	record.Name = name
	record.Mode = mode
	record.MaxTopLevel = normalizePositiveIntPointer(input.MaxTopLevel)
	record.MaxDepth = normalizePositiveIntPointer(input.MaxDepth)
	record.IncludeItemIDs = normalizeCodeList(input.IncludeItemIDs)
	record.ExcludeItemIDs = normalizeCodeList(input.ExcludeItemIDs)
	record.Status = status
	record.UpdatedBy = input.Actor
	record.UpdatedAt = now
	record.EnvironmentID = envID
	if status == MenuStatusPublished {
		published := now
		record.PublishedAt = &published
	}

	if exists {
		updated, err := s.viewProfiles.Update(ctx, record)
		if err != nil {
			return nil, err
		}
		return updated, nil
	}
	return s.viewProfiles.Create(ctx, record)
}

func (s *service) ResolveLocationBinding(ctx context.Context, location, locale string, opts MenuQueryOptions, env ...string) (*MenuLocationBinding, error) {
	candidates, err := s.resolveLocationBindings(ctx, location, locale, opts, env...)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, ErrMenuLocationBindingNotFound
	}
	return candidates[0], nil
}

func (s *service) MenuByCode(ctx context.Context, code string, locale string, opts MenuQueryOptions, env ...string) (*ResolvedMenu, error) {
	menu, err := s.GetMenuByCode(ctx, code, env...)
	if err != nil {
		return nil, err
	}
	if !s.statusAllowed(menu.Status, opts) {
		return nil, ErrMenuNotFound
	}

	projected, profile, err := s.applyViewProfileWithRecord(ctx, menu, opts.ViewProfile, opts, env...)
	if err != nil {
		return nil, err
	}
	nodes, err := s.navigationNodesForMenu(ctx, projected, locale)
	if err != nil {
		return nil, err
	}

	out := &ResolvedMenu{
		Location:        menu.Location,
		RequestedLocale: strings.TrimSpace(locale),
		ResolvedLocale:  strings.TrimSpace(locale),
		Menu:            projected,
		Menus:           []*Menu{projected},
		ViewProfile:     profile,
		Items:           nodes,
		Preview: ResolvedMenuPreview{
			IncludeDrafts:       opts.IncludeDrafts,
			PreviewTokenPresent: strings.TrimSpace(opts.PreviewToken) != "",
			MenuStatus:          strings.TrimSpace(menu.Status),
		},
		FamilyID: cloneUUIDPointer(menu.FamilyID),
	}
	if profile != nil {
		out.Preview.ViewProfileStatus = profile.Status
	}
	return out, nil
}

func (s *service) MenuByLocation(ctx context.Context, location string, locale string, opts MenuQueryOptions, env ...string) (*ResolvedMenu, error) {
	location = strings.TrimSpace(location)
	envID, _, err := s.resolveEnvironment(ctx, pickEnvironmentKey(env...))
	if err != nil {
		return nil, err
	}
	candidates, err := s.resolveLocationBindings(ctx, location, locale, opts, env...)
	if err != nil {
		return nil, err
	}

	var (
		bindings       []*MenuLocationBinding
		merged         []NavigationNode
		menusOut       []*Menu
		primaryMenu    *Menu
		primaryBinding *MenuLocationBinding
		primaryProfile *MenuViewProfile
		groupID        *uuid.UUID
	)

	if len(candidates) == 0 {
		// Backwards-compatible fallback to direct menu.location lookups.
		if menu, menuErr := s.GetMenuByLocation(ctx, location, env...); menuErr == nil {
			resolved, resolveErr := s.MenuByCode(ctx, menu.Code, locale, opts, env...)
			if resolveErr != nil {
				return nil, resolveErr
			}
			if resolved != nil {
				merged = append(merged, resolved.Items...)
				menusOut = append(menusOut, resolved.Menus...)
				primaryMenu = resolved.Menu
				primaryBinding = resolved.Binding
				primaryProfile = resolved.ViewProfile
				groupID = cloneUUIDPointer(resolved.FamilyID)
			}
		} else if !errors.Is(menuErr, ErrMenuNotFound) {
			return nil, menuErr
		}
	} else {
		bindings = candidates
		if s.effectiveBindingPolicy(opts) == MenuBindingPolicySingle && len(bindings) > 1 {
			bindings = bindings[:1]
		}

		seen := map[string]struct{}{}
		merged = make([]NavigationNode, 0)
		menusOut = make([]*Menu, 0, len(bindings))
		for _, binding := range bindings {
			menu, err := s.GetMenuByCode(ctx, binding.MenuCode, env...)
			if err != nil {
				continue
			}
			if !s.statusAllowed(menu.Status, opts) {
				continue
			}

			profileCode := opts.ViewProfile
			if strings.TrimSpace(profileCode) == "" && binding.ViewProfileCode != nil {
				profileCode = strings.TrimSpace(*binding.ViewProfileCode)
			}
			projected, profile, err := s.applyViewProfileWithRecord(ctx, menu, profileCode, opts, env...)
			if err != nil {
				return nil, err
			}
			nodes, err := s.navigationNodesForMenu(ctx, projected, locale)
			if err != nil {
				return nil, err
			}

			if primaryBinding == nil {
				primaryBinding = cloneMenuLocationBinding(binding)
				primaryMenu = projected
				primaryProfile = profile
				if menu.FamilyID != nil {
					groupID = cloneUUIDPointer(menu.FamilyID)
				}
			}
			menusOut = append(menusOut, projected)
			for _, node := range nodes {
				key := strings.TrimSpace(node.URL) + "|" + strings.TrimSpace(node.Label)
				if key == "|" {
					key = node.ID.String()
				}
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				merged = append(merged, node)
			}
		}
	}

	var memberships []ContentMenuMembership
	if s.includeContributions(opts) {
		contributions, computedMemberships, contributionPolicy, err := s.resolveContentContributions(ctx, location, locale, opts, envID)
		if err != nil {
			return nil, err
		}
		if len(contributions) > 0 {
			merged = s.mergeContributionNodes(merged, contributions, opts, contributionPolicy)
		}
		memberships = computedMemberships
	}

	if len(merged) == 0 {
		return nil, ErrMenuNotFound
	}
	out := &ResolvedMenu{
		Location:          location,
		RequestedLocale:   strings.TrimSpace(locale),
		ResolvedLocale:    strings.TrimSpace(locale),
		Menu:              primaryMenu,
		Menus:             menusOut,
		Binding:           primaryBinding,
		Bindings:          bindings,
		ViewProfile:       primaryProfile,
		Items:             merged,
		ContentMembership: memberships,
		Preview: ResolvedMenuPreview{
			IncludeDrafts:       opts.IncludeDrafts,
			PreviewTokenPresent: strings.TrimSpace(opts.PreviewToken) != "",
		},
		FamilyID: groupID,
	}
	if primaryMenu != nil {
		out.Preview.MenuStatus = primaryMenu.Status
	}
	if primaryBinding != nil {
		out.Preview.BindingStatus = primaryBinding.Status
	}
	if primaryProfile != nil {
		out.Preview.ViewProfileStatus = primaryProfile.Status
	}
	return out, nil
}

func (s *service) ApplyViewProfile(ctx context.Context, menu *Menu, profileCode string, opts MenuQueryOptions, env ...string) (*Menu, error) {
	projected, _, err := s.applyViewProfileWithRecord(ctx, menu, profileCode, opts, env...)
	return projected, err
}

// GetMenu retrieves a menu by ID including its hierarchical items.
func (s *service) GetMenu(ctx context.Context, id uuid.UUID) (*Menu, error) {
	menu, err := s.menus.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrMenuNotFound) {
			return nil, err
		}
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}
	if err := s.ensureEnvironmentActive(ctx, menu.EnvironmentID); err != nil {
		return nil, err
	}
	return s.hydrateMenu(ctx, menu)
}

// GetMenuByCode retrieves a menu using its code.
func (s *service) GetMenuByCode(ctx context.Context, code string, env ...string) (*Menu, error) {
	envID, _, err := s.resolveEnvironment(ctx, pickEnvironmentKey(env...))
	if err != nil {
		return nil, err
	}
	menu, err := s.menus.GetByCode(ctx, strings.TrimSpace(code), envID.String())
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}
	return s.hydrateMenu(ctx, menu)
}

// GetMenuByLocation retrieves a menu using its assigned location.
func (s *service) GetMenuByLocation(ctx context.Context, location string, env ...string) (*Menu, error) {
	envID, _, err := s.resolveEnvironment(ctx, pickEnvironmentKey(env...))
	if err != nil {
		return nil, err
	}
	menu, err := s.menus.GetByLocation(ctx, strings.TrimSpace(location), envID.String())
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}
	return s.hydrateMenu(ctx, menu)
}

// DeleteMenu removes a menu after enforcing usage guard rails.
func (s *service) DeleteMenu(ctx context.Context, req DeleteMenuRequest) error {
	if req.MenuID == uuid.Nil {
		return ErrMenuNotFound
	}

	menu, err := s.menus.GetByID(ctx, req.MenuID)
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return ErrMenuNotFound
		}
		return err
	}

	if err := s.ensureMenuDeletionAllowed(ctx, menu.ID, req.Force); err != nil {
		return err
	}

	items, err := s.items.ListByMenu(ctx, menu.ID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ParentID != nil {
			continue
		}
		if err := s.deleteMenuItemRecursive(ctx, item, req.DeletedBy, true); err != nil {
			return err
		}
	}

	// Safety net in case of corrupted hierarchies.
	remaining, err := s.items.ListByMenu(ctx, menu.ID)
	if err != nil {
		return err
	}
	for _, item := range remaining {
		if err := s.deleteMenuItemRecursive(ctx, item, req.DeletedBy, true); err != nil {
			if errors.Is(err, ErrMenuItemNotFound) {
				continue
			}
			return err
		}
	}

	if err := s.menus.Delete(ctx, menu.ID); err != nil {
		return err
	}

	if err := s.InvalidateCache(ctx); err != nil {
		return err
	}
	meta := map[string]any{
		"code": menu.Code,
	}
	if menu.EnvironmentID != uuid.Nil {
		meta["environment_id"] = menu.EnvironmentID.String()
	}
	s.emitActivity(ctx, req.DeletedBy, "delete", "menu", menu.ID, meta)
	return nil
}

func (s *service) ResetMenuByCode(ctx context.Context, code string, actor uuid.UUID, force bool, env ...string) error {
	menuCode := strings.TrimSpace(code)
	if menuCode == "" {
		return ErrMenuCodeRequired
	}

	envID, _, err := s.resolveEnvironment(ctx, pickEnvironmentKey(env...))
	if err != nil {
		return err
	}
	menu, err := s.menus.GetByCode(ctx, menuCode, envID.String())
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return ErrMenuNotFound
		}
		return err
	}

	if err := s.ensureMenuDeletionAllowed(ctx, menu.ID, force); err != nil {
		s.emitMenuResetAudit(ctx, actor, menu, force, nil, err)
		return err
	}

	counts, err := s.resetMenuContents(ctx, menu.ID)
	if err != nil {
		s.emitMenuResetAudit(ctx, actor, menu, force, nil, err)
		return err
	}

	if err := s.InvalidateCache(ctx); err != nil {
		return err
	}

	meta := map[string]any{
		"code":                 menu.Code,
		"force":                force,
		"strategy":             "contents_only",
		"items_deleted":        counts.ItemsDeleted,
		"translations_deleted": counts.TranslationsDeleted,
	}
	if menu.EnvironmentID != uuid.Nil {
		meta["environment_id"] = menu.EnvironmentID.String()
	}
	s.emitActivity(ctx, actor, "reset", "menu", menu.ID, meta)
	s.emitMenuResetAudit(ctx, actor, menu, force, &counts, nil)

	return nil
}

// AddMenuItem registers a new menu item at the specified position.
func (s *service) AddMenuItem(ctx context.Context, input AddMenuItemInput) (*MenuItem, error) {
	menu, err := s.menus.GetByID(ctx, input.MenuID)
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}
	if err := s.ensureEnvironmentActive(ctx, menu.EnvironmentID); err != nil {
		return nil, err
	}
	envID := menu.EnvironmentID
	if envID == uuid.Nil {
		resolvedID, _, err := s.resolveEnvironment(ctx, "")
		if err != nil {
			return nil, err
		}
		envID = resolvedID
	}

	itemType, err := normalizeMenuItemTypeValue(input.Type)
	if err != nil {
		return nil, ErrMenuItemTypeInvalid
	}

	if itemType != MenuItemTypeSeparator && s.translationsRequired() && len(input.Translations) == 0 && !input.AllowMissingTranslations {
		return nil, ErrMenuItemTranslations
	}
	if itemType == MenuItemTypeSeparator && len(input.Translations) > 0 {
		return nil, ErrMenuItemSeparatorFields
	}

	if input.Position < 0 {
		return nil, ErrMenuItemPosition
	}

	target, err := s.normalizeTargetForType(ctx, itemType, input.Target, envID)
	if err != nil {
		return nil, err
	}

	parentID, parentRef, err := s.resolveParent(ctx, input, s.forgivingBootstrap)
	if err != nil {
		return nil, err
	}
	if parentID != nil {
		parent, err := s.items.GetByID(ctx, *parentID)
		if err != nil {
			var notFound *NotFoundError
			if errors.As(err, &notFound) {
				return nil, ErrMenuItemParentInvalid
			}
			return nil, err
		}
		if parent.MenuID != menu.ID {
			return nil, ErrMenuItemParentInvalid
		}
		if normalizeMenuItemTypeValueOrDefault(parent.Type) == MenuItemTypeSeparator {
			return nil, ErrMenuItemParentUnsupported
		}
	}

	normalizedInput := input
	normalizedInput.Type = itemType
	normalizedInput.Target = target
	normalizedInput.ParentID = parentID
	normalizedInput.ExternalCode = normalizeExternalCode(input.ExternalCode)
	canonicalKey := deriveCanonicalKeyFromInput(itemType, target, normalizedInput, parentID, parentRef)
	if key := canonicalKeyForExternalCode(normalizedInput.ExternalCode); key != nil {
		canonicalKey = key
	}
	if trimmed := strings.TrimSpace(input.CanonicalKey); trimmed != "" {
		canonicalKey = &trimmed
	}
	if canonicalKey != nil {
		normalizedInput.CanonicalKey = *canonicalKey
	}
	itemID := s.pickMenuItemID(normalizedInput)
	if s.forgivingBootstrap && input.ID == nil && canonicalKey != nil {
		itemID = s.deterministicUUID("go-cms:menu_item:" + menu.ID.String() + ":" + *canonicalKey)
	}
	if s.maxDepth > 0 {
		records, err := s.items.ListByMenu(ctx, menu.ID)
		if err != nil {
			return nil, err
		}
		parents := make(map[uuid.UUID]*uuid.UUID, len(records)+1)
		for _, record := range records {
			if record == nil {
				continue
			}
			parents[record.ID] = normalizeUUIDPtr(record.ParentID)
		}
		parents[itemID] = normalizeUUIDPtr(parentID)
		if hierarchyDepthExceeded(parents, s.maxDepth) {
			return nil, ErrMenuItemDepthExceeded
		}
	}

	if canonicalKey != nil {
		existing, err := s.items.GetByMenuAndCanonicalKey(ctx, menu.ID, *canonicalKey)
		if err == nil && existing != nil {
			merged, err := s.mergeTranslations(ctx, existing, itemType, input.Translations)
			if err != nil {
				return nil, err
			}
			return merged, nil
		}
		var notFound *NotFoundError
		if err != nil && !errors.As(err, &notFound) {
			return nil, err
		}
	}

	// Re-index siblings to make room.
	siblings, err := s.fetchSiblings(ctx, menu.ID, parentID)
	if err != nil {
		return nil, err
	}
	insertAt := min(input.Position, len(siblings))
	if err := s.shiftSiblings(ctx, siblings, insertAt); err != nil {
		return nil, err
	}

	if input.Collapsed && !input.Collapsible {
		return nil, ErrMenuItemCollapsedWithoutCollapsible
	}
	if err := validateMenuItemSemantics(menuItemSemantics{
		Type:             itemType,
		Target:           target,
		Icon:             strings.TrimSpace(input.Icon),
		Badge:            input.Badge,
		TranslationCount: len(input.Translations),
		Collapsible:      input.Collapsible,
		Collapsed:        input.Collapsed,
	}, false, s.forgivingBootstrap); err != nil {
		return nil, err
	}

	now := s.now()
	item := &MenuItem{
		ID:            itemID,
		MenuID:        menu.ID,
		ParentID:      parentID,
		ParentRef:     normalizeParentRefPointer(parentRef),
		ExternalCode:  normalizedInput.ExternalCode,
		Position:      insertAt,
		Type:          itemType,
		Target:        ensureNonNilTarget(target),
		Icon:          strings.TrimSpace(input.Icon),
		Badge:         cloneMapAny(input.Badge),
		Permissions:   cloneStringSlice(input.Permissions),
		Classes:       cloneStringSlice(input.Classes),
		Styles:        cloneMapString(input.Styles),
		Collapsible:   itemType != MenuItemTypeSeparator && input.Collapsible,
		Collapsed:     itemType != MenuItemTypeSeparator && input.Collapsible && input.Collapsed,
		Metadata:      ensureMapAny(input.Metadata),
		EnvironmentID: envID,
		CreatedBy:     input.CreatedBy,
		UpdatedBy:     input.UpdatedBy,
		CreatedAt:     now,
		UpdatedAt:     now,
		CanonicalKey:  canonicalKey,
	}

	created, err := s.items.Create(ctx, item)
	if err != nil {
		return nil, err
	}

	var trs []*MenuItemTranslation
	if itemType != MenuItemTypeSeparator && len(input.Translations) > 0 {
		trs, err = s.attachTranslations(ctx, created.ID, itemType, input.Translations)
		if err != nil {
			return nil, err
		}
	}
	created.Translations = trs
	meta := map[string]any{
		"menu_id":   created.MenuID.String(),
		"menu_code": menu.Code,
		"position":  created.Position,
		"parent_id": created.ParentID,
		"locales":   collectMenuLocalesFromInputs(input.Translations),
	}
	if menu.EnvironmentID != uuid.Nil {
		meta["environment_id"] = menu.EnvironmentID.String()
	}
	s.emitActivity(ctx, pickActor(input.CreatedBy, input.UpdatedBy), "create", "menu_item", created.ID, meta)

	if s.forgivingBootstrap {
		actor := pickActor(input.UpdatedBy, input.CreatedBy)
		if _, err := s.ReconcileMenu(ctx, ReconcileMenuRequest{MenuID: menu.ID, UpdatedBy: actor}); err != nil {
			return nil, err
		}
	}
	return created, nil
}

// UpsertMenuItem creates or returns an existing item using canonical identity (menu_id + canonical_key).
// When the menu service is configured with forgiving bootstrap mode, missing parents are stored as parent_ref
// and resolved later by reconciliation.
func (s *service) UpsertMenuItem(ctx context.Context, input UpsertMenuItemInput) (*MenuItem, error) {
	actor := input.Actor
	if actor == uuid.Nil {
		actor = uuid.Nil
	}

	var menu *Menu
	if input.MenuID != nil && *input.MenuID != uuid.Nil {
		record, err := s.menus.GetByID(ctx, *input.MenuID)
		if err != nil {
			var notFound *NotFoundError
			if errors.As(err, &notFound) {
				return nil, ErrMenuNotFound
			}
			return nil, err
		}
		menu = record
	} else {
		code := strings.TrimSpace(input.MenuCode)
		if code == "" {
			return nil, ErrMenuCodeRequired
		}
		record, err := s.GetOrCreateMenu(ctx, CreateMenuInput{
			Code:           code,
			Description:    input.MenuDescription,
			CreatedBy:      actor,
			UpdatedBy:      actor,
			EnvironmentKey: input.EnvironmentKey,
		})
		if err != nil {
			return nil, err
		}
		menu = record
	}

	position := int(^uint(0) >> 1) // clamp-to-append default for new items when Position is nil
	if input.Position != nil {
		position = *input.Position
	}

	add := AddMenuItemInput{
		MenuID:                   menu.ID,
		ParentID:                 input.ParentID,
		ParentCode:               input.ParentCode,
		ExternalCode:             input.ExternalCode,
		CanonicalKey:             input.CanonicalKey,
		Position:                 position,
		Type:                     input.Type,
		Target:                   input.Target,
		Icon:                     input.Icon,
		Badge:                    input.Badge,
		Permissions:              input.Permissions,
		Classes:                  input.Classes,
		Styles:                   input.Styles,
		Collapsible:              input.Collapsible,
		Collapsed:                input.Collapsed,
		Metadata:                 input.Metadata,
		CreatedBy:                actor,
		UpdatedBy:                actor,
		Translations:             input.Translations,
		AllowMissingTranslations: input.AllowMissingTranslations,
	}

	return s.AddMenuItem(ctx, add)
}

type menuContentsResetter interface {
	ResetMenuContents(ctx context.Context, menuID uuid.UUID) (itemsDeleted int, translationsDeleted int, err error)
}

func (s *service) resetMenuContents(ctx context.Context, menuID uuid.UUID) (ResetMenuCounts, error) {
	if resetter, ok := s.items.(menuContentsResetter); ok {
		itemsDeleted, translationsDeleted, err := resetter.ResetMenuContents(ctx, menuID)
		if err != nil {
			return ResetMenuCounts{}, err
		}
		return ResetMenuCounts{ItemsDeleted: itemsDeleted, TranslationsDeleted: translationsDeleted}, nil
	}

	items, err := s.items.ListByMenu(ctx, menuID)
	if err != nil {
		return ResetMenuCounts{}, err
	}

	var translationsDeleted int
	for _, item := range items {
		translations, err := s.translations.ListByMenuItem(ctx, item.ID)
		if err != nil {
			return ResetMenuCounts{}, err
		}
		for _, tr := range translations {
			if err := s.translations.Delete(ctx, tr.ID); err != nil {
				return ResetMenuCounts{}, err
			}
			translationsDeleted++
		}
	}

	var itemsDeleted int
	for _, item := range items {
		if err := s.items.Delete(ctx, item.ID); err != nil {
			var notFound *NotFoundError
			if errors.As(err, &notFound) {
				continue
			}
			return ResetMenuCounts{}, err
		}
		itemsDeleted++
	}

	return ResetMenuCounts{ItemsDeleted: itemsDeleted, TranslationsDeleted: translationsDeleted}, nil
}

func (s *service) emitMenuResetAudit(ctx context.Context, actor uuid.UUID, menu *Menu, force bool, counts *ResetMenuCounts, resetErr error) {
	if s.audit == nil || menu == nil || menu.ID == uuid.Nil {
		return
	}

	metadata := map[string]any{
		"actor":    actor.String(),
		"code":     menu.Code,
		"menu_id":  menu.ID.String(),
		"force":    force,
		"strategy": "contents_only",
	}

	action := "menu_reset"
	if counts != nil {
		metadata["items_deleted"] = counts.ItemsDeleted
		metadata["translations_deleted"] = counts.TranslationsDeleted
	}

	if resetErr != nil {
		action = "menu_reset_failed"
		var inUse *MenuInUseError
		if errors.As(resetErr, &inUse) {
			action = "menu_reset_blocked"
			metadata["bindings"] = len(inUse.Bindings)
		}
		metadata["error"] = resetErr.Error()
	}

	_ = s.audit.Record(ctx, jobs.AuditEvent{
		EntityType: "menu",
		EntityID:   menu.ID.String(),
		Action:     action,
		OccurredAt: s.now().UTC(),
		Metadata:   metadata,
	})
}

// UpdateMenuItem mutates supported fields on an existing item.
func (s *service) UpdateMenuItem(ctx context.Context, input UpdateMenuItemInput) (*MenuItem, error) {
	if input.ItemID == uuid.Nil {
		return nil, ErrMenuItemNotFound
	}

	item, err := s.items.GetByID(ctx, input.ItemID)
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuItemNotFound
		}
		return nil, err
	}
	menu, err := s.menus.GetByID(ctx, item.MenuID)
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}
	envID := menu.EnvironmentID
	if envID == uuid.Nil {
		resolvedID, _, err := s.resolveEnvironment(ctx, "")
		if err != nil {
			return nil, err
		}
		envID = resolvedID
	}

	originalPosition := item.Position
	var originalParent *uuid.UUID
	if item.ParentID != nil {
		parentCopy := *item.ParentID
		originalParent = &parentCopy
	}

	currentType := normalizeMenuItemTypeValueOrDefault(item.Type)
	targetType := currentType
	if input.Type != nil {
		var typeErr error
		targetType, typeErr = normalizeMenuItemTypeValue(*input.Type)
		if typeErr != nil {
			return nil, ErrMenuItemTypeInvalid
		}
	}

	if input.Collapsed != nil && *input.Collapsed && input.Collapsible != nil && !*input.Collapsible {
		return nil, ErrMenuItemCollapsedWithoutCollapsible
	}

	translations, err := s.translations.ListByMenuItem(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	item.Translations = translations
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}

	if input.Target != nil {
		target, err := s.normalizeTargetForType(ctx, targetType, input.Target, envID)
		if err != nil {
			return nil, err
		}
		item.Target = target
	} else if targetType != currentType && targetType != MenuItemTypeItem {
		item.Target = map[string]any{}
	}

	if input.Icon != nil {
		item.Icon = strings.TrimSpace(*input.Icon)
	} else if targetType == MenuItemTypeGroup || targetType == MenuItemTypeSeparator {
		item.Icon = ""
	}

	if input.Badge != nil {
		item.Badge = cloneMapAny(input.Badge)
	} else if targetType == MenuItemTypeGroup || targetType == MenuItemTypeSeparator {
		item.Badge = nil
	}

	if input.Permissions != nil {
		item.Permissions = cloneStringSlice(input.Permissions)
	}
	if input.Classes != nil {
		item.Classes = cloneStringSlice(input.Classes)
	}
	if input.Styles != nil {
		item.Styles = cloneMapString(input.Styles)
	}
	if input.Metadata != nil {
		item.Metadata = ensureMapAny(input.Metadata)
	}

	if input.Collapsible != nil {
		item.Collapsible = *input.Collapsible
	}
	if input.Collapsed != nil {
		item.Collapsed = *input.Collapsed
	}

	if input.Position != nil {
		if *input.Position < 0 {
			return nil, ErrMenuItemPosition
		}
		siblings, err := s.fetchSiblings(ctx, item.MenuID, item.ParentID)
		if err != nil {
			return nil, err
		}
		desired := min(*input.Position, len(siblings))
		if err := s.repositionItem(ctx, item, siblings, desired); err != nil {
			return nil, err
		}
		item.Position = desired
	}

	if input.ParentID != nil {
		parentID := input.ParentID
		if parentID != nil && *parentID != uuid.Nil {
			parent, err := s.items.GetByID(ctx, *parentID)
			if err != nil {
				var notFound *NotFoundError
				if errors.As(err, &notFound) {
					return nil, ErrMenuItemParentInvalid
				}
				return nil, err
			}
			if parent.MenuID != item.MenuID {
				return nil, ErrMenuItemParentInvalid
			}
			if normalizeMenuItemTypeValueOrDefault(parent.Type) == MenuItemTypeSeparator {
				return nil, ErrMenuItemParentUnsupported
			}
		}
		item.ParentID = parentID
	}

	if s.maxDepth > 0 {
		records, err := s.items.ListByMenu(ctx, item.MenuID)
		if err != nil {
			return nil, err
		}
		parents := make(map[uuid.UUID]*uuid.UUID, len(records))
		for _, record := range records {
			if record == nil {
				continue
			}
			parents[record.ID] = normalizeUUIDPtr(record.ParentID)
		}
		parents[item.ID] = normalizeUUIDPtr(item.ParentID)
		if hasCycle(parents) {
			return nil, ErrMenuItemCycle
		}
		if hierarchyDepthExceeded(parents, s.maxDepth) {
			return nil, ErrMenuItemDepthExceeded
		}
	}

	var hasChildren bool
	if targetType == MenuItemTypeSeparator || item.Collapsible {
		children, err := s.items.ListChildren(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		hasChildren = len(children) > 0
		if targetType == MenuItemTypeSeparator && hasChildren {
			return nil, ErrMenuItemSeparatorFields
		}
	}

	if targetType == MenuItemTypeSeparator {
		item.Collapsible = false
		item.Collapsed = false
	}
	if item.Collapsed && !item.Collapsible {
		return nil, ErrMenuItemCollapsedWithoutCollapsible
	}
	if err := validateMenuItemSemantics(menuItemSemantics{
		Type:             targetType,
		Target:           item.Target,
		Icon:             item.Icon,
		Badge:            item.Badge,
		TranslationCount: len(item.Translations),
		Collapsible:      item.Collapsible,
		Collapsed:        item.Collapsed,
	}, hasChildren, s.forgivingBootstrap); err != nil {
		return nil, err
	}

	item.Type = targetType
	item.Target = ensureNonNilTarget(item.Target)
	item.Metadata = ensureMapAny(item.Metadata)
	item.CanonicalKey = deriveCanonicalKeyFromMenuItem(item)
	item.UpdatedBy = input.UpdatedBy
	item.UpdatedAt = s.now()
	updated, err := s.items.Update(ctx, item)
	if err != nil {
		return nil, err
	}

	updated.Translations = translations

	verb := "update"
	if originalParent == nil && updated.ParentID != nil ||
		originalParent != nil && (updated.ParentID == nil || *originalParent != *updated.ParentID) ||
		originalPosition != updated.Position {
		verb = "reorder"
	}

	meta := map[string]any{
		"menu_id":   updated.MenuID.String(),
		"position":  updated.Position,
		"parent_id": updated.ParentID,
		"locales":   collectMenuLocalesFromTranslations(updated.Translations),
	}
	if menu.EnvironmentID != uuid.Nil {
		meta["environment_id"] = menu.EnvironmentID.String()
	}
	s.emitActivity(ctx, input.UpdatedBy, verb, "menu_item", updated.ID, meta)

	return updated, nil
}

// ReconcileMenu resolves deferred parent references for a menu.
func (s *service) ReconcileMenu(ctx context.Context, req ReconcileMenuRequest) (*ReconcileResult, error) {
	if req.MenuID == uuid.Nil {
		return nil, ErrMenuNotFound
	}
	if _, err := s.menus.GetByID(ctx, req.MenuID); err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}

	items, err := s.items.ListByMenu(ctx, req.MenuID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return &ReconcileResult{}, nil
	}

	byID := make(map[uuid.UUID]*MenuItem, len(items))
	byCanonical := make(map[string]*MenuItem, len(items))
	byExternal := make(map[string]*MenuItem, len(items))
	parents := make(map[uuid.UUID]*uuid.UUID, len(items))

	for _, item := range items {
		if item == nil {
			continue
		}
		byID[item.ID] = item
		if item.CanonicalKey != nil && strings.TrimSpace(*item.CanonicalKey) != "" {
			byCanonical[strings.TrimSpace(*item.CanonicalKey)] = item
		}
		if code := normalizeExternalCode(item.ExternalCode); code != "" {
			byExternal[code] = item
		}
		if item.ParentID != nil {
			parent := *item.ParentID
			parents[item.ID] = &parent
		} else {
			parents[item.ID] = nil
		}
	}

	var resolved int
	touched := make(map[uuid.UUID]struct{})
	affectedParents := make(map[string]struct{})
	for _, item := range items {
		if item == nil || item.ParentID != nil || item.ParentRef == nil {
			continue
		}
		ref := strings.TrimSpace(*item.ParentRef)
		if ref == "" {
			continue
		}

		parent := resolveMenuItemRef(ref, byID, byCanonical, byExternal)
		if parent == nil {
			continue
		}
		if parent.ID == item.ID {
			return nil, ErrMenuItemCycle
		}
		if normalizeMenuItemTypeValueOrDefault(parent.Type) == MenuItemTypeSeparator {
			return nil, ErrMenuItemParentUnsupported
		}

		parentID := parent.ID
		affectedParents[parentKey(item.ParentID)] = struct{}{}
		affectedParents[parentKey(&parentID)] = struct{}{}
		parents[item.ID] = &parentID
		item.ParentID = &parentID
		item.ParentRef = nil
		touched[item.ID] = struct{}{}
		resolved++
	}

	if resolved == 0 {
		remaining := countPendingParentRefs(items)
		return &ReconcileResult{Resolved: 0, Remaining: remaining}, nil
	}

	if hasCycle(parents) {
		return nil, ErrMenuItemCycle
	}
	if s.maxDepth > 0 && hierarchyDepthExceeded(parents, s.maxDepth) {
		return nil, ErrMenuItemDepthExceeded
	}

	dirty := normalizeMenuItemPositionsForParents(items, affectedParents, touched)
	now := s.now()
	for _, item := range dirty {
		item.UpdatedAt = now
		item.UpdatedBy = req.UpdatedBy
	}

	if err := s.items.BulkUpdateParentLinks(ctx, dirty); err != nil {
		return nil, err
	}
	if err := s.InvalidateCache(ctx); err != nil {
		return nil, err
	}

	remaining := countPendingParentRefs(items)
	return &ReconcileResult{Resolved: resolved, Remaining: remaining}, nil
}

// DeleteMenuItem removes the requested menu item, optionally cascading to children.
func (s *service) DeleteMenuItem(ctx context.Context, req DeleteMenuItemRequest) error {
	if req.ItemID == uuid.Nil {
		return ErrMenuItemNotFound
	}

	item, err := s.items.GetByID(ctx, req.ItemID)
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return ErrMenuItemNotFound
		}
		return err
	}

	if err := s.deleteMenuItemRecursive(ctx, item, req.DeletedBy, req.CascadeChildren); err != nil {
		return err
	}
	meta := map[string]any{
		"menu_id":   item.MenuID.String(),
		"parent_id": item.ParentID,
		"position":  item.Position,
	}
	if menu, err := s.menus.GetByID(ctx, item.MenuID); err == nil && menu != nil && menu.EnvironmentID != uuid.Nil {
		meta["environment_id"] = menu.EnvironmentID.String()
	}
	s.emitActivity(ctx, req.DeletedBy, "delete", "menu_item", item.ID, meta)

	return s.InvalidateCache(ctx)
}

// BulkReorderMenuItems overwrites the hierarchy/positions for a menu's items atomically.
func (s *service) BulkReorderMenuItems(ctx context.Context, input BulkReorderMenuItemsInput) ([]*MenuItem, error) {
	if input.MenuID == uuid.Nil {
		return nil, ErrMenuNotFound
	}

	if _, err := s.menus.GetByID(ctx, input.MenuID); err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}

	items, err := s.items.ListByMenu(ctx, input.MenuID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}

	if len(input.Items) != len(items) {
		return nil, fmt.Errorf("menus: reorder requires %d items, got %d", len(items), len(input.Items))
	}

	itemIndex := make(map[uuid.UUID]*MenuItem, len(items))
	for _, item := range items {
		itemIndex[item.ID] = item
	}

	parentMap := make(map[uuid.UUID]*uuid.UUID, len(items))
	positionMap := make(map[string][]ItemOrder)
	seen := make(map[uuid.UUID]struct{}, len(items))

	for _, entry := range input.Items {
		if entry.ItemID == uuid.Nil {
			return nil, ErrMenuItemNotFound
		}
		if entry.Position < 0 {
			return nil, ErrMenuItemPosition
		}
		if _, ok := itemIndex[entry.ItemID]; !ok {
			return nil, ErrMenuItemNotFound
		}
		if entry.ParentID != nil {
			if *entry.ParentID == entry.ItemID {
				return nil, ErrMenuItemCycle
			}
			parent, ok := itemIndex[*entry.ParentID]
			if !ok || parent.MenuID != input.MenuID {
				return nil, ErrMenuItemParentInvalid
			}
			if normalizeMenuItemTypeValueOrDefault(parent.Type) == MenuItemTypeSeparator {
				return nil, ErrMenuItemParentUnsupported
			}
		}

		parentMap[entry.ItemID] = entry.ParentID
		parentKey := parentKey(entry.ParentID)
		positionMap[parentKey] = append(positionMap[parentKey], entry)
		if _, dup := seen[entry.ItemID]; dup {
			return nil, fmt.Errorf("menus: duplicate item %s in reorder request", entry.ItemID)
		}
		seen[entry.ItemID] = struct{}{}
	}

	if hasCycle(parentMap) {
		return nil, ErrMenuItemCycle
	}
	if s.maxDepth > 0 && hierarchyDepthExceeded(parentMap, s.maxDepth) {
		return nil, ErrMenuItemDepthExceeded
	}

	dirty := make([]*MenuItem, 0, len(items))
	// Apply ordering per parent
	for key, entries := range positionMap {
		slices.SortFunc(entries, func(a, b ItemOrder) int {
			return a.Position - b.Position
		})
		for idx, entry := range entries {
			item := itemIndex[entry.ItemID]
			parent := normalizeUUIDPtr(entry.ParentID)
			needsUpdate := item.Position != idx || !uuidPtrEqual(item.ParentID, parent)
			item.ParentID = parent
			item.Position = idx
			item.UpdatedAt = s.now()
			item.UpdatedBy = input.UpdatedBy
			if needsUpdate {
				dirty = append(dirty, item)
			}
		}
		// Update map after reorder
		positionMap[key] = entries
	}

	if len(dirty) > 0 {
		if err := s.items.BulkUpdateHierarchy(ctx, dirty); err != nil {
			return nil, err
		}
	}

	// Return items ordered by parent and position for convenience.
	result := slices.Clone(items)
	slices.SortFunc(result, func(a, b *MenuItem) int {
		if parentKey(a.ParentID) == parentKey(b.ParentID) {
			return a.Position - b.Position
		}
		return strings.Compare(parentKey(a.ParentID), parentKey(b.ParentID))
	})

	meta := map[string]any{
		"menu_id": input.MenuID.String(),
		"count":   len(result),
	}
	if menu, err := s.menus.GetByID(ctx, input.MenuID); err == nil && menu != nil && menu.EnvironmentID != uuid.Nil {
		meta["environment_id"] = menu.EnvironmentID.String()
	}
	s.emitActivity(ctx, input.UpdatedBy, "reorder", "menu", input.MenuID, meta)

	return result, nil
}

// AddMenuItemTranslation registers a localized label for a menu item.
func (s *service) AddMenuItemTranslation(ctx context.Context, input AddMenuItemTranslationInput) (*MenuItemTranslation, error) {
	if input.ItemID == uuid.Nil {
		return nil, ErrMenuItemNotFound
	}

	item, err := s.items.GetByID(ctx, input.ItemID)
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuItemNotFound
		}
		return nil, err
	}
	itemType := normalizeMenuItemTypeValueOrDefault(item.Type)
	if itemType == MenuItemTypeSeparator {
		return nil, ErrMenuItemSeparatorFields
	}

	normalizedInput, err := normalizeMenuItemTranslationInput(itemType, MenuItemTranslationInput{
		Locale:        input.Locale,
		Label:         input.Label,
		LabelKey:      input.LabelKey,
		GroupTitle:    input.GroupTitle,
		GroupTitleKey: input.GroupTitleKey,
		URLOverride:   input.URLOverride,
	})
	if err != nil {
		return nil, err
	}

	menu, _ := s.menus.GetByID(ctx, item.MenuID)

	locale, err := s.lookupLocale(ctx, normalizedInput.Locale)
	if err != nil {
		return nil, err
	}

	if existing, err := s.translations.GetByMenuItemAndLocale(ctx, item.ID, locale.ID); err == nil && existing != nil {
		return nil, ErrTranslationExists
	}

	now := s.now()
	translation := &MenuItemTranslation{
		ID:            s.nextID(),
		MenuItemID:    item.ID,
		LocaleID:      locale.ID,
		Locale:        locale,
		Label:         normalizedInput.Label,
		LabelKey:      normalizedInput.LabelKey,
		GroupTitle:    normalizedInput.GroupTitle,
		GroupTitleKey: normalizedInput.GroupTitleKey,
		URLOverride:   normalizedInput.URLOverride,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	created, err := s.translations.Create(ctx, translation)
	if err != nil {
		return nil, err
	}

	meta := map[string]any{
		"menu_id": item.MenuID.String(),
		"item_id": item.ID.String(),
		"locale":  locale.Code,
	}
	if menu != nil {
		meta["menu_code"] = menu.Code
	}
	if normalizedInput.Label != "" {
		meta["label"] = normalizedInput.Label
	}
	if normalizedInput.LabelKey != "" {
		meta["label_key"] = normalizedInput.LabelKey
	}
	if normalizedInput.GroupTitle != "" {
		meta["group_title"] = normalizedInput.GroupTitle
	}
	if normalizedInput.GroupTitleKey != "" {
		meta["group_title_key"] = normalizedInput.GroupTitleKey
	}
	if menu != nil && menu.EnvironmentID != uuid.Nil {
		meta["environment_id"] = menu.EnvironmentID.String()
	}
	s.emitActivity(ctx, pickActor(item.UpdatedBy, item.CreatedBy), "create", "menu_item_translation", created.ID, meta)

	return created, nil
}

// UpsertMenuItemTranslation adds or updates localized metadata for an item.
func (s *service) UpsertMenuItemTranslation(ctx context.Context, input UpsertMenuItemTranslationInput) (*MenuItemTranslation, error) {
	if input.ItemID == uuid.Nil {
		return nil, ErrMenuItemNotFound
	}

	item, err := s.items.GetByID(ctx, input.ItemID)
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuItemNotFound
		}
		return nil, err
	}
	itemType := normalizeMenuItemTypeValueOrDefault(item.Type)
	if itemType == MenuItemTypeSeparator {
		return nil, ErrMenuItemSeparatorFields
	}

	normalizedInput, err := normalizeMenuItemTranslationInput(itemType, MenuItemTranslationInput{
		Locale:        input.Locale,
		Label:         input.Label,
		LabelKey:      input.LabelKey,
		GroupTitle:    input.GroupTitle,
		GroupTitleKey: input.GroupTitleKey,
		URLOverride:   input.URLOverride,
	})
	if err != nil {
		return nil, err
	}

	locale, err := s.lookupLocale(ctx, normalizedInput.Locale)
	if err != nil {
		return nil, err
	}

	now := s.now()

	existing, err := s.translations.GetByMenuItemAndLocale(ctx, item.ID, locale.ID)
	if err != nil {
		var notFound *NotFoundError
		if !errors.As(err, &notFound) {
			return nil, err
		}
		existing = nil
	}

	if existing == nil {
		translation := &MenuItemTranslation{
			ID:            s.nextID(),
			MenuItemID:    item.ID,
			LocaleID:      locale.ID,
			Locale:        locale,
			Label:         normalizedInput.Label,
			LabelKey:      normalizedInput.LabelKey,
			GroupTitle:    normalizedInput.GroupTitle,
			GroupTitleKey: normalizedInput.GroupTitleKey,
			URLOverride:   normalizedInput.URLOverride,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		created, err := s.translations.Create(ctx, translation)
		if err != nil {
			return nil, err
		}
		return created, nil
	}

	existing.Label = normalizedInput.Label
	existing.LabelKey = normalizedInput.LabelKey
	existing.GroupTitle = normalizedInput.GroupTitle
	existing.GroupTitleKey = normalizedInput.GroupTitleKey
	existing.URLOverride = normalizedInput.URLOverride
	existing.UpdatedAt = now
	updated, err := s.translations.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// GetMenuItemByExternalCode resolves a menu item by menu code and external code (stable path identifier).
func (s *service) GetMenuItemByExternalCode(ctx context.Context, menuCode string, externalCode string, env ...string) (*MenuItem, error) {
	code := strings.TrimSpace(menuCode)
	if code == "" {
		return nil, ErrMenuCodeRequired
	}
	envID, _, err := s.resolveEnvironment(ctx, pickEnvironmentKey(env...))
	if err != nil {
		return nil, err
	}
	menu, err := s.menus.GetByCode(ctx, code, envID.String())
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}

	ext := normalizeExternalCode(externalCode)
	if ext == "" {
		return nil, ErrMenuItemNotFound
	}

	item, err := s.items.GetByMenuAndExternalCode(ctx, menu.ID, ext)
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, ErrMenuItemNotFound
		}
		return nil, err
	}
	return item, nil
}

// ResolveNavigation builds a localized navigation tree for the requested menu.
func (s *service) ResolveNavigation(ctx context.Context, menuCode string, locale string, env ...string) ([]NavigationNode, error) {
	code := strings.TrimSpace(menuCode)
	envID, _, err := s.resolveEnvironment(ctx, pickEnvironmentKey(env...))
	if err != nil {
		return nil, err
	}
	if s.forgivingBootstrap && s.reconcileOnResolve && code != "" {
		if record, err := s.menus.GetByCode(ctx, code, envID.String()); err == nil && record != nil {
			if _, err := s.ReconcileMenu(ctx, ReconcileMenuRequest{MenuID: record.ID, UpdatedBy: uuid.Nil}); err != nil {
				return nil, err
			}
		}
	}

	resolved, err := s.MenuByCode(ctx, code, locale, MenuQueryOptions{}, env...)
	if err != nil {
		return nil, err
	}
	if resolved == nil || len(resolved.Items) == 0 {
		return nil, nil
	}
	_ = envID
	return resolved.Items, nil
}

// ResolveNavigationByLocation resolves navigation for a menu location.
func (s *service) ResolveNavigationByLocation(ctx context.Context, location string, locale string, env ...string) ([]NavigationNode, error) {
	resolved, err := s.MenuByLocation(ctx, location, locale, MenuQueryOptions{}, env...)
	if err != nil {
		return nil, err
	}
	return resolved.Items, nil
}

func (s *service) InvalidateCache(ctx context.Context) error {
	var errs []error

	if invalidator, ok := s.menus.(cacheInvalidator); ok {
		if err := invalidator.InvalidateCache(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if invalidator, ok := s.items.(cacheInvalidator); ok {
		if err := invalidator.InvalidateCache(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if invalidator, ok := s.translations.(cacheInvalidator); ok {
		if err := invalidator.InvalidateCache(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if invalidator, ok := s.bindings.(cacheInvalidator); ok {
		if err := invalidator.InvalidateCache(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if invalidator, ok := s.viewProfiles.(cacheInvalidator); ok {
		if err := invalidator.InvalidateCache(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *service) buildNavigationNode(ctx context.Context, menuCode string, item *MenuItem, envID uuid.UUID, localeID uuid.UUID, locale string) (NavigationNode, error) {
	node := NavigationNode{
		ID:          item.ID,
		Position:    item.Position,
		Type:        normalizeMenuItemTypeValueOrDefault(item.Type),
		Icon:        strings.TrimSpace(item.Icon),
		Badge:       cloneMapAny(item.Badge),
		Permissions: cloneStringSlice(item.Permissions),
		Classes:     cloneStringSlice(item.Classes),
		Styles:      cloneMapString(item.Styles),
		Collapsible: item.Collapsible,
		Collapsed:   item.Collapsed,
		Metadata:    cloneMapAny(item.Metadata),
	}
	if item.Target != nil {
		node.Target = maps.Clone(item.Target)
	}

	if node.Type == MenuItemTypeSeparator {
		return node, nil
	}

	primary, fallback := selectMenuTranslation(item.Translations, localeID)
	translation := primary
	if translation == nil {
		translation = fallback
	}

	var label, labelKey, groupTitle, groupTitleKey string
	if translation != nil {
		label = strings.TrimSpace(translation.Label)
		labelKey = strings.TrimSpace(translation.LabelKey)
		groupTitle = strings.TrimSpace(translation.GroupTitle)
		groupTitleKey = strings.TrimSpace(translation.GroupTitleKey)
		node.Label = label
		node.LabelKey = labelKey
		node.GroupTitle = groupTitle
		node.GroupTitleKey = groupTitleKey
		if node.Type == MenuItemTypeItem && translation.URLOverride != nil {
			if url := strings.TrimSpace(*translation.URLOverride); url != "" {
				node.URL = url
			}
		}
	}

	if node.URL == "" {
		node.URL = s.resolveNodeURL(ctx, menuCode, item, envID, localeID, locale)
	}

	if node.Type == MenuItemTypeGroup {
		if node.Label == "" {
			switch {
			case groupTitle != "":
				node.Label = groupTitle
			case groupTitleKey != "":
				node.Label = groupTitleKey
			case label != "":
				node.Label = label
			case labelKey != "":
				node.Label = labelKey
			}
		}
	} else if node.Type == MenuItemTypeItem {
		if node.Label == "" && labelKey != "" {
			node.Label = labelKey
		}
		if node.Label == "" {
			if slug, ok := extractSlug(item.Target); ok && slug != "" {
				node.Label = slug
			} else if translation != nil && translation.Label != "" {
				node.Label = translation.Label
			} else if labelKey != "" {
				node.Label = labelKey
			} else if targetType, ok := item.Target["type"].(string); ok {
				node.Label = targetType
			} else {
				node.Label = item.ID.String()
			}
		}
	}

	if len(item.Children) > 0 {
		children := make([]NavigationNode, 0, len(item.Children))
		for _, child := range item.Children {
			childNode, err := s.buildNavigationNode(ctx, menuCode, child, envID, localeID, locale)
			if err != nil {
				return NavigationNode{}, err
			}
			children = append(children, childNode)
		}
		node.Children = children
	}
	if len(node.Children) == 0 || node.Type == MenuItemTypeSeparator {
		node.Collapsible = false
		node.Collapsed = false
	} else if !node.Collapsible {
		node.Collapsed = false
	}

	return node, nil
}

func (s *service) hydrateMenu(ctx context.Context, menu *Menu) (*Menu, error) {
	items, err := s.items.ListByMenu(ctx, menu.ID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		menu.Items = nil
		return menu, nil
	}

	trMap := make(map[uuid.UUID][]*MenuItemTranslation, len(items))
	for _, item := range items {
		list, err := s.translations.ListByMenuItem(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		trMap[item.ID] = list
	}

	menu.Items = buildHierarchy(items, trMap)
	return menu, nil
}

func (s *service) resolveNodeURL(ctx context.Context, menuCode string, item *MenuItem, envID uuid.UUID, localeID uuid.UUID, locale string) string {
	if item == nil {
		return ""
	}
	if normalizeMenuItemTypeValueOrDefault(item.Type) != MenuItemTypeItem {
		return ""
	}
	if s.urlResolver != nil {
		url, err := s.urlResolver.Resolve(ctx, ResolveRequest{
			MenuCode: menuCode,
			Item:     item,
			Locale:   locale,
			LocaleID: localeID,
		})
		if err == nil {
			if trimmed := strings.TrimSpace(url); trimmed != "" {
				return trimmed
			}
		}
	}
	url, err := s.resolveURLForTarget(ctx, item.Target, envID, localeID)
	if err != nil {
		return ""
	}
	return url
}

func (s *service) attachTranslations(ctx context.Context, itemID uuid.UUID, itemType string, inputs []MenuItemTranslationInput) ([]*MenuItemTranslation, error) {
	seen := make(map[string]struct{}, len(inputs))
	translations := make([]*MenuItemTranslation, 0, len(inputs))
	now := s.now()

	for _, tr := range inputs {
		normalized, err := normalizeMenuItemTranslationInput(itemType, tr)
		if err != nil {
			return nil, err
		}

		localeCode := normalized.Locale
		if localeCode == "" {
			return nil, ErrUnknownLocale
		}
		if _, exists := seen[localeCode]; exists {
			return nil, ErrMenuItemDuplicateLocale
		}
		seen[localeCode] = struct{}{}

		locale, err := s.lookupLocale(ctx, localeCode)
		if err != nil {
			return nil, err
		}

		if existing, err := s.translations.GetByMenuItemAndLocale(ctx, itemID, locale.ID); err == nil && existing != nil {
			return nil, ErrTranslationExists
		}

		record := &MenuItemTranslation{
			ID:            s.nextID(),
			MenuItemID:    itemID,
			LocaleID:      locale.ID,
			Locale:        locale,
			Label:         normalized.Label,
			LabelKey:      normalized.LabelKey,
			GroupTitle:    normalized.GroupTitle,
			GroupTitleKey: normalized.GroupTitleKey,
			URLOverride:   normalized.URLOverride,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		created, err := s.translations.Create(ctx, record)
		if err != nil {
			return nil, err
		}
		translations = append(translations, created)
	}

	return translations, nil
}

func (s *service) ensureMenuDeletionAllowed(ctx context.Context, menuID uuid.UUID, force bool) error {
	if force || s.usageResolver == nil {
		return nil
	}
	bindings, err := s.usageResolver.ResolveMenuUsage(ctx, menuID)
	if err != nil {
		return err
	}
	if len(bindings) == 0 {
		return nil
	}
	return &MenuInUseError{MenuID: menuID, Bindings: bindings}
}

func (s *service) deleteMenuItemRecursive(ctx context.Context, item *MenuItem, deletedBy uuid.UUID, cascade bool) error {
	if item == nil {
		return ErrMenuItemNotFound
	}

	children, err := s.items.ListChildren(ctx, item.ID)
	if err != nil {
		return err
	}
	if len(children) > 0 {
		if !cascade {
			return ErrMenuItemHasChildren
		}
		for _, child := range children {
			if err := s.deleteMenuItemRecursive(ctx, child, deletedBy, true); err != nil {
				return err
			}
		}
	}

	if err := s.deleteMenuItemTranslations(ctx, item.ID); err != nil {
		return err
	}

	if err := s.items.Delete(ctx, item.ID); err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return ErrMenuItemNotFound
		}
		return err
	}

	return s.compactSiblingPositions(ctx, item.MenuID, item.ParentID, deletedBy)
}

func (s *service) deleteMenuItemTranslations(ctx context.Context, itemID uuid.UUID) error {
	translations, err := s.translations.ListByMenuItem(ctx, itemID)
	if err != nil {
		return err
	}
	for _, tr := range translations {
		if err := s.translations.Delete(ctx, tr.ID); err != nil {
			return err
		}
	}
	return nil
}

type menuItemSemantics struct {
	Type             string
	Target           map[string]any
	Icon             string
	Badge            map[string]any
	TranslationCount int
	Collapsible      bool
	Collapsed        bool
}

func normalizeMenuItemTypeValue(raw string) (string, error) {
	typ := strings.ToLower(strings.TrimSpace(raw))
	if typ == "" {
		return MenuItemTypeItem, nil
	}
	switch typ {
	case MenuItemTypeItem, MenuItemTypeGroup, MenuItemTypeSeparator:
		return typ, nil
	default:
		return "", ErrMenuItemTypeInvalid
	}
}

func normalizeMenuItemTypeValueOrDefault(raw string) string {
	typ, err := normalizeMenuItemTypeValue(raw)
	if err != nil || typ == "" {
		return MenuItemTypeItem
	}
	return typ
}

func (s *service) normalizeTargetForType(ctx context.Context, itemType string, raw map[string]any, envID uuid.UUID) (map[string]any, error) {
	switch itemType {
	case MenuItemTypeItem:
		return s.sanitizeTarget(ctx, raw, envID)
	case MenuItemTypeGroup:
		if len(raw) > 0 {
			return nil, ErrMenuItemGroupFields
		}
		return nil, nil
	case MenuItemTypeSeparator:
		if len(raw) > 0 {
			return nil, ErrMenuItemSeparatorFields
		}
		return nil, nil
	default:
		return nil, ErrMenuItemTypeInvalid
	}
}

func validateMenuItemSemantics(sem menuItemSemantics, hasChildren bool, allowCollapsibleWithoutChildren bool) error {
	switch sem.Type {
	case MenuItemTypeSeparator:
		if len(sem.Target) > 0 || sem.Icon != "" || len(sem.Badge) > 0 || sem.TranslationCount > 0 {
			return ErrMenuItemSeparatorFields
		}
		if sem.Collapsible || sem.Collapsed {
			return ErrMenuItemCollapsibleWithoutChildren
		}
	case MenuItemTypeGroup:
		if len(sem.Target) > 0 || sem.Icon != "" || len(sem.Badge) > 0 {
			return ErrMenuItemGroupFields
		}
	}

	if sem.Collapsed && !sem.Collapsible {
		return ErrMenuItemCollapsedWithoutCollapsible
	}
	if sem.Collapsible && !hasChildren && !allowCollapsibleWithoutChildren {
		return ErrMenuItemCollapsibleWithoutChildren
	}
	return nil
}

func cloneMapAny(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	return maps.Clone(input)
}

func cloneMapString(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	return maps.Clone(input)
}

func cloneStringSlice(input []string) []string {
	if input == nil {
		return nil
	}
	return slices.Clone(input)
}

func ensureMapAny(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	return maps.Clone(input)
}

func ensureNonNilTarget(target map[string]any) map[string]any {
	if target == nil {
		return map[string]any{}
	}
	return maps.Clone(target)
}

type normalizedMenuItemTranslation struct {
	Locale        string
	Label         string
	LabelKey      string
	GroupTitle    string
	GroupTitleKey string
	URLOverride   *string
}

func normalizeMenuItemTranslationInput(itemType string, input MenuItemTranslationInput) (normalizedMenuItemTranslation, error) {
	normalized := normalizedMenuItemTranslation{
		Locale:        strings.TrimSpace(input.Locale),
		Label:         strings.TrimSpace(input.Label),
		LabelKey:      strings.TrimSpace(input.LabelKey),
		GroupTitle:    strings.TrimSpace(input.GroupTitle),
		GroupTitleKey: strings.TrimSpace(input.GroupTitleKey),
		URLOverride:   trimURLPointer(input.URLOverride),
	}

	switch itemType {
	case MenuItemTypeSeparator:
		if normalized.Label != "" || normalized.LabelKey != "" || normalized.GroupTitle != "" || normalized.GroupTitleKey != "" {
			return normalizedMenuItemTranslation{}, ErrMenuItemSeparatorFields
		}
	case MenuItemTypeGroup:
		if normalized.Label == "" && normalized.LabelKey == "" && normalized.GroupTitle == "" && normalized.GroupTitleKey == "" {
			return normalizedMenuItemTranslation{}, ErrMenuItemTranslationTextRequired
		}
	default:
		if normalized.Label == "" && normalized.LabelKey == "" {
			return normalizedMenuItemTranslation{}, ErrMenuItemTranslationTextRequired
		}
	}

	if normalized.Label == "" {
		if itemType == MenuItemTypeGroup {
			if normalized.GroupTitle != "" {
				normalized.Label = normalized.GroupTitle
			} else if normalized.GroupTitleKey != "" {
				normalized.Label = normalized.GroupTitleKey
			}
		}
		if normalized.Label == "" && normalized.LabelKey != "" {
			normalized.Label = normalized.LabelKey
		}
	}

	return normalized, nil
}

func trimURLPointer(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *service) compactSiblingPositions(ctx context.Context, menuID uuid.UUID, parentID *uuid.UUID, actor uuid.UUID) error {
	siblings, err := s.fetchSiblings(ctx, menuID, parentID)
	if err != nil {
		return err
	}
	updated := make([]*MenuItem, 0, len(siblings))
	for idx, sibling := range siblings {
		if sibling.Position == idx {
			continue
		}
		sibling.Position = idx
		sibling.UpdatedAt = s.now()
		sibling.UpdatedBy = actor
		updated = append(updated, sibling)
	}
	if len(updated) == 0 {
		return nil
	}
	return s.items.BulkUpdateHierarchy(ctx, updated)
}

func (s *service) resolveParent(ctx context.Context, input AddMenuItemInput, allowMissing bool) (*uuid.UUID, *string, error) {
	if input.ParentID != nil && *input.ParentID != uuid.Nil {
		parentID := *input.ParentID
		return &parentID, nil, nil
	}
	ref := strings.TrimSpace(input.ParentCode)
	if ref == "" {
		return nil, nil, nil
	}
	if parsed, err := uuid.Parse(ref); err == nil && parsed != uuid.Nil {
		return &parsed, nil, nil
	}

	// Prefer external code lookup (human-friendly identifiers).
	if code := normalizeExternalCode(ref); code != "" {
		parent, err := s.items.GetByMenuAndExternalCode(ctx, input.MenuID, code)
		if err == nil && parent != nil {
			return &parent.ID, nil, nil
		}
		var notFound *NotFoundError
		if err != nil && !errors.As(err, &notFound) {
			return nil, nil, err
		}
	}

	// Fallback: allow referencing by canonical key.
	parent, err := s.items.GetByMenuAndCanonicalKey(ctx, input.MenuID, ref)
	if err == nil && parent != nil {
		return &parent.ID, nil, nil
	}
	var notFound *NotFoundError
	if err != nil && !errors.As(err, &notFound) {
		return nil, nil, err
	}

	if s.parentResolver != nil {
		id, err := s.parentResolver(ctx, ref, input)
		if err != nil {
			return nil, nil, err
		}
		if id != nil && *id != uuid.Nil {
			return id, nil, nil
		}
	}

	if allowMissing {
		return nil, &ref, nil
	}
	return nil, nil, ErrMenuItemParentInvalid
}

func (s *service) pickMenuItemID(input AddMenuItemInput) uuid.UUID {
	if input.ID != nil && *input.ID != uuid.Nil {
		return *input.ID
	}
	if s.id == nil {
		return uuid.New()
	}
	id := s.id(input)
	if id == uuid.Nil {
		return uuid.New()
	}
	return id
}

func (s *service) mergeTranslations(ctx context.Context, existing *MenuItem, itemType string, inputs []MenuItemTranslationInput) (*MenuItem, error) {
	if existing == nil {
		return nil, ErrMenuItemNotFound
	}

	existingTranslations, err := s.translations.ListByMenuItem(ctx, existing.ID)
	if err != nil {
		return nil, err
	}

	seen := make(map[uuid.UUID]struct{}, len(existingTranslations))
	for _, tr := range existingTranslations {
		if tr != nil {
			seen[tr.LocaleID] = struct{}{}
		}
	}

	var added []*MenuItemTranslation
	for _, tr := range inputs {
		normalized, err := normalizeMenuItemTranslationInput(itemType, tr)
		if err != nil {
			return nil, err
		}

		locale, err := s.lookupLocale(ctx, normalized.Locale)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[locale.ID]; exists {
			continue
		}

		now := s.now()
		record := &MenuItemTranslation{
			ID:            s.nextID(),
			MenuItemID:    existing.ID,
			LocaleID:      locale.ID,
			Locale:        locale,
			Label:         normalized.Label,
			LabelKey:      normalized.LabelKey,
			GroupTitle:    normalized.GroupTitle,
			GroupTitleKey: normalized.GroupTitleKey,
			URLOverride:   normalized.URLOverride,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		created, err := s.translations.Create(ctx, record)
		if err != nil {
			return nil, err
		}
		added = append(added, created)
		seen[locale.ID] = struct{}{}
	}

	if len(added) > 0 {
		existingTranslations = append(existingTranslations, added...)
	}
	existing.Translations = existingTranslations
	return existing, nil
}

func (s *service) findDuplicateByCanonicalKey(ctx context.Context, menuID uuid.UUID, key string) (*MenuItem, error) {
	items, err := s.items.ListByMenu(ctx, menuID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item == nil || item.CanonicalKey == nil {
			continue
		}
		if *item.CanonicalKey == key {
			return item, nil
		}
	}
	return nil, nil
}

func deriveCanonicalKey(itemType string, target map[string]any) *string {
	// legacy helper retained for backward compatibility; use deriveCanonicalKeyFromInput/deriveCanonicalKeyFromMenuItem instead.
	if itemType != MenuItemTypeItem || len(target) == 0 {
		return nil
	}
	t := strings.TrimSpace(fmt.Sprint(target["type"]))
	switch t {
	case "page":
		if raw, ok := target["page_id"]; ok {
			if val := strings.TrimSpace(fmt.Sprint(raw)); val != "" {
				key := "page:id:" + val
				return &key
			}
		}
		if raw, ok := target["slug"]; ok {
			if val := strings.TrimSpace(fmt.Sprint(raw)); val != "" {
				key := "page:slug:" + val
				return &key
			}
		}
	default:
		if raw, ok := target["url"]; ok {
			if val := strings.TrimSpace(fmt.Sprint(raw)); val != "" {
				key := "url:" + val
				return &key
			}
		}
		if raw, ok := target["path"]; ok {
			if val := strings.TrimSpace(fmt.Sprint(raw)); val != "" {
				key := "path:" + val
				return &key
			}
		}
	}
	return nil
}

func normalizeExternalCode(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func canonicalKeyForExternalCode(code string) *string {
	code = normalizeExternalCode(code)
	if code == "" {
		return nil
	}
	key := "code:" + code
	return &key
}

func normalizeParentRefPointer(ref *string) *string {
	if ref == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*ref)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func canonicalParentRef(parentID *uuid.UUID, parentRef *string, parentCode string) string {
	if parentRef != nil {
		if trimmed := strings.TrimSpace(*parentRef); trimmed != "" {
			return "ref:" + strings.ToLower(trimmed)
		}
	}
	if trimmed := strings.TrimSpace(parentCode); trimmed != "" {
		return "ref:" + strings.ToLower(trimmed)
	}
	return parentKey(parentID)
}

func deriveCanonicalKeyFromInput(itemType string, target map[string]any, input AddMenuItemInput, parentID *uuid.UUID, parentRef *string) *string {
	if itemType == MenuItemTypeItem {
		return deriveCanonicalKey(itemType, target)
	}

	parentKey := canonicalParentRef(parentID, parentRef, input.ParentCode)

	switch itemType {
	case MenuItemTypeGroup:
		groupKey := extractGroupKeyFromInputs(input.Translations)
		if groupKey != "" {
			key := "group:" + groupKey + ":" + parentKey
			return &key
		}
		key := "group:" + parentKey
		return &key
	case MenuItemTypeSeparator:
		key := fmt.Sprintf("separator:%s:%d", parentKey, input.Position)
		return &key
	default:
		return nil
	}
}

func deriveCanonicalKeyFromMenuItem(item *MenuItem) *string {
	if item == nil {
		return nil
	}
	if key := canonicalKeyForExternalCode(normalizeExternalCode(item.ExternalCode)); key != nil {
		return key
	}
	if normalizeMenuItemTypeValueOrDefault(item.Type) == MenuItemTypeItem {
		return deriveCanonicalKey(item.Type, item.Target)
	}

	parentKey := canonicalParentRef(item.ParentID, item.ParentRef, "")
	switch normalizeMenuItemTypeValueOrDefault(item.Type) {
	case MenuItemTypeGroup:
		groupKey := extractGroupKeyFromTranslations(item.Translations)
		if groupKey != "" {
			key := "group:" + groupKey + ":" + parentKey
			return &key
		}
		key := "group:" + parentKey
		return &key
	case MenuItemTypeSeparator:
		key := fmt.Sprintf("separator:%s:%d", parentKey, item.Position)
		return &key
	default:
		return nil
	}
}

func extractGroupKeyFromInputs(inputs []MenuItemTranslationInput) string {
	for _, tr := range inputs {
		if key := strings.TrimSpace(tr.GroupTitleKey); key != "" {
			return key
		}
		if key := strings.TrimSpace(tr.LabelKey); key != "" {
			return key
		}
		if title := strings.TrimSpace(tr.GroupTitle); title != "" {
			return title
		}
		if label := strings.TrimSpace(tr.Label); label != "" {
			return label
		}
	}
	return ""
}

func extractGroupKeyFromTranslations(translations []*MenuItemTranslation) string {
	for _, tr := range translations {
		if tr == nil {
			continue
		}
		if key := strings.TrimSpace(tr.GroupTitleKey); key != "" {
			return key
		}
		if key := strings.TrimSpace(tr.LabelKey); key != "" {
			return key
		}
		if title := strings.TrimSpace(tr.GroupTitle); title != "" {
			return title
		}
		if label := strings.TrimSpace(tr.Label); label != "" {
			return label
		}
	}
	return ""
}

func (s *service) lookupLocale(ctx context.Context, code string) (*content.Locale, error) {
	if strings.TrimSpace(code) == "" {
		return nil, ErrUnknownLocale
	}
	locale, err := s.locales.GetByCode(ctx, strings.TrimSpace(code))
	if err != nil {
		return nil, ErrUnknownLocale
	}
	return locale, nil
}

func (s *service) fetchSiblings(ctx context.Context, menuID uuid.UUID, parentID *uuid.UUID) ([]*MenuItem, error) {
	items, err := s.items.ListByMenu(ctx, menuID)
	if err != nil {
		return nil, err
	}
	out := make([]*MenuItem, 0, len(items))
	for _, item := range items {
		if uuidPtrEqual(item.ParentID, parentID) {
			out = append(out, item)
		}
	}
	slices.SortFunc(out, func(a, b *MenuItem) int {
		return a.Position - b.Position
	})
	return out, nil
}

func (s *service) shiftSiblings(ctx context.Context, siblings []*MenuItem, start int) error {
	for i := len(siblings) - 1; i >= start; i-- {
		sibling := siblings[i]
		sibling.Position++
		sibling.UpdatedAt = s.now()
		if _, err := s.items.Update(ctx, sibling); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) repositionItem(ctx context.Context, item *MenuItem, siblings []*MenuItem, desired int) error {
	currentIdx := -1
	for idx, sib := range siblings {
		if sib.ID == item.ID {
			currentIdx = idx
			break
		}
	}
	if currentIdx == -1 {
		// Item may have been moved or parent changed; just ensure positions consistent.
		siblings = append(siblings, item)
	}

	if desired == currentIdx {
		return nil
	}

	// Remove item and re-insert.
	var remaining []*MenuItem
	for _, sib := range siblings {
		if sib.ID != item.ID {
			remaining = append(remaining, sib)
		}
	}

	if desired > len(remaining) {
		desired = len(remaining)
	}

	updatedOrder := append([]*MenuItem{}, remaining[:desired]...)
	updatedOrder = append(updatedOrder, item)
	updatedOrder = append(updatedOrder, remaining[desired:]...)

	for idx, sib := range updatedOrder {
		if sib.Position != idx {
			sib.Position = idx
			sib.UpdatedAt = s.now()
			if _, err := s.items.Update(ctx, sib); err != nil {
				return err
			}
		}
	}

	return nil
}

func buildHierarchy(items []*MenuItem, translations map[uuid.UUID][]*MenuItemTranslation) []*MenuItem {
	byID := make(map[uuid.UUID]*MenuItem, len(items))
	children := make(map[string][]*MenuItem, len(items))

	for _, item := range items {
		clone := *item
		if item.Target != nil {
			clone.Target = maps.Clone(item.Target)
		}
		if item.Badge != nil {
			clone.Badge = maps.Clone(item.Badge)
		}
		if item.Metadata != nil {
			clone.Metadata = maps.Clone(item.Metadata)
		}
		if item.Styles != nil {
			clone.Styles = maps.Clone(item.Styles)
		}
		if len(item.Permissions) > 0 {
			clone.Permissions = cloneStringSlice(item.Permissions)
		}
		if len(item.Classes) > 0 {
			clone.Classes = cloneStringSlice(item.Classes)
		}
		if item.CanonicalKey != nil {
			key := *item.CanonicalKey
			clone.CanonicalKey = &key
		}
		clone.Children = nil
		clone.Translations = translations[item.ID]
		byID[item.ID] = &clone
		parentKey := parentKey(item.ParentID)
		children[parentKey] = append(children[parentKey], &clone)
	}

	for _, item := range byID {
		key := parentKey(&item.ID)
		if kids, ok := children[key]; ok {
			slices.SortFunc(kids, func(a, b *MenuItem) int {
				return a.Position - b.Position
			})
			item.Children = kids
		}
	}

	rootKey := parentKey(nil)
	root := children[rootKey]
	slices.SortFunc(root, func(a, b *MenuItem) int {
		return a.Position - b.Position
	})
	return root
}

func parentKey(id *uuid.UUID) string {
	if id == nil {
		return "root"
	}
	return id.String()
}

func hasCycle(parents map[uuid.UUID]*uuid.UUID) bool {
	visited := make(map[uuid.UUID]int, len(parents))

	var visit func(uuid.UUID) bool
	visit = func(id uuid.UUID) bool {
		state := visited[id]
		if state == 1 {
			return true
		}
		if state == 2 {
			return false
		}
		visited[id] = 1
		if parent := parents[id]; parent != nil {
			if visit(*parent) {
				return true
			}
		}
		visited[id] = 2
		return false
	}

	for id := range parents {
		if visit(id) {
			return true
		}
	}
	return false
}

func hierarchyDepthExceeded(parents map[uuid.UUID]*uuid.UUID, maxDepth int) bool {
	if maxDepth <= 0 {
		return false
	}
	memo := make(map[uuid.UUID]int, len(parents))
	state := make(map[uuid.UUID]int, len(parents))

	var depth func(uuid.UUID) int
	depth = func(id uuid.UUID) int {
		if d, ok := memo[id]; ok {
			return d
		}
		if state[id] == 1 {
			return maxDepth + 1
		}
		state[id] = 1
		d := 1
		if parent := parents[id]; parent != nil {
			d = depth(*parent) + 1
		}
		state[id] = 2
		memo[id] = d
		return d
	}

	for id := range parents {
		if depth(id) > maxDepth {
			return true
		}
	}
	return false
}

func resolveMenuItemRef(ref string, byID map[uuid.UUID]*MenuItem, byCanonical map[string]*MenuItem, byExternal map[string]*MenuItem) *MenuItem {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return nil
	}
	if parsed, err := uuid.Parse(trimmed); err == nil && parsed != uuid.Nil {
		return byID[parsed]
	}
	if item := byExternal[normalizeExternalCode(trimmed)]; item != nil {
		return item
	}
	return byCanonical[trimmed]
}

func countPendingParentRefs(items []*MenuItem) int {
	count := 0
	for _, item := range items {
		if item == nil || item.ParentID != nil || item.ParentRef == nil {
			continue
		}
		if strings.TrimSpace(*item.ParentRef) == "" {
			continue
		}
		count++
	}
	return count
}

func normalizeMenuItemPositionsForParents(items []*MenuItem, parentKeys map[string]struct{}, touched map[uuid.UUID]struct{}) []*MenuItem {
	byParent := make(map[string][]*MenuItem)
	for _, item := range items {
		if item == nil {
			continue
		}
		key := parentKey(item.ParentID)
		byParent[key] = append(byParent[key], item)
	}

	dirty := make([]*MenuItem, 0, len(touched))
	dirtyIDs := make(map[uuid.UUID]struct{}, len(touched))

	for key := range parentKeys {
		siblings := byParent[key]
		if len(siblings) == 0 {
			continue
		}
		slices.SortFunc(siblings, func(a, b *MenuItem) int {
			if a.Position == b.Position {
				return a.CreatedAt.Compare(b.CreatedAt)
			}
			return a.Position - b.Position
		})
		for idx, item := range siblings {
			if item.Position != idx {
				item.Position = idx
				if _, ok := dirtyIDs[item.ID]; !ok {
					dirty = append(dirty, item)
					dirtyIDs[item.ID] = struct{}{}
				}
			}
		}
	}

	for id := range touched {
		if _, ok := dirtyIDs[id]; ok {
			continue
		}
		if item := findMenuItem(items, id); item != nil {
			dirty = append(dirty, item)
			dirtyIDs[id] = struct{}{}
		}
	}

	return dirty
}

func findMenuItem(items []*MenuItem, id uuid.UUID) *MenuItem {
	for _, item := range items {
		if item != nil && item.ID == id {
			return item
		}
	}
	return nil
}

func (s *service) sanitizeTarget(ctx context.Context, raw map[string]any, envID uuid.UUID) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, ErrMenuItemTargetMissing
	}

	normalized := maps.Clone(raw)
	typVal, ok := normalized["type"]
	if !ok {
		return nil, ErrMenuItemTargetMissing
	}

	typ := strings.ToLower(strings.TrimSpace(fmt.Sprint(typVal)))
	if typ == "" {
		return nil, ErrMenuItemTargetMissing
	}
	normalized["type"] = typ

	switch typ {
	case "page":
		return s.sanitizePageTarget(ctx, normalized, envID)
	case "content":
		return s.sanitizeContentTarget(normalized), nil
	case "route":
		return s.sanitizeRouteTarget(normalized), nil
	case "module":
		return s.sanitizeModuleTarget(normalized), nil
	case "external":
		return s.sanitizeExternalTarget(normalized), nil
	default:
		if urlVal, ok := normalized["url"]; ok {
			if url := strings.TrimSpace(fmt.Sprint(urlVal)); url != "" {
				normalized["url"] = url
			} else {
				delete(normalized, "url")
			}
		}
		return normalized, nil
	}
}

func (s *service) sanitizeContentTarget(target map[string]any) map[string]any {
	cloned := maps.Clone(target)
	if raw, ok := cloned["slug"]; ok {
		slug := strings.TrimSpace(fmt.Sprint(raw))
		if slug != "" {
			cloned["slug"] = slug
		} else {
			delete(cloned, "slug")
		}
	}
	if raw, ok := cloned["path"]; ok {
		path := strings.TrimSpace(fmt.Sprint(raw))
		if path != "" {
			cloned["path"] = ensureLeadingSlash(path)
		} else {
			delete(cloned, "path")
		}
	}
	if raw, ok := cloned["url"]; ok {
		url := strings.TrimSpace(fmt.Sprint(raw))
		if url != "" {
			cloned["url"] = url
		} else {
			delete(cloned, "url")
		}
	}
	return cloned
}

func (s *service) sanitizeRouteTarget(target map[string]any) map[string]any {
	cloned := maps.Clone(target)
	if raw, ok := cloned["path"]; ok {
		path := strings.TrimSpace(fmt.Sprint(raw))
		if path != "" {
			cloned["path"] = ensureLeadingSlash(path)
		} else {
			delete(cloned, "path")
		}
	}
	if raw, ok := cloned["route"]; ok {
		route := strings.TrimSpace(fmt.Sprint(raw))
		if route != "" {
			cloned["route"] = route
		} else {
			delete(cloned, "route")
		}
	}
	return cloned
}

func (s *service) sanitizeModuleTarget(target map[string]any) map[string]any {
	cloned := maps.Clone(target)
	if raw, ok := cloned["path"]; ok {
		path := strings.TrimSpace(fmt.Sprint(raw))
		if path != "" {
			cloned["path"] = ensureLeadingSlash(path)
		} else {
			delete(cloned, "path")
		}
	}
	if raw, ok := cloned["module"]; ok {
		module := strings.TrimSpace(fmt.Sprint(raw))
		if module != "" {
			cloned["module"] = module
		} else {
			delete(cloned, "module")
		}
	}
	return cloned
}

func (s *service) sanitizeExternalTarget(target map[string]any) map[string]any {
	cloned := maps.Clone(target)
	if raw, ok := cloned["url"]; ok {
		url := strings.TrimSpace(fmt.Sprint(raw))
		if url != "" {
			cloned["url"] = url
		} else {
			delete(cloned, "url")
		}
	}
	return cloned
}

func (s *service) sanitizePageTarget(ctx context.Context, target map[string]any, envID uuid.UUID) (map[string]any, error) {
	cloned := maps.Clone(target)

	slug, _ := extractSlug(cloned)
	if slug != "" {
		cloned["slug"] = slug
	} else {
		delete(cloned, "slug")
	}

	var (
		pageID uuid.UUID
		hasID  bool
	)
	if rawID, ok := cloned["page_id"]; ok {
		id, okID, err := parseUUIDValue(rawID)
		if err != nil {
			return nil, err
		}
		if okID {
			pageID = id
			hasID = true
		} else {
			delete(cloned, "page_id")
		}
	} else {
		delete(cloned, "page_id")
	}

	if slug == "" && !hasID {
		return nil, ErrMenuItemPageSlugRequired
	}

	if s.pageRepo != nil {
		var (
			page *pages.Page
			err  error
		)
		if hasID {
			page, err = s.pageRepo.GetByID(ctx, pageID)
		} else {
			page, err = s.pageRepo.GetBySlug(ctx, slug, envID.String())
		}
		if err != nil {
			var notFound *pages.PageNotFoundError
			if errors.As(err, &notFound) {
				return nil, ErrMenuItemPageNotFound
			}
			return nil, err
		}
		if page != nil {
			pageID = page.ID
			if slug == "" {
				slug = strings.TrimSpace(page.Slug)
				cloned["slug"] = slug
			}
		}
	}

	if pageID != uuid.Nil {
		cloned["page_id"] = pageID.String()
	}
	if slug != "" {
		cloned["slug"] = slug
	}

	return cloned, nil
}

func (s *service) resolveURLForTarget(ctx context.Context, target map[string]any, envID uuid.UUID, localeID uuid.UUID) (string, error) {
	if len(target) == 0 {
		return "", nil
	}
	typVal, ok := target["type"]
	if !ok {
		return "", nil
	}
	typ := strings.ToLower(strings.TrimSpace(fmt.Sprint(typVal)))
	switch typ {
	case "page":
		return s.resolvePageURL(ctx, target, envID, localeID)
	case "content":
		if raw, ok := target["path"]; ok {
			if path := strings.TrimSpace(fmt.Sprint(raw)); path != "" {
				return ensureLeadingSlash(path), nil
			}
		}
		if slug, ok := extractSlug(target); ok && slug != "" {
			return ensureLeadingSlash(slug), nil
		}
		if raw, ok := target["url"]; ok {
			return strings.TrimSpace(fmt.Sprint(raw)), nil
		}
	case "route", "module":
		if raw, ok := target["path"]; ok {
			if path := strings.TrimSpace(fmt.Sprint(raw)); path != "" {
				return ensureLeadingSlash(path), nil
			}
		}
		if raw, ok := target["url"]; ok {
			return strings.TrimSpace(fmt.Sprint(raw)), nil
		}
	case "external":
		if raw, ok := target["url"]; ok {
			return strings.TrimSpace(fmt.Sprint(raw)), nil
		}
	default:
		if raw, ok := target["url"]; ok {
			return strings.TrimSpace(fmt.Sprint(raw)), nil
		}
	}
	return "", nil
}

func (s *service) resolvePageURL(ctx context.Context, target map[string]any, envID uuid.UUID, localeID uuid.UUID) (string, error) {
	slug, _ := extractSlug(target)

	var pageID uuid.UUID
	if raw, ok := target["page_id"]; ok {
		if id, hasID, err := parseUUIDValue(raw); err == nil && hasID {
			pageID = id
		}
	}

	if s.pageRepo == nil {
		if slug != "" {
			return ensureLeadingSlash(slug), nil
		}
		return "", nil
	}

	var (
		page *pages.Page
		err  error
	)
	if pageID != uuid.Nil {
		page, err = s.pageRepo.GetByID(ctx, pageID)
	} else if slug != "" {
		page, err = s.pageRepo.GetBySlug(ctx, slug, envID.String())
	} else {
		return "", ErrMenuItemPageSlugRequired
	}
	if err != nil {
		var notFound *pages.PageNotFoundError
		if errors.As(err, &notFound) {
			return "", ErrMenuItemPageNotFound
		}
		return "", err
	}
	if page == nil {
		return "", ErrMenuItemPageNotFound
	}

	if slug == "" {
		slug = strings.TrimSpace(page.Slug)
	}

	path := findPagePath(page, localeID)
	if path == "" {
		if slug != "" {
			return ensureLeadingSlash(slug), nil
		}
		return "", nil
	}

	return ensureLeadingSlash(path), nil
}

func findPagePath(page *pages.Page, localeID uuid.UUID) string {
	if page == nil {
		return ""
	}
	if localeID != uuid.Nil {
		for _, tr := range page.Translations {
			if tr.LocaleID == localeID && tr.Path != "" {
				return tr.Path
			}
		}
	}
	for _, tr := range page.Translations {
		if tr.Path != "" {
			return tr.Path
		}
	}
	return ""
}

func selectMenuTranslation(translations []*MenuItemTranslation, localeID uuid.UUID) (*MenuItemTranslation, *MenuItemTranslation) {
	var fallback *MenuItemTranslation
	for _, tr := range translations {
		if fallback == nil {
			fallback = tr
		}
		if localeID != uuid.Nil && tr.LocaleID == localeID {
			return tr, fallback
		}
	}
	return nil, fallback
}

func extractSlug(target map[string]any) (string, bool) {
	if target == nil {
		return "", false
	}
	raw, ok := target["slug"]
	if !ok {
		return "", false
	}
	return strings.TrimSpace(fmt.Sprint(raw)), true
}

func normalizeNavigationNodes(nodes []NavigationNode) []NavigationNode {
	if len(nodes) == 0 {
		return nil
	}
	candidates := make([]NavigationNode, 0, len(nodes))

	for _, node := range nodes {
		if node.Type == "" {
			node.Type = MenuItemTypeItem
		}

		node.Children = normalizeNavigationNodes(node.Children)
		if node.Type == MenuItemTypeGroup && len(node.Children) == 0 && isEffectivelyEmptyGroupNode(node) {
			continue
		}

		if node.Type == MenuItemTypeSeparator {
			node.Collapsible = false
			node.Collapsed = false
		} else if len(node.Children) == 0 {
			node.Collapsible = false
			node.Collapsed = false
		} else if !node.Collapsible {
			node.Collapsed = false
		}

		candidates = append(candidates, node)
	}

	slices.SortStableFunc(candidates, func(a, b NavigationNode) int {
		switch {
		case a.Position < b.Position:
			return -1
		case a.Position > b.Position:
			return 1
		}
		return bytes.Compare(a.ID[:], b.ID[:])
	})

	normalized := make([]NavigationNode, 0, len(candidates))
	prevSeparator := false

	for _, node := range candidates {
		if node.Type == MenuItemTypeSeparator {
			if prevSeparator || len(normalized) == 0 {
				continue
			}
			prevSeparator = true
			normalized = append(normalized, node)
			continue
		}

		prevSeparator = false
		normalized = append(normalized, node)
	}

	for len(normalized) > 0 && normalized[len(normalized)-1].Type == MenuItemTypeSeparator {
		normalized = normalized[:len(normalized)-1]
	}
	return normalized
}

func isEffectivelyEmptyGroupNode(node NavigationNode) bool {
	if node.Type != MenuItemTypeGroup {
		return false
	}
	if strings.TrimSpace(node.GroupTitle) != "" ||
		strings.TrimSpace(node.GroupTitleKey) != "" {
		return false
	}
	if strings.TrimSpace(node.URL) != "" {
		return false
	}
	if len(node.Target) > 0 {
		return false
	}
	if strings.TrimSpace(node.Icon) != "" {
		return false
	}
	if len(node.Badge) > 0 {
		return false
	}
	if len(node.Permissions) > 0 || len(node.Classes) > 0 {
		return false
	}
	if len(node.Styles) > 0 {
		return false
	}
	if len(node.Metadata) > 0 {
		return false
	}
	return true
}

type contentContributionCandidate struct {
	Node            NavigationNode
	Membership      ContentMenuMembership
	MergeMode       string
	DuplicatePolicy string
	SortOrder       *int
	PublishedAt     time.Time
}

func (s *service) includeContributions(opts MenuQueryOptions) bool {
	if opts.IncludeContributions == nil {
		return true
	}
	return *opts.IncludeContributions
}

func (s *service) resolveContentContributions(ctx context.Context, location string, locale string, opts MenuQueryOptions, envID uuid.UUID) ([]contentContributionCandidate, []ContentMenuMembership, string, error) {
	if s.contentRepo == nil || s.contentTypeRepo == nil {
		return nil, nil, MenuContributionDuplicateByURL, nil
	}
	contentTypes, err := s.contentTypeRepo.List(ctx, envID.String())
	if err != nil {
		return nil, nil, "", err
	}
	if len(contentTypes) == 0 {
		return nil, nil, MenuContributionDuplicateByURL, nil
	}

	typeByID := make(map[uuid.UUID]*content.ContentType, len(contentTypes))
	navByType := make(map[uuid.UUID]content.NavigationConfig, len(contentTypes))
	for _, contentType := range contentTypes {
		if contentType == nil {
			continue
		}
		cfg := content.ReadNavigationConfig(contentType)
		if !cfg.Enabled || !locationEligible(cfg.EligibleLocations, location) {
			continue
		}
		typeByID[contentType.ID] = contentType
		navByType[contentType.ID] = cfg
	}
	if len(typeByID) == 0 {
		return nil, nil, MenuContributionDuplicateByURL, nil
	}

	records, err := s.contentRepo.List(ctx, content.ContentListOption(envID.String()), content.WithTranslations())
	if err != nil {
		return nil, nil, "", err
	}

	candidates := make([]contentContributionCandidate, 0)
	for _, record := range records {
		if record == nil || !s.contentStatusAllowed(record, opts) {
			continue
		}
		contentType, ok := typeByID[record.ContentTypeID]
		if !ok {
			continue
		}
		cfg := navByType[record.ContentTypeID]
		visibility := content.ResolveNavigationVisibility(contentType, record.Metadata)
		visible, origin, state := locationVisibilityFor(visibility, location)
		if !visible {
			continue
		}

		translation := selectContentTranslation(record, locale)
		label := resolveContributionLabel(record, translation, cfg.LabelField)
		url := resolveContributionURL(record, translation, cfg.URLField)
		if url == "" {
			url = ensureLeadingSlash(strings.TrimSpace(record.Slug))
		}
		if label == "" {
			label = strings.TrimSpace(record.Slug)
		}
		if label == "" {
			label = record.ID.String()
		}

		sortOrder, hasSortOrder := extractContentSortOrder(record.Metadata)
		var sortOrderPtr *int
		if hasSortOrder {
			value := sortOrder
			sortOrderPtr = &value
		}

		target := map[string]any{
			"type":              "content",
			"content_id":        record.ID.String(),
			"content_type_id":   record.ContentTypeID.String(),
			"content_type_slug": strings.TrimSpace(contentType.Slug),
			"slug":              strings.TrimSpace(record.Slug),
			"url":               url,
		}
		metadata := map[string]any{
			"contribution":        "content",
			"contribution_origin": origin,
			"visibility_state":    state,
			"content_id":          record.ID.String(),
			"content_type_id":     record.ContentTypeID.String(),
			"content_type_slug":   strings.TrimSpace(contentType.Slug),
			"content_slug":        strings.TrimSpace(record.Slug),
		}
		node := NavigationNode{
			ID:                 record.ID,
			Type:               MenuItemTypeItem,
			Label:              label,
			URL:                url,
			Target:             target,
			Metadata:           metadata,
			Contribution:       true,
			ContributionOrigin: origin,
		}
		membership := ContentMenuMembership{
			ContentID:       record.ID,
			ContentTypeID:   record.ContentTypeID,
			ContentTypeSlug: strings.TrimSpace(contentType.Slug),
			ContentSlug:     strings.TrimSpace(record.Slug),
			Location:        strings.TrimSpace(location),
			Visible:         true,
			Origin:          origin,
			VisibilityState: state,
			MergeMode:       cfg.MergeMode,
			DuplicatePolicy: cfg.DuplicatePolicy,
			URL:             url,
			Label:           label,
			SortOrder:       sortOrderPtr,
		}
		publishedAt := time.Time{}
		if record.PublishedAt != nil {
			publishedAt = *record.PublishedAt
		}
		candidates = append(candidates, contentContributionCandidate{
			Node:            node,
			Membership:      membership,
			MergeMode:       cfg.MergeMode,
			DuplicatePolicy: cfg.DuplicatePolicy,
			SortOrder:       sortOrderPtr,
			PublishedAt:     publishedAt,
		})
	}

	if len(candidates) == 0 {
		return nil, nil, MenuContributionDuplicateByURL, nil
	}
	slices.SortStableFunc(candidates, func(a, b contentContributionCandidate) int {
		if a.SortOrder != nil && b.SortOrder == nil {
			return -1
		}
		if a.SortOrder == nil && b.SortOrder != nil {
			return 1
		}
		if a.SortOrder != nil && b.SortOrder != nil && *a.SortOrder != *b.SortOrder {
			return *a.SortOrder - *b.SortOrder
		}
		if !a.PublishedAt.Equal(b.PublishedAt) {
			if a.PublishedAt.After(b.PublishedAt) {
				return -1
			}
			return 1
		}
		if cmp := strings.Compare(strings.ToLower(strings.TrimSpace(a.Node.Label)), strings.ToLower(strings.TrimSpace(b.Node.Label))); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(strings.ToLower(strings.TrimSpace(a.Membership.ContentSlug)), strings.ToLower(strings.TrimSpace(b.Membership.ContentSlug))); cmp != 0 {
			return cmp
		}
		return bytes.Compare(a.Node.ID[:], b.Node.ID[:])
	})
	for idx := range candidates {
		candidates[idx].Node.Position = idx
	}

	memberships := make([]ContentMenuMembership, 0, len(candidates))
	for _, candidate := range candidates {
		memberships = append(memberships, candidate.Membership)
	}

	policy := MenuContributionDuplicateByURL
	if strings.TrimSpace(opts.ContributionDuplicatePolicy) != "" {
		policy = normalizeContributionDuplicatePolicy(opts.ContributionDuplicatePolicy)
	} else if len(candidates) > 0 {
		policy = normalizeContributionDuplicatePolicy(candidates[0].DuplicatePolicy)
		for i := 1; i < len(candidates); i++ {
			if normalizeContributionDuplicatePolicy(candidates[i].DuplicatePolicy) != policy {
				policy = MenuContributionDuplicateByURL
				break
			}
		}
	}
	return candidates, memberships, policy, nil
}

func (s *service) mergeContributionNodes(manual []NavigationNode, contributions []contentContributionCandidate, opts MenuQueryOptions, duplicatePolicy string) []NavigationNode {
	if len(contributions) == 0 {
		return reindexNavigationRootPositions(manual)
	}
	merged := append([]NavigationNode{}, manual...)

	overrideMergeMode := ""
	if strings.TrimSpace(opts.ContributionMergeMode) != "" {
		overrideMergeMode = normalizeContributionMergeMode(opts.ContributionMergeMode)
	}

	replaceStarted := false
	for _, candidate := range contributions {
		mode := candidate.MergeMode
		if overrideMergeMode != "" {
			mode = overrideMergeMode
		}
		switch normalizeContributionMergeMode(mode) {
		case content.NavigationMergePrepend:
			merged = append([]NavigationNode{candidate.Node}, merged...)
		case content.NavigationMergeReplace:
			if !replaceStarted {
				merged = nil
				replaceStarted = true
			}
			merged = append(merged, candidate.Node)
		default:
			merged = append(merged, candidate.Node)
		}
	}

	policy := normalizeContributionDuplicatePolicy(duplicatePolicy)
	if strings.TrimSpace(opts.ContributionDuplicatePolicy) != "" {
		policy = normalizeContributionDuplicatePolicy(opts.ContributionDuplicatePolicy)
	}
	if policy != content.NavigationDuplicateNone {
		merged = dedupeNavigationNodesByPolicy(merged, policy)
	}
	return reindexNavigationRootPositions(merged)
}

func dedupeNavigationNodesByPolicy(nodes []NavigationNode, policy string) []NavigationNode {
	if len(nodes) == 0 {
		return nil
	}
	normalizedPolicy := normalizeContributionDuplicatePolicy(policy)
	if normalizedPolicy == content.NavigationDuplicateNone {
		return append([]NavigationNode{}, nodes...)
	}
	seen := map[string]struct{}{}
	out := make([]NavigationNode, 0, len(nodes))
	for _, node := range nodes {
		key := contributionDuplicateKey(node, normalizedPolicy)
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		out = append(out, node)
	}
	return out
}

func contributionDuplicateKey(node NavigationNode, policy string) string {
	switch normalizeContributionDuplicatePolicy(policy) {
	case content.NavigationDuplicateByTarget:
		targetKey := canonicalTargetKey(node.Target)
		if targetKey == "" {
			return ""
		}
		return "target:" + targetKey
	case content.NavigationDuplicateByURL:
		fallthrough
	default:
		url := strings.ToLower(strings.TrimSpace(node.URL))
		if url == "" {
			return ""
		}
		return "url:" + url
	}
}

func canonicalTargetKey(target map[string]any) string {
	if len(target) == 0 {
		return ""
	}
	raw, err := json.Marshal(target)
	if err != nil {
		return ""
	}
	return string(raw)
}

func locationEligible(eligible []string, location string) bool {
	needle := strings.TrimSpace(location)
	if needle == "" {
		return false
	}
	for _, value := range eligible {
		if strings.EqualFold(strings.TrimSpace(value), needle) {
			return true
		}
	}
	return false
}

func locationVisibilityFor(result content.NavigationVisibilityResult, location string) (bool, string, string) {
	needle := strings.TrimSpace(location)
	if needle == "" {
		return false, "", content.NavigationStateInherit
	}
	for key, visible := range result.EffectiveVisibility {
		if !strings.EqualFold(strings.TrimSpace(key), needle) {
			continue
		}
		origin := result.Origins[key]
		if strings.TrimSpace(origin) == "" {
			origin = content.NavigationOriginDefault
		}
		state := result.EffectiveState[key]
		if strings.TrimSpace(state) == "" {
			state = content.NavigationStateInherit
		}
		return visible, origin, state
	}
	return false, content.NavigationOriginDefault, content.NavigationStateInherit
}

func normalizeContributionDuplicatePolicy(policy string) string {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case content.NavigationDuplicateByTarget:
		return content.NavigationDuplicateByTarget
	case content.NavigationDuplicateNone:
		return content.NavigationDuplicateNone
	case content.NavigationDuplicateByURL:
		fallthrough
	default:
		return content.NavigationDuplicateByURL
	}
}

func normalizeContributionMergeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case content.NavigationMergePrepend:
		return content.NavigationMergePrepend
	case content.NavigationMergeReplace:
		return content.NavigationMergeReplace
	case content.NavigationMergeAppend:
		fallthrough
	default:
		return content.NavigationMergeAppend
	}
}

func resolveContributionLabel(record *content.Content, translation *content.ContentTranslation, field string) string {
	key := strings.TrimSpace(field)
	if key == "" {
		key = "title"
	}
	switch strings.ToLower(key) {
	case "title":
		if translation != nil && strings.TrimSpace(translation.Title) != "" {
			return strings.TrimSpace(translation.Title)
		}
	case "slug":
		return strings.TrimSpace(record.Slug)
	}
	if translation != nil {
		if value, ok := contributionStringValue(translation.Content, key); ok {
			return value
		}
	}
	if value, ok := contributionStringValue(record.Metadata, key); ok {
		return value
	}
	if translation != nil && strings.TrimSpace(translation.Title) != "" {
		return strings.TrimSpace(translation.Title)
	}
	return ""
}

func resolveContributionURL(record *content.Content, translation *content.ContentTranslation, field string) string {
	key := strings.TrimSpace(field)
	if key == "" {
		key = "path"
	}
	switch strings.ToLower(key) {
	case "slug":
		return ensureLeadingSlash(strings.TrimSpace(record.Slug))
	case "path":
		if value, ok := contributionStringValue(record.Metadata, key); ok {
			return ensureLeadingSlash(value)
		}
		if translation != nil {
			if value, ok := contributionStringValue(translation.Content, key); ok {
				return ensureLeadingSlash(value)
			}
		}
	default:
		if translation != nil {
			if value, ok := contributionStringValue(translation.Content, key); ok {
				return ensureLeadingSlash(value)
			}
		}
		if value, ok := contributionStringValue(record.Metadata, key); ok {
			return ensureLeadingSlash(value)
		}
	}
	return ""
}

func contributionStringValue(payload map[string]any, key string) (string, bool) {
	if len(payload) == 0 {
		return "", false
	}
	raw, ok := payload[key]
	if !ok {
		return "", false
	}
	value := strings.TrimSpace(fmt.Sprint(raw))
	if value == "" {
		return "", false
	}
	return value, true
}

func extractContentSortOrder(metadata map[string]any) (int, bool) {
	if len(metadata) == 0 {
		return 0, false
	}
	if value, ok := metadata["sort_order"]; ok {
		if parsed, ok := parseIntValue(value); ok {
			return parsed, true
		}
	}
	if value, ok := metadata["order"]; ok {
		if parsed, ok := parseIntValue(value); ok {
			return parsed, true
		}
	}
	return 0, false
}

func parseIntValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case uint:
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		return int(typed), true
	case float32:
		if float32(int(typed)) != typed {
			return 0, false
		}
		return int(typed), true
	case float64:
		if float64(int(typed)) != typed {
			return 0, false
		}
		return int(typed), true
	default:
		return 0, false
	}
}

func selectContentTranslation(record *content.Content, locale string) *content.ContentTranslation {
	if record == nil || len(record.Translations) == 0 {
		return nil
	}
	requested := strings.ToLower(strings.TrimSpace(locale))
	if requested != "" {
		for _, translation := range record.Translations {
			if translation == nil || translation.Locale == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(translation.Locale.Code), requested) {
				return translation
			}
		}
	}
	primary := strings.ToLower(strings.TrimSpace(record.PrimaryLocale))
	if primary != "" {
		for _, translation := range record.Translations {
			if translation == nil || translation.Locale == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(translation.Locale.Code), primary) {
				return translation
			}
		}
	}
	for _, translation := range record.Translations {
		if translation != nil {
			return translation
		}
	}
	return nil
}

func (s *service) contentStatusAllowed(record *content.Content, opts MenuQueryOptions) bool {
	status := strings.ToLower(strings.TrimSpace(contentEffectiveStatus(record, s.now)))
	requested := strings.ToLower(strings.TrimSpace(opts.Status))
	if requested != "" {
		return status == requested
	}
	if opts.IncludeDrafts {
		return status == MenuStatusPublished || status == MenuStatusDraft
	}
	return status == MenuStatusPublished
}

func contentEffectiveStatus(record *content.Content, now func() time.Time) string {
	if record == nil {
		return MenuStatusDraft
	}
	current := strings.ToLower(strings.TrimSpace(record.Status))
	if current == "" {
		current = MenuStatusDraft
	}
	currentTime := time.Now()
	if now != nil {
		currentTime = now()
	}
	if record.UnpublishAt != nil && !record.UnpublishAt.After(currentTime) {
		return "archived"
	}
	if record.PublishAt != nil {
		if record.PublishAt.After(currentTime) {
			return "scheduled"
		}
		return MenuStatusPublished
	}
	if record.PublishedAt != nil && !record.PublishedAt.After(currentTime) {
		return MenuStatusPublished
	}
	return current
}

func reindexNavigationRootPositions(nodes []NavigationNode) []NavigationNode {
	if len(nodes) == 0 {
		return nil
	}
	out := append([]NavigationNode{}, nodes...)
	for idx := range out {
		out[idx].Position = idx
	}
	return out
}

func normalizeMenuStatus(status string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(status))
	if trimmed == "" {
		return MenuStatusPublished, nil
	}
	switch trimmed {
	case MenuStatusDraft, MenuStatusPublished:
		return trimmed, nil
	default:
		return "", ErrMenuStatusInvalid
	}
}

func normalizeMenuViewMode(mode string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(mode))
	if trimmed == "" {
		return MenuViewModeFull, nil
	}
	switch trimmed {
	case MenuViewModeFull, MenuViewModeTopLevelLimit, MenuViewModeMaxDepth, MenuViewModeIncludeIDs, MenuViewModeExcludeIDs, MenuViewModeComposed:
		return trimmed, nil
	default:
		return "", ErrMenuViewModeInvalid
	}
}

func normalizeBindingPolicy(policy string) string {
	trimmed := strings.ToLower(strings.TrimSpace(policy))
	switch trimmed {
	case MenuBindingPolicyPriorityMulti:
		return MenuBindingPolicyPriorityMulti
	case MenuBindingPolicySingle, "":
		return MenuBindingPolicySingle
	default:
		return MenuBindingPolicySingle
	}
}

func (s *service) effectiveBindingPolicy(opts MenuQueryOptions) string {
	if s == nil {
		return MenuBindingPolicySingle
	}
	if normalized := normalizeBindingPolicy(opts.BindingPolicy); normalized != MenuBindingPolicySingle || strings.TrimSpace(opts.BindingPolicy) != "" {
		return normalized
	}
	return normalizeBindingPolicy(s.duplicateBindingPolicy)
}

func (s *service) statusAllowed(status string, opts MenuQueryOptions) bool {
	normalized, err := normalizeMenuStatus(status)
	if err != nil {
		normalized = MenuStatusPublished
	}
	if strings.TrimSpace(opts.Status) != "" {
		requested, reqErr := normalizeMenuStatus(opts.Status)
		if reqErr != nil {
			return false
		}
		return normalized == requested
	}
	if opts.IncludeDrafts {
		return normalized == MenuStatusPublished || normalized == MenuStatusDraft
	}
	return normalized == MenuStatusPublished
}

func normalizeLocalePointer(locale *string) *string {
	if locale == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*locale)
	if trimmed == "" {
		return nil
	}
	normalized := strings.ToLower(trimmed)
	return &normalized
}

func normalizeCodePointer(code *string) *string {
	if code == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*code)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func cloneUUIDPointer(id *uuid.UUID) *uuid.UUID {
	if id == nil {
		return nil
	}
	copy := *id
	return &copy
}

func normalizePositiveIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	if *value <= 0 {
		return nil
	}
	copy := *value
	return &copy
}

func normalizeCodeList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func localePointerEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(*a), strings.TrimSpace(*b))
}

func stringPointerEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return strings.TrimSpace(*a) == strings.TrimSpace(*b)
}

func (s *service) resolveLocationBindings(ctx context.Context, location, locale string, opts MenuQueryOptions, env ...string) ([]*MenuLocationBinding, error) {
	if s.bindings == nil {
		return nil, nil
	}
	location = strings.TrimSpace(location)
	if location == "" {
		return nil, nil
	}
	envID, _, err := s.resolveEnvironment(ctx, pickEnvironmentKey(env...))
	if err != nil {
		return nil, err
	}
	records, err := s.bindings.ListByLocation(ctx, location, envID.String())
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	requestedLocale := strings.ToLower(strings.TrimSpace(locale))
	filtered := make([]*MenuLocationBinding, 0, len(records))
	for _, binding := range records {
		if binding == nil {
			continue
		}
		if !s.statusAllowed(binding.Status, opts) {
			continue
		}
		if requestedLocale != "" && binding.Locale != nil {
			if normalized := strings.ToLower(strings.TrimSpace(*binding.Locale)); normalized != "" && normalized != requestedLocale {
				continue
			}
		}
		filtered = append(filtered, binding)
	}
	slices.SortStableFunc(filtered, func(a, b *MenuLocationBinding) int {
		aLocaleRank := bindingLocaleRank(a, requestedLocale)
		bLocaleRank := bindingLocaleRank(b, requestedLocale)
		if aLocaleRank != bLocaleRank {
			return aLocaleRank - bLocaleRank
		}
		aStatusRank := bindingStatusRank(a, opts)
		bStatusRank := bindingStatusRank(b, opts)
		if aStatusRank != bStatusRank {
			return aStatusRank - bStatusRank
		}
		if a.Priority != b.Priority {
			return b.Priority - a.Priority
		}
		if !a.UpdatedAt.Equal(b.UpdatedAt) {
			if a.UpdatedAt.After(b.UpdatedAt) {
				return -1
			}
			return 1
		}
		return bytes.Compare(a.ID[:], b.ID[:])
	})
	return filtered, nil
}

func bindingLocaleRank(binding *MenuLocationBinding, locale string) int {
	if binding == nil {
		return 3
	}
	if strings.TrimSpace(locale) == "" {
		return 0
	}
	if binding.Locale == nil || strings.TrimSpace(*binding.Locale) == "" {
		return 1
	}
	if strings.EqualFold(strings.TrimSpace(*binding.Locale), strings.TrimSpace(locale)) {
		return 0
	}
	return 2
}

func bindingStatusRank(binding *MenuLocationBinding, opts MenuQueryOptions) int {
	if binding == nil {
		return 2
	}
	status, err := normalizeMenuStatus(binding.Status)
	if err != nil {
		status = MenuStatusPublished
	}
	if opts.IncludeDrafts && strings.TrimSpace(opts.PreviewToken) != "" {
		if status == MenuStatusDraft {
			return 0
		}
		return 1
	}
	if status == MenuStatusPublished {
		return 0
	}
	return 1
}

func (s *service) applyViewProfileWithRecord(ctx context.Context, menu *Menu, profileCode string, opts MenuQueryOptions, env ...string) (*Menu, *MenuViewProfile, error) {
	if menu == nil {
		return nil, nil, ErrMenuNotFound
	}
	projected := cloneMenu(menu)
	code := strings.TrimSpace(profileCode)
	if code == "" {
		return projected, nil, nil
	}
	envID, _, err := s.resolveEnvironment(ctx, pickEnvironmentKey(env...))
	if err != nil {
		return nil, nil, err
	}
	profile, err := s.viewProfiles.GetByCode(ctx, code, envID.String())
	if err != nil {
		var notFound *NotFoundError
		if errors.As(err, &notFound) {
			return nil, nil, ErrMenuViewProfileNotFound
		}
		return nil, nil, err
	}
	if !s.statusAllowed(profile.Status, opts) {
		return nil, nil, ErrMenuViewProfileNotFound
	}
	mode, err := normalizeMenuViewMode(profile.Mode)
	if err != nil {
		return nil, nil, err
	}
	profile.Mode = mode
	projected = applyMenuProjection(projected, profile)
	return projected, profile, nil
}

func applyMenuProjection(menu *Menu, profile *MenuViewProfile) *Menu {
	if menu == nil {
		return nil
	}
	if profile == nil {
		return cloneMenu(menu)
	}
	projected := cloneMenu(menu)
	items := projected.Items
	includeSet := make(map[string]struct{}, len(profile.IncludeItemIDs))
	for _, id := range profile.IncludeItemIDs {
		key := strings.TrimSpace(id)
		if key != "" {
			includeSet[key] = struct{}{}
		}
	}
	excludeSet := make(map[string]struct{}, len(profile.ExcludeItemIDs))
	for _, id := range profile.ExcludeItemIDs {
		key := strings.TrimSpace(id)
		if key != "" {
			excludeSet[key] = struct{}{}
		}
	}

	switch profile.Mode {
	case MenuViewModeTopLevelLimit:
		items = projectTopLevelLimit(items, profile.MaxTopLevel)
	case MenuViewModeMaxDepth:
		items = projectMaxDepth(items, profile.MaxDepth, 1)
	case MenuViewModeIncludeIDs:
		items = projectIncludeIDs(items, includeSet)
	case MenuViewModeExcludeIDs:
		items = projectExcludeIDs(items, excludeSet)
	case MenuViewModeComposed:
		if len(includeSet) > 0 {
			items = projectIncludeIDs(items, includeSet)
		}
		if len(excludeSet) > 0 {
			items = projectExcludeIDs(items, excludeSet)
		}
		items = projectMaxDepth(items, profile.MaxDepth, 1)
		items = projectTopLevelLimit(items, profile.MaxTopLevel)
	default:
		// full mode keeps source items.
	}
	projected.Items = normalizeProjectedPositions(items)
	return projected
}

func projectTopLevelLimit(items []*MenuItem, max *int) []*MenuItem {
	if max == nil || *max <= 0 || len(items) <= *max {
		return items
	}
	return slices.Clone(items[:*max])
}

func projectMaxDepth(items []*MenuItem, max *int, depth int) []*MenuItem {
	if max == nil || *max <= 0 {
		return items
	}
	if depth >= *max {
		out := make([]*MenuItem, 0, len(items))
		for _, item := range items {
			if item == nil {
				continue
			}
			clone := cloneMenuItem(item)
			clone.Children = nil
			out = append(out, clone)
		}
		return out
	}
	out := make([]*MenuItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		clone := cloneMenuItem(item)
		clone.Children = projectMaxDepth(clone.Children, max, depth+1)
		out = append(out, clone)
	}
	return out
}

func projectIncludeIDs(items []*MenuItem, include map[string]struct{}) []*MenuItem {
	if len(include) == 0 {
		return items
	}
	out := make([]*MenuItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		children := projectIncludeIDs(item.Children, include)
		_, keepSelf := include[itemProjectionID(item)]
		if !keepSelf && len(children) == 0 {
			continue
		}
		clone := cloneMenuItem(item)
		clone.Children = children
		out = append(out, clone)
	}
	return out
}

func projectExcludeIDs(items []*MenuItem, exclude map[string]struct{}) []*MenuItem {
	if len(exclude) == 0 {
		return items
	}
	out := make([]*MenuItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, excluded := exclude[itemProjectionID(item)]; excluded {
			continue
		}
		clone := cloneMenuItem(item)
		clone.Children = projectExcludeIDs(clone.Children, exclude)
		out = append(out, clone)
	}
	return out
}

func normalizeProjectedPositions(items []*MenuItem) []*MenuItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]*MenuItem, 0, len(items))
	for idx, item := range items {
		if item == nil {
			continue
		}
		clone := cloneMenuItem(item)
		clone.Position = idx
		clone.Children = normalizeProjectedPositions(clone.Children)
		out = append(out, clone)
	}
	return out
}

func itemProjectionID(item *MenuItem) string {
	if item == nil {
		return ""
	}
	if code := strings.TrimSpace(item.ExternalCode); code != "" {
		return code
	}
	return item.ID.String()
}

func (s *service) navigationNodesForMenu(ctx context.Context, menu *Menu, locale string) ([]NavigationNode, error) {
	if menu == nil || len(menu.Items) == 0 {
		return nil, nil
	}
	envID := menu.EnvironmentID
	if envID == uuid.Nil {
		resolvedID, _, err := s.resolveEnvironment(ctx, "")
		if err != nil {
			return nil, err
		}
		envID = resolvedID
	}
	var localeID uuid.UUID
	if trimmed := strings.TrimSpace(locale); trimmed != "" {
		if loc, err := s.lookupLocale(ctx, trimmed); err == nil {
			localeID = loc.ID
		}
	}
	nodes := make([]NavigationNode, 0, len(menu.Items))
	for _, item := range menu.Items {
		node, err := s.buildNavigationNode(ctx, menu.Code, item, envID, localeID, locale)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return normalizeNavigationNodes(nodes), nil
}

func parseUUIDValue(value any) (uuid.UUID, bool, error) {
	switch v := value.(type) {
	case uuid.UUID:
		if v == uuid.Nil {
			return uuid.UUID{}, false, nil
		}
		return v, true, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return uuid.UUID{}, false, nil
		}
		id, err := uuid.Parse(trimmed)
		if err != nil {
			return uuid.UUID{}, false, err
		}
		return id, true, nil
	default:
		return uuid.UUID{}, false, fmt.Errorf("menus: unsupported identifier type %T", value)
	}
}

func ensureLeadingSlash(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func pickActor(ids ...uuid.UUID) uuid.UUID {
	for _, id := range ids {
		if id != uuid.Nil {
			return id
		}
	}
	return uuid.Nil
}

func (s *service) nextID() uuid.UUID {
	if s.newID == nil {
		return uuid.New()
	}
	id := s.newID()
	if id == uuid.Nil {
		return uuid.New()
	}
	return id
}

func (s *service) deterministicUUID(key string) uuid.UUID {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return s.nextID()
	}
	return identity.UUID(trimmed)
}

func collectMenuLocalesFromInputs(inputs []MenuItemTranslationInput) []string {
	if len(inputs) == 0 {
		return nil
	}
	locales := make([]string, 0, len(inputs))
	for _, tr := range inputs {
		code := strings.TrimSpace(tr.Locale)
		if code == "" {
			continue
		}
		locales = append(locales, code)
	}
	return locales
}

func collectMenuLocalesFromTranslations(translations []*MenuItemTranslation) []string {
	if len(translations) == 0 {
		return nil
	}
	locales := make([]string, 0, len(translations))
	for _, tr := range translations {
		if tr == nil {
			continue
		}
		if tr.Locale != nil && strings.TrimSpace(tr.Locale.Code) != "" {
			locales = append(locales, strings.TrimSpace(tr.Locale.Code))
			continue
		}
		locales = append(locales, tr.LocaleID.String())
	}
	return locales
}

func normalizeUUIDPtr(id *uuid.UUID) *uuid.UUID {
	if id == nil {
		return nil
	}
	copy := *id
	return &copy
}

func isValidCode(code string) bool {
	for _, r := range code {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' ||
			r == '_' {
			continue
		}
		return false
	}
	return true
}
