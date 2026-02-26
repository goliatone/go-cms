package cms

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/menus"
	"github.com/google/uuid"
)

// Public menu errors.
var (
	ErrMenuNotFound = menus.ErrMenuNotFound
	ErrMenuInUse    = menus.ErrMenuInUse
)

var errNilModule = errors.New("cms: module is nil")

// MenuInfo is a stable public view of a menu record.
type MenuInfo struct {
	Code               string     `json:"code"`
	Location           string     `json:"location,omitempty"`
	Description        *string    `json:"description,omitempty"`
	Status             string     `json:"status"`
	Locale             *string    `json:"locale,omitempty"`
	TranslationGroupID *uuid.UUID `json:"translation_group_id,omitempty"`
	PublishedAt        *time.Time `json:"published_at,omitempty"`
}

type MenuResolveOptions struct {
	IncludeDrafts               bool   `json:"include_drafts,omitempty"`
	PreviewToken                string `json:"preview_token,omitempty"`
	Status                      string `json:"status,omitempty"`
	ViewProfile                 string `json:"view_profile,omitempty"`
	BindingPolicy               string `json:"binding_policy,omitempty"`
	IncludeContributions        *bool  `json:"include_contributions,omitempty"`
	ContributionMergeMode       string `json:"contribution_merge_mode,omitempty"`
	ContributionDuplicatePolicy string `json:"contribution_duplicate_policy,omitempty"`
}

type MenuLocationBindingInfo struct {
	Location        string  `json:"location"`
	MenuCode        string  `json:"menu_code"`
	ViewProfileCode *string `json:"view_profile_code,omitempty"`
	Locale          *string `json:"locale,omitempty"`
	Priority        int     `json:"priority"`
	Status          string  `json:"status"`
}

type MenuViewProfileInfo struct {
	Code           string   `json:"code"`
	Name           string   `json:"name"`
	Mode           string   `json:"mode"`
	MaxTopLevel    *int     `json:"max_top_level,omitempty"`
	MaxDepth       *int     `json:"max_depth,omitempty"`
	IncludeItemIDs []string `json:"include_item_ids,omitempty"`
	ExcludeItemIDs []string `json:"exclude_item_ids,omitempty"`
	Status         string   `json:"status"`
}

type ResolvedMenuPreviewInfo struct {
	IncludeDrafts       bool   `json:"include_drafts"`
	PreviewTokenPresent bool   `json:"preview_token_present"`
	MenuStatus          string `json:"menu_status,omitempty"`
	BindingStatus       string `json:"binding_status,omitempty"`
	ViewProfileStatus   string `json:"view_profile_status,omitempty"`
}

type ResolvedMenuInfo struct {
	Location           string                      `json:"location"`
	RequestedLocale    string                      `json:"requested_locale,omitempty"`
	ResolvedLocale     string                      `json:"resolved_locale,omitempty"`
	Menu               *MenuInfo                   `json:"menu,omitempty"`
	Binding            *MenuLocationBindingInfo    `json:"binding,omitempty"`
	Bindings           []MenuLocationBindingInfo   `json:"bindings,omitempty"`
	ViewProfile        *MenuViewProfileInfo        `json:"view_profile,omitempty"`
	Items              []NavigationNode            `json:"items,omitempty"`
	ContentMembership  []ContentMenuMembershipInfo `json:"content_membership,omitempty"`
	Preview            ResolvedMenuPreviewInfo     `json:"preview"`
	TranslationGroupID *uuid.UUID                  `json:"translation_group_id,omitempty"`
}

// NavigationNode is a localized, presentation-ready navigation node.
// This type intentionally omits UUIDs; menu identity is expressed via menu codes and item paths.
type NavigationNode struct {
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

type ContentMenuMembershipInfo struct {
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

// MenuService is the public menus API for the cms package.
//
// Identity rules:
// - Menus are addressed by code (e.g. "admin").
// - Menu items are addressed by dot-paths (e.g. "admin.content.pages") and never by UUID.
//
// UUIDs remain a persistence detail and must not be required for callers to use this API.
type MenuService interface {
	GetOrCreateMenu(ctx context.Context, code string, description *string, actor uuid.UUID) (*MenuInfo, error)
	GetOrCreateMenuWithLocation(ctx context.Context, code string, location string, description *string, actor uuid.UUID) (*MenuInfo, error)
	UpsertMenu(ctx context.Context, code string, description *string, actor uuid.UUID) (*MenuInfo, error)
	UpsertMenuWithLocation(ctx context.Context, code string, location string, description *string, actor uuid.UUID) (*MenuInfo, error)
	GetMenuByCode(ctx context.Context, code string) (*MenuInfo, error)
	GetMenuByLocation(ctx context.Context, location string) (*MenuInfo, error)
	ResolveMenuByCode(ctx context.Context, code string, locale string, opts MenuResolveOptions) (*ResolvedMenuInfo, error)
	ResolveMenuByLocation(ctx context.Context, location string, locale string, opts MenuResolveOptions) (*ResolvedMenuInfo, error)
	ListMenuItemsByCode(ctx context.Context, menuCode string) ([]*MenuItemInfo, error)
	ResolveNavigation(ctx context.Context, menuCode string, locale string) ([]NavigationNode, error)
	ResolveNavigationByLocation(ctx context.Context, location string, locale string) ([]NavigationNode, error)
	ReconcileMenuByCode(ctx context.Context, menuCode string, actor uuid.UUID) (*ReconcileMenuResult, error)
	ResetMenuByCode(ctx context.Context, code string, actor uuid.UUID, force bool) error

	UpsertMenuItemByPath(ctx context.Context, input UpsertMenuItemByPathInput) (*MenuItemInfo, error)
	UpdateMenuItemByPath(ctx context.Context, menuCode string, path string, input UpdateMenuItemByPathInput) (*MenuItemInfo, error)
	DeleteMenuItemByPath(ctx context.Context, menuCode string, path string, actor uuid.UUID, cascadeChildren bool) error
	UpsertMenuItemTranslationByPath(ctx context.Context, menuCode string, path string, input MenuItemTranslationInput) error

	MoveMenuItemToTop(ctx context.Context, menuCode string, path string, actor uuid.UUID) error
	MoveMenuItemToBottom(ctx context.Context, menuCode string, path string, actor uuid.UUID) error
	MoveMenuItemBefore(ctx context.Context, menuCode string, path string, beforePath string, actor uuid.UUID) error
	MoveMenuItemAfter(ctx context.Context, menuCode string, path string, afterPath string, actor uuid.UUID) error
	SetMenuSiblingOrder(ctx context.Context, menuCode string, parentPath string, siblingPaths []string, actor uuid.UUID) error
}

// MenuItemInfo is a stable public view of a menu item record.
// Identity is always expressed via Path (external_code), never via UUID.
type MenuItemInfo struct {
	Path        string            `json:"path"`
	Type        string            `json:"type,omitempty"`
	Position    int               `json:"position,omitempty"`
	Target      map[string]any    `json:"target,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	Badge       map[string]any    `json:"badge,omitempty"`
	Permissions []string          `json:"permissions,omitempty"`
	Classes     []string          `json:"classes,omitempty"`
	Styles      map[string]string `json:"styles,omitempty"`
	Collapsible bool              `json:"collapsible,omitempty"`
	Collapsed   bool              `json:"collapsed,omitempty"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

type MenuItemTranslationInput struct {
	Locale        string  `json:"locale"`
	Label         string  `json:"label,omitempty"`
	LabelKey      string  `json:"label_key,omitempty"`
	GroupTitle    string  `json:"group_title,omitempty"`
	GroupTitleKey string  `json:"group_title_key,omitempty"`
	URLOverride   *string `json:"url_override,omitempty"`
}

type ReconcileMenuResult struct {
	Resolved  int `json:"resolved"`
	Remaining int `json:"remaining"`
}

type UpsertMenuItemByPathInput struct {
	Path       string
	ParentPath string
	// Position is a 0-based insertion index among siblings.
	// Values past the end are clamped to append. Nil defaults to append for new items.
	Position    *int
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

	Translations             []MenuItemTranslationInput
	AllowMissingTranslations bool
	Actor                    uuid.UUID
}

type UpdateMenuItemByPathInput struct {
	ParentPath  *string
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
	Position *int
	Actor    uuid.UUID
}

type menuService struct {
	module *Module
	svc    menus.Service
}

func newMenuService(m *Module) MenuService {
	if m == nil || m.container == nil {
		return &menuService{module: m, svc: nil}
	}
	return &menuService{module: m, svc: m.container.MenuService()}
}

func (s *menuService) GetOrCreateMenu(ctx context.Context, code string, description *string, actor uuid.UUID) (*MenuInfo, error) {
	return s.GetOrCreateMenuWithLocation(ctx, code, "", description, actor)
}

func (s *menuService) GetOrCreateMenuWithLocation(ctx context.Context, code string, location string, description *string, actor uuid.UUID) (*MenuInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}

	code = CanonicalMenuCode(code)
	if code == "" {
		return nil, ErrMenuCodeRequired
	}

	record, err := s.svc.GetOrCreateMenu(ctx, menus.CreateMenuInput{
		Code:        code,
		Location:    location,
		Description: description,
		CreatedBy:   actor,
		UpdatedBy:   actor,
	})
	if err != nil {
		return nil, err
	}

	return toPublicMenuInfo(record), nil
}

func (s *menuService) UpsertMenu(ctx context.Context, code string, description *string, actor uuid.UUID) (*MenuInfo, error) {
	return s.UpsertMenuWithLocation(ctx, code, "", description, actor)
}

func (s *menuService) UpsertMenuWithLocation(ctx context.Context, code string, location string, description *string, actor uuid.UUID) (*MenuInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}

	code = CanonicalMenuCode(code)
	if code == "" {
		return nil, ErrMenuCodeRequired
	}

	record, err := s.svc.UpsertMenu(ctx, menus.UpsertMenuInput{
		Code:        code,
		Location:    location,
		Description: description,
		Actor:       actor,
	})
	if err != nil {
		return nil, err
	}

	return toPublicMenuInfo(record), nil
}

func (s *menuService) GetMenuByCode(ctx context.Context, code string) (*MenuInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}

	code = CanonicalMenuCode(code)
	if code == "" {
		return nil, ErrMenuCodeRequired
	}

	record, err := s.svc.GetMenuByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrMenuNotFound
	}

	return toPublicMenuInfo(record), nil
}

func (s *menuService) GetMenuByLocation(ctx context.Context, location string) (*MenuInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}

	location = strings.TrimSpace(location)
	if location == "" {
		return nil, ErrMenuCodeRequired
	}

	record, err := s.svc.GetMenuByLocation(ctx, location)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrMenuNotFound
	}

	return toPublicMenuInfo(record), nil
}

func (s *menuService) ResolveMenuByCode(ctx context.Context, code string, locale string, opts MenuResolveOptions) (*ResolvedMenuInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}
	code = CanonicalMenuCode(code)
	if code == "" {
		return nil, ErrMenuCodeRequired
	}
	resolved, err := s.svc.MenuByCode(ctx, code, locale, toInternalResolveOptions(opts))
	if err != nil {
		return nil, err
	}
	return toPublicResolvedMenu(resolved), nil
}

func (s *menuService) ResolveMenuByLocation(ctx context.Context, location string, locale string, opts MenuResolveOptions) (*ResolvedMenuInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}
	location = strings.TrimSpace(location)
	if location == "" {
		return nil, ErrMenuCodeRequired
	}
	resolved, err := s.svc.MenuByLocation(ctx, location, locale, toInternalResolveOptions(opts))
	if err != nil {
		return nil, err
	}
	return toPublicResolvedMenu(resolved), nil
}

func (s *menuService) ListMenuItemsByCode(ctx context.Context, menuCode string) ([]*MenuItemInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}

	menuCode = CanonicalMenuCode(menuCode)
	if menuCode == "" {
		return nil, ErrMenuCodeRequired
	}

	menu, err := s.svc.GetMenuByCode(ctx, menuCode)
	if err != nil {
		return nil, err
	}

	items := flattenMenuItems(menu.Items)
	sort.Slice(items, func(i, j int) bool {
		pi, pj := uuidPtrKey(items[i].ParentID), uuidPtrKey(items[j].ParentID)
		if pi != pj {
			return pi < pj
		}
		if items[i].Position != items[j].Position {
			return items[i].Position < items[j].Position
		}
		return items[i].ExternalCode < items[j].ExternalCode
	})

	out := make([]*MenuItemInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPublicMenuItemInfo(item))
	}
	return out, nil
}

func (s *menuService) ResolveNavigation(ctx context.Context, menuCode string, locale string) ([]NavigationNode, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}

	menuCode = CanonicalMenuCode(menuCode)
	if menuCode == "" {
		return nil, ErrMenuCodeRequired
	}

	nodes, err := s.svc.ResolveNavigation(ctx, menuCode, locale)
	if err != nil {
		return nil, err
	}

	out := make([]NavigationNode, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, toPublicNavigationNode(node))
	}
	return out, nil
}

func (s *menuService) ResolveNavigationByLocation(ctx context.Context, location string, locale string) ([]NavigationNode, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}
	location = strings.TrimSpace(location)
	if location == "" {
		return nil, ErrMenuCodeRequired
	}

	nodes, err := s.svc.ResolveNavigationByLocation(ctx, location, locale)
	if err != nil {
		return nil, err
	}

	out := make([]NavigationNode, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, toPublicNavigationNode(node))
	}
	return out, nil
}

func (s *menuService) ReconcileMenuByCode(ctx context.Context, menuCode string, actor uuid.UUID) (*ReconcileMenuResult, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}

	menuCode = CanonicalMenuCode(menuCode)
	if menuCode == "" {
		return nil, ErrMenuCodeRequired
	}

	menu, err := s.svc.GetMenuByCode(ctx, menuCode)
	if err != nil {
		return nil, err
	}
	result, err := s.svc.ReconcileMenu(ctx, menus.ReconcileMenuRequest{
		MenuID:    menu.ID,
		UpdatedBy: actor,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &ReconcileMenuResult{}, nil
	}
	return &ReconcileMenuResult{
		Resolved:  result.Resolved,
		Remaining: result.Remaining,
	}, nil
}

func (s *menuService) ResetMenuByCode(ctx context.Context, code string, actor uuid.UUID, force bool) error {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return errNilModule
	}
	code = CanonicalMenuCode(code)
	if code == "" {
		return ErrMenuCodeRequired
	}
	return s.svc.ResetMenuByCode(ctx, code, actor, force)
}

func (s *menuService) MoveMenuItemToTop(ctx context.Context, menuCode string, path string, actor uuid.UUID) error {
	menuCode = CanonicalMenuCode(menuCode)
	if menuCode == "" {
		return ErrMenuCodeRequired
	}
	canonicalPath, err := CanonicalMenuItemPath(menuCode, path)
	if err != nil {
		return err
	}

	return s.reorderByPaths(ctx, menuCode, actor, func(menuID uuid.UUID, items []*menus.MenuItem, byPath map[string]*menus.MenuItem) error {
		item := byPath[canonicalPath]
		if item == nil {
			return fmt.Errorf("cms: menu item %q not found", canonicalPath)
		}
		targetParent := normalizeUUIDPtr(item.ParentID)
		moveSiblingToIndex(items, item.ID, targetParent, 0)
		return nil
	})
}

func (s *menuService) MoveMenuItemToBottom(ctx context.Context, menuCode string, path string, actor uuid.UUID) error {
	menuCode = CanonicalMenuCode(menuCode)
	if menuCode == "" {
		return ErrMenuCodeRequired
	}
	canonicalPath, err := CanonicalMenuItemPath(menuCode, path)
	if err != nil {
		return err
	}

	return s.reorderByPaths(ctx, menuCode, actor, func(menuID uuid.UUID, items []*menus.MenuItem, byPath map[string]*menus.MenuItem) error {
		item := byPath[canonicalPath]
		if item == nil {
			return fmt.Errorf("cms: menu item %q not found", canonicalPath)
		}
		targetParent := normalizeUUIDPtr(item.ParentID)
		moveSiblingToIndex(items, item.ID, targetParent, int(^uint(0)>>1))
		return nil
	})
}

func (s *menuService) MoveMenuItemBefore(ctx context.Context, menuCode string, path string, beforePath string, actor uuid.UUID) error {
	return s.moveMenuItemRelative(ctx, menuCode, path, beforePath, actor, true)
}

func (s *menuService) MoveMenuItemAfter(ctx context.Context, menuCode string, path string, afterPath string, actor uuid.UUID) error {
	return s.moveMenuItemRelative(ctx, menuCode, path, afterPath, actor, false)
}

func (s *menuService) SetMenuSiblingOrder(ctx context.Context, menuCode string, parentPath string, siblingPaths []string, actor uuid.UUID) error {
	menuCode = CanonicalMenuCode(menuCode)
	if menuCode == "" {
		return ErrMenuCodeRequired
	}

	parentPath, err := canonicalParentPath(menuCode, parentPath)
	if err != nil {
		return err
	}

	return s.reorderByPaths(ctx, menuCode, actor, func(_ uuid.UUID, items []*menus.MenuItem, byPath map[string]*menus.MenuItem) error {
		parentID, err := resolveParentIDForPath(menuCode, parentPath, byPath)
		if err != nil {
			return err
		}

		desired := make([]uuid.UUID, 0, len(siblingPaths))
		seen := make(map[string]struct{}, len(siblingPaths))
		for _, raw := range siblingPaths {
			canonicalSiblingPath, err := CanonicalMenuItemPath(menuCode, raw)
			if err != nil {
				return err
			}
			if _, ok := seen[canonicalSiblingPath]; ok {
				return fmt.Errorf("cms: duplicate sibling path %q", canonicalSiblingPath)
			}
			seen[canonicalSiblingPath] = struct{}{}

			item := byPath[canonicalSiblingPath]
			if item == nil {
				return fmt.Errorf("cms: menu item %q not found", canonicalSiblingPath)
			}
			item.ParentID = parentID
			desired = append(desired, item.ID)
		}

		applySiblingOrder(items, parentID, desired)
		return nil
	})
}

func (s *menuService) UpsertMenuItemByPath(ctx context.Context, input UpsertMenuItemByPathInput) (*MenuItemInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}

	trimmedPath := strings.TrimSpace(input.Path)
	if trimmedPath == "" {
		return nil, ErrMenuItemPathRequired
	}

	menuCode := ""
	trimmedParent := strings.TrimSpace(input.ParentPath)
	switch {
	case trimmedParent != "" && (strings.Contains(trimmedParent, ".") || strings.Contains(trimmedParent, "/")):
		parentParsed, err := ParseMenuItemPath(trimmedParent)
		if err != nil {
			return nil, err
		}
		menuCode = CanonicalMenuCode(parentParsed.MenuCode)
	case trimmedParent != "":
		menuCode = CanonicalMenuCode(trimmedParent)
	default:
		parsed, err := ParseMenuItemPath(trimmedPath)
		if err != nil {
			return nil, err
		}
		menuCode = CanonicalMenuCode(parsed.MenuCode)
	}

	if menuCode == "" {
		return nil, ErrMenuCodeRequired
	}

	derived, err := DeriveMenuItemPaths(menuCode, trimmedPath, trimmedParent, "")
	if err != nil {
		return nil, err
	}
	parsed, err := ParseMenuItemPathForMenu(menuCode, derived.Path)
	if err != nil {
		return nil, err
	}

	parentCode := ""
	if trimmedParent != "" {
		parentPath, err := canonicalParentPath(menuCode, trimmedParent)
		if err != nil {
			return nil, err
		}
		if parentPath != "" && parentPath != menuCode {
			parentCode = parentPath
		}
	} else if parsed.ParentPath != "" && parsed.ParentPath != menuCode {
		parentCode = parsed.ParentPath
	}

	translations := make([]menus.MenuItemTranslationInput, 0, len(input.Translations))
	for _, tr := range input.Translations {
		translations = append(translations, menus.MenuItemTranslationInput{
			Locale:        tr.Locale,
			Label:         tr.Label,
			LabelKey:      tr.LabelKey,
			GroupTitle:    tr.GroupTitle,
			GroupTitleKey: tr.GroupTitleKey,
			URLOverride:   tr.URLOverride,
		})
	}

	item, err := s.svc.UpsertMenuItem(ctx, menus.UpsertMenuItemInput{
		MenuCode:                 menuCode,
		ExternalCode:             parsed.Path,
		ParentCode:               parentCode,
		Position:                 input.Position,
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
		Translations:             translations,
		AllowMissingTranslations: input.AllowMissingTranslations,
		Actor:                    input.Actor,
	})
	if err != nil {
		return nil, err
	}
	return toPublicMenuItemInfo(item), nil
}

func (s *menuService) UpdateMenuItemByPath(ctx context.Context, menuCode string, path string, input UpdateMenuItemByPathInput) (*MenuItemInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}

	menuCode = CanonicalMenuCode(menuCode)
	if menuCode == "" {
		return nil, ErrMenuCodeRequired
	}

	canonicalPath, err := CanonicalMenuItemPath(menuCode, path)
	if err != nil {
		return nil, err
	}

	parsed, err := ParseMenuItemPathForMenu(menuCode, canonicalPath)
	if err != nil {
		return nil, err
	}

	item, err := s.svc.GetMenuItemByExternalCode(ctx, menuCode, parsed.Path)
	if err != nil {
		return nil, err
	}

	var parentID *uuid.UUID
	if input.ParentPath != nil {
		parentPath, err := canonicalParentPath(menuCode, *input.ParentPath)
		if err != nil {
			return nil, err
		}
		switch {
		case parentPath == "" || parentPath == menuCode:
			parentID = nil
		default:
			parent, err := s.svc.GetMenuItemByExternalCode(ctx, menuCode, parentPath)
			if err != nil {
				return nil, err
			}
			parentID = &parent.ID
		}
	}

	updated, err := s.svc.UpdateMenuItem(ctx, menus.UpdateMenuItemInput{
		ItemID:      item.ID,
		Type:        input.Type,
		Target:      input.Target,
		Icon:        input.Icon,
		Badge:       input.Badge,
		Permissions: input.Permissions,
		Classes:     input.Classes,
		Styles:      input.Styles,
		Collapsible: input.Collapsible,
		Collapsed:   input.Collapsed,
		Metadata:    input.Metadata,
		Position:    input.Position,
		ParentID:    parentID,
		UpdatedBy:   input.Actor,
	})
	if err != nil {
		return nil, err
	}
	return toPublicMenuItemInfo(updated), nil
}

func (s *menuService) DeleteMenuItemByPath(ctx context.Context, menuCode string, path string, actor uuid.UUID, cascadeChildren bool) error {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return errNilModule
	}
	menuCode = CanonicalMenuCode(menuCode)
	if menuCode == "" {
		return ErrMenuCodeRequired
	}

	canonicalPath, err := CanonicalMenuItemPath(menuCode, path)
	if err != nil {
		return err
	}

	parsed, err := ParseMenuItemPathForMenu(menuCode, canonicalPath)
	if err != nil {
		return err
	}
	item, err := s.svc.GetMenuItemByExternalCode(ctx, menuCode, parsed.Path)
	if err != nil {
		return err
	}
	return s.svc.DeleteMenuItem(ctx, menus.DeleteMenuItemRequest{
		ItemID:          item.ID,
		DeletedBy:       actor,
		CascadeChildren: cascadeChildren,
	})
}

func (s *menuService) UpsertMenuItemTranslationByPath(ctx context.Context, menuCode string, path string, input MenuItemTranslationInput) error {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return errNilModule
	}
	menuCode = CanonicalMenuCode(menuCode)
	if menuCode == "" {
		return ErrMenuCodeRequired
	}

	canonicalPath, err := CanonicalMenuItemPath(menuCode, path)
	if err != nil {
		return err
	}

	parsed, err := ParseMenuItemPathForMenu(menuCode, canonicalPath)
	if err != nil {
		return err
	}
	item, err := s.svc.GetMenuItemByExternalCode(ctx, menuCode, parsed.Path)
	if err != nil {
		return err
	}
	_, err = s.svc.UpsertMenuItemTranslation(ctx, menus.UpsertMenuItemTranslationInput{
		ItemID:        item.ID,
		Locale:        input.Locale,
		Label:         input.Label,
		LabelKey:      input.LabelKey,
		GroupTitle:    input.GroupTitle,
		GroupTitleKey: input.GroupTitleKey,
		URLOverride:   input.URLOverride,
	})
	return err
}

func toInternalResolveOptions(opts MenuResolveOptions) menus.MenuQueryOptions {
	return menus.MenuQueryOptions{
		IncludeDrafts:               opts.IncludeDrafts,
		PreviewToken:                strings.TrimSpace(opts.PreviewToken),
		Status:                      strings.TrimSpace(opts.Status),
		ViewProfile:                 strings.TrimSpace(opts.ViewProfile),
		BindingPolicy:               strings.TrimSpace(opts.BindingPolicy),
		IncludeContributions:        opts.IncludeContributions,
		ContributionMergeMode:       strings.TrimSpace(opts.ContributionMergeMode),
		ContributionDuplicatePolicy: strings.TrimSpace(opts.ContributionDuplicatePolicy),
	}
}

func toPublicMenuInfo(record *menus.Menu) *MenuInfo {
	if record == nil {
		return nil
	}
	out := &MenuInfo{
		Code:        record.Code,
		Location:    record.Location,
		Description: record.Description,
		Status:      record.Status,
		Locale:      record.Locale,
		PublishedAt: record.PublishedAt,
	}
	if record.TranslationGroupID != nil {
		groupID := *record.TranslationGroupID
		out.TranslationGroupID = &groupID
	}
	return out
}

func toPublicResolvedMenu(resolved *menus.ResolvedMenu) *ResolvedMenuInfo {
	if resolved == nil {
		return nil
	}
	out := &ResolvedMenuInfo{
		Location:        resolved.Location,
		RequestedLocale: resolved.RequestedLocale,
		ResolvedLocale:  resolved.ResolvedLocale,
		Menu:            toPublicMenuInfo(resolved.Menu),
		Preview: ResolvedMenuPreviewInfo{
			IncludeDrafts:       resolved.Preview.IncludeDrafts,
			PreviewTokenPresent: resolved.Preview.PreviewTokenPresent,
			MenuStatus:          resolved.Preview.MenuStatus,
			BindingStatus:       resolved.Preview.BindingStatus,
			ViewProfileStatus:   resolved.Preview.ViewProfileStatus,
		},
	}
	if resolved.Binding != nil {
		out.Binding = &MenuLocationBindingInfo{
			Location:        resolved.Binding.Location,
			MenuCode:        resolved.Binding.MenuCode,
			ViewProfileCode: resolved.Binding.ViewProfileCode,
			Locale:          resolved.Binding.Locale,
			Priority:        resolved.Binding.Priority,
			Status:          resolved.Binding.Status,
		}
	}
	if len(resolved.Bindings) > 0 {
		out.Bindings = make([]MenuLocationBindingInfo, 0, len(resolved.Bindings))
		for _, binding := range resolved.Bindings {
			if binding == nil {
				continue
			}
			out.Bindings = append(out.Bindings, MenuLocationBindingInfo{
				Location:        binding.Location,
				MenuCode:        binding.MenuCode,
				ViewProfileCode: binding.ViewProfileCode,
				Locale:          binding.Locale,
				Priority:        binding.Priority,
				Status:          binding.Status,
			})
		}
	}
	if resolved.ViewProfile != nil {
		out.ViewProfile = &MenuViewProfileInfo{
			Code:           resolved.ViewProfile.Code,
			Name:           resolved.ViewProfile.Name,
			Mode:           resolved.ViewProfile.Mode,
			MaxTopLevel:    resolved.ViewProfile.MaxTopLevel,
			MaxDepth:       resolved.ViewProfile.MaxDepth,
			IncludeItemIDs: append([]string{}, resolved.ViewProfile.IncludeItemIDs...),
			ExcludeItemIDs: append([]string{}, resolved.ViewProfile.ExcludeItemIDs...),
			Status:         resolved.ViewProfile.Status,
		}
	}
	if resolved.TranslationGroupID != nil {
		groupID := *resolved.TranslationGroupID
		out.TranslationGroupID = &groupID
	}
	if len(resolved.Items) > 0 {
		out.Items = make([]NavigationNode, 0, len(resolved.Items))
		for _, node := range resolved.Items {
			out.Items = append(out.Items, toPublicNavigationNode(node))
		}
	}
	if len(resolved.ContentMembership) > 0 {
		out.ContentMembership = make([]ContentMenuMembershipInfo, 0, len(resolved.ContentMembership))
		for _, membership := range resolved.ContentMembership {
			out.ContentMembership = append(out.ContentMembership, ContentMenuMembershipInfo{
				ContentID:       membership.ContentID,
				ContentTypeID:   membership.ContentTypeID,
				ContentTypeSlug: membership.ContentTypeSlug,
				ContentSlug:     membership.ContentSlug,
				Location:        membership.Location,
				Visible:         membership.Visible,
				Origin:          membership.Origin,
				VisibilityState: membership.VisibilityState,
				MergeMode:       membership.MergeMode,
				DuplicatePolicy: membership.DuplicatePolicy,
				URL:             membership.URL,
				Label:           membership.Label,
				SortOrder:       membership.SortOrder,
			})
		}
	}
	return out
}

func toPublicNavigationNode(node menus.NavigationNode) NavigationNode {
	out := NavigationNode{
		Position:           node.Position,
		Type:               node.Type,
		Label:              node.Label,
		LabelKey:           node.LabelKey,
		GroupTitle:         node.GroupTitle,
		GroupTitleKey:      node.GroupTitleKey,
		URL:                node.URL,
		Target:             node.Target,
		Icon:               node.Icon,
		Badge:              node.Badge,
		Permissions:        node.Permissions,
		Classes:            node.Classes,
		Styles:             node.Styles,
		Collapsible:        node.Collapsible,
		Collapsed:          node.Collapsed,
		Metadata:           node.Metadata,
		Contribution:       node.Contribution,
		ContributionOrigin: node.ContributionOrigin,
	}
	if len(node.Children) > 0 {
		out.Children = make([]NavigationNode, 0, len(node.Children))
		for _, child := range node.Children {
			out.Children = append(out.Children, toPublicNavigationNode(child))
		}
	}
	return out
}

func toPublicMenuItemInfo(item *menus.MenuItem) *MenuItemInfo {
	if item == nil {
		return nil
	}
	return &MenuItemInfo{
		Path:        item.ExternalCode,
		Type:        item.Type,
		Position:    item.Position,
		Target:      item.Target,
		Icon:        item.Icon,
		Badge:       item.Badge,
		Permissions: item.Permissions,
		Classes:     item.Classes,
		Styles:      item.Styles,
		Collapsible: item.Collapsible,
		Collapsed:   item.Collapsed,
		Metadata:    item.Metadata,
	}
}

func (s *menuService) moveMenuItemRelative(ctx context.Context, menuCode string, path string, anchorPath string, actor uuid.UUID, before bool) error {
	menuCode = CanonicalMenuCode(menuCode)
	if menuCode == "" {
		return ErrMenuCodeRequired
	}
	canonicalPath, err := CanonicalMenuItemPath(menuCode, path)
	if err != nil {
		return err
	}
	canonicalAnchorPath, err := CanonicalMenuItemPath(menuCode, anchorPath)
	if err != nil {
		return err
	}

	return s.reorderByPaths(ctx, menuCode, actor, func(_ uuid.UUID, items []*menus.MenuItem, byPath map[string]*menus.MenuItem) error {
		item := byPath[canonicalPath]
		if item == nil {
			return fmt.Errorf("cms: menu item %q not found", canonicalPath)
		}
		anchor := byPath[canonicalAnchorPath]
		if anchor == nil {
			return fmt.Errorf("cms: menu item %q not found", canonicalAnchorPath)
		}
		if item.ID == anchor.ID {
			return nil
		}

		targetParent := normalizeUUIDPtr(anchor.ParentID)
		item.ParentID = targetParent

		siblings := collectSiblings(items, targetParent)
		filtered := make([]*menus.MenuItem, 0, len(siblings))
		for _, sibling := range siblings {
			if sibling.ID == item.ID {
				continue
			}
			filtered = append(filtered, sibling)
		}
		insertIndex := -1
		for idx, sibling := range filtered {
			if sibling.ID == anchor.ID {
				insertIndex = idx
				break
			}
		}
		if insertIndex < 0 {
			return fmt.Errorf("cms: failed to locate anchor %q in siblings", canonicalAnchorPath)
		}
		if !before {
			insertIndex++
		}

		moveSiblingToIndex(items, item.ID, targetParent, insertIndex)
		return nil
	})
}

func (s *menuService) reorderByPaths(ctx context.Context, menuCode string, actor uuid.UUID, mutate func(menuID uuid.UUID, items []*menus.MenuItem, byPath map[string]*menus.MenuItem) error) error {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return errNilModule
	}
	menuCode = CanonicalMenuCode(menuCode)
	if menuCode == "" {
		return ErrMenuCodeRequired
	}
	menu, err := s.svc.GetMenuByCode(ctx, menuCode)
	if err != nil {
		return err
	}
	if _, err := s.svc.ReconcileMenu(ctx, menus.ReconcileMenuRequest{MenuID: menu.ID, UpdatedBy: actor}); err != nil {
		return err
	}

	menu, err = s.svc.GetMenuByCode(ctx, menuCode)
	if err != nil {
		return err
	}
	items := flattenMenuItems(menu.Items)
	byPath := make(map[string]*menus.MenuItem, len(items))
	for _, item := range items {
		if item.ExternalCode == "" {
			continue
		}
		byPath[item.ExternalCode] = item
	}

	if err := mutate(menu.ID, items, byPath); err != nil {
		return err
	}

	normalizeAllPositions(items)

	orders := make([]menus.ItemOrder, 0, len(items))
	for _, item := range items {
		orders = append(orders, menus.ItemOrder{
			ItemID:   item.ID,
			ParentID: normalizeUUIDPtr(item.ParentID),
			Position: item.Position,
		})
	}

	_, err = s.svc.BulkReorderMenuItems(ctx, menus.BulkReorderMenuItemsInput{
		MenuID:    menu.ID,
		Items:     orders,
		UpdatedBy: actor,
		Version:   nil,
	})
	return err
}

func flattenMenuItems(items []*menus.MenuItem) []*menus.MenuItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]*menus.MenuItem, 0, len(items))
	var walk func(nodes []*menus.MenuItem)
	walk = func(nodes []*menus.MenuItem) {
		for _, item := range nodes {
			if item == nil {
				continue
			}
			out = append(out, item)
			if len(item.Children) > 0 {
				walk(item.Children)
			}
		}
	}
	walk(items)
	return out
}

func uuidPtrKey(id *uuid.UUID) string {
	if id == nil || *id == uuid.Nil {
		return ""
	}
	return id.String()
}

func normalizeUUIDPtr(id *uuid.UUID) *uuid.UUID {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	cp := *id
	return &cp
}

func resolveParentIDForPath(menuCode string, parentPath string, byPath map[string]*menus.MenuItem) (*uuid.UUID, error) {
	menuCode = CanonicalMenuCode(menuCode)
	if menuCode == "" {
		return nil, ErrMenuCodeRequired
	}

	trimmed := strings.TrimSpace(parentPath)
	parent, err := canonicalParentPath(menuCode, trimmed)
	if err != nil {
		return nil, err
	}
	if parent == "" || parent == menuCode {
		return nil, nil
	}
	record := byPath[parent]
	if record == nil {
		return nil, fmt.Errorf("cms: menu item %q not found", parent)
	}
	return normalizeUUIDPtr(&record.ID), nil
}

func moveSiblingToIndex(items []*menus.MenuItem, itemID uuid.UUID, parentID *uuid.UUID, insertIndex int) {
	siblings := collectSiblings(items, parentID)
	var moving *menus.MenuItem
	rest := make([]*menus.MenuItem, 0, len(siblings))
	for _, sibling := range siblings {
		if sibling.ID == itemID {
			moving = sibling
			continue
		}
		rest = append(rest, sibling)
	}
	if moving == nil {
		return
	}
	moving.ParentID = parentID

	if insertIndex < 0 {
		insertIndex = 0
	}
	if insertIndex > len(rest) {
		insertIndex = len(rest)
	}

	ordered := make([]*menus.MenuItem, 0, len(rest)+1)
	ordered = append(ordered, rest[:insertIndex]...)
	ordered = append(ordered, moving)
	ordered = append(ordered, rest[insertIndex:]...)
	applyPositions(parentID, ordered)
}

func applySiblingOrder(items []*menus.MenuItem, parentID *uuid.UUID, desired []uuid.UUID) {
	siblings := collectSiblings(items, parentID)
	index := make(map[uuid.UUID]*menus.MenuItem, len(siblings))
	for _, sibling := range siblings {
		index[sibling.ID] = sibling
	}

	ordered := make([]*menus.MenuItem, 0, len(siblings))
	seen := make(map[uuid.UUID]struct{}, len(siblings))
	for _, id := range desired {
		item := index[id]
		if item == nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		ordered = append(ordered, item)
		seen[id] = struct{}{}
	}
	for _, sibling := range siblings {
		if _, ok := seen[sibling.ID]; ok {
			continue
		}
		ordered = append(ordered, sibling)
	}
	applyPositions(parentID, ordered)
}

func applyPositions(parentID *uuid.UUID, siblings []*menus.MenuItem) {
	for idx, sibling := range siblings {
		sibling.ParentID = parentID
		sibling.Position = idx
	}
}

func collectSiblings(items []*menus.MenuItem, parentID *uuid.UUID) []*menus.MenuItem {
	out := make([]*menus.MenuItem, 0)
	for _, item := range items {
		if item == nil {
			continue
		}
		if uuidPtrEqual(item.ParentID, parentID) {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Position != out[j].Position {
			return out[i].Position < out[j].Position
		}
		return out[i].ExternalCode < out[j].ExternalCode
	})
	return out
}

func uuidPtrEqual(a *uuid.UUID, b *uuid.UUID) bool {
	if a == nil || *a == uuid.Nil {
		return b == nil || *b == uuid.Nil
	}
	if b == nil || *b == uuid.Nil {
		return false
	}
	return *a == *b
}

func normalizeAllPositions(items []*menus.MenuItem) {
	groups := make(map[string][]*menus.MenuItem)
	for _, item := range items {
		if item == nil {
			continue
		}
		key := uuidPtrKey(item.ParentID)
		groups[key] = append(groups[key], item)
	}

	for _, siblings := range groups {
		sort.Slice(siblings, func(i, j int) bool {
			if siblings[i].Position != siblings[j].Position {
				return siblings[i].Position < siblings[j].Position
			}
			return siblings[i].ExternalCode < siblings[j].ExternalCode
		})
		for idx, sibling := range siblings {
			sibling.Position = idx
		}
	}
}
