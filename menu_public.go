package cms

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

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
	Code        string
	Description *string
}

// NavigationNode is a localized, presentation-ready navigation node.
// This type intentionally omits UUIDs; menu identity is expressed via menu codes and item paths.
type NavigationNode struct {
	Type          string            `json:"type,omitempty"`
	Label         string            `json:"label,omitempty"`
	LabelKey      string            `json:"label_key,omitempty"`
	GroupTitle    string            `json:"group_title,omitempty"`
	GroupTitleKey string            `json:"group_title_key,omitempty"`
	URL           string            `json:"url"`
	Target        map[string]any    `json:"target,omitempty"`
	Icon          string            `json:"icon,omitempty"`
	Badge         map[string]any    `json:"badge,omitempty"`
	Permissions   []string          `json:"permissions,omitempty"`
	Classes       []string          `json:"classes,omitempty"`
	Styles        map[string]string `json:"styles,omitempty"`
	Collapsible   bool              `json:"collapsible,omitempty"`
	Collapsed     bool              `json:"collapsed,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
	Children      []NavigationNode  `json:"children,omitempty"`
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
	UpsertMenu(ctx context.Context, code string, description *string, actor uuid.UUID) (*MenuInfo, error)
	GetMenuByCode(ctx context.Context, code string) (*MenuInfo, error)
	ListMenuItemsByCode(ctx context.Context, menuCode string) ([]*MenuItemInfo, error)
	ResolveNavigation(ctx context.Context, menuCode string, locale string) ([]NavigationNode, error)
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
	Path        string
	ParentPath  string
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
	Position    *int
	Actor       uuid.UUID
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
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}

	record, err := s.svc.GetOrCreateMenu(ctx, menus.CreateMenuInput{
		Code:        code,
		Description: description,
		CreatedBy:   actor,
		UpdatedBy:   actor,
	})
	if err != nil {
		return nil, err
	}

	return &MenuInfo{
		Code:        record.Code,
		Description: record.Description,
	}, nil
}

func (s *menuService) UpsertMenu(ctx context.Context, code string, description *string, actor uuid.UUID) (*MenuInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}

	record, err := s.svc.UpsertMenu(ctx, menus.UpsertMenuInput{
		Code:        code,
		Description: description,
		Actor:       actor,
	})
	if err != nil {
		return nil, err
	}

	return &MenuInfo{
		Code:        record.Code,
		Description: record.Description,
	}, nil
}

func (s *menuService) GetMenuByCode(ctx context.Context, code string) (*MenuInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
	}

	record, err := s.svc.GetMenuByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrMenuNotFound
	}

	return &MenuInfo{
		Code:        record.Code,
		Description: record.Description,
	}, nil
}

func (s *menuService) ListMenuItemsByCode(ctx context.Context, menuCode string) ([]*MenuItemInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
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

func (s *menuService) ReconcileMenuByCode(ctx context.Context, menuCode string, actor uuid.UUID) (*ReconcileMenuResult, error) {
	if s == nil || s.module == nil || s.module.container == nil || s.svc == nil {
		return nil, errNilModule
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
	return s.svc.ResetMenuByCode(ctx, code, actor, force)
}

func (s *menuService) MoveMenuItemToTop(ctx context.Context, menuCode string, path string, actor uuid.UUID) error {
	return s.reorderByPaths(ctx, menuCode, actor, func(menuID uuid.UUID, items []*menus.MenuItem, byPath map[string]*menus.MenuItem) error {
		parsed, err := ParseMenuItemPathForMenu(menuCode, path)
		if err != nil {
			return err
		}
		item := byPath[parsed.Path]
		if item == nil {
			return fmt.Errorf("cms: menu item %q not found", parsed.Path)
		}
		targetParent := normalizeUUIDPtr(item.ParentID)
		moveSiblingToIndex(items, item.ID, targetParent, 0)
		return nil
	})
}

func (s *menuService) MoveMenuItemToBottom(ctx context.Context, menuCode string, path string, actor uuid.UUID) error {
	return s.reorderByPaths(ctx, menuCode, actor, func(menuID uuid.UUID, items []*menus.MenuItem, byPath map[string]*menus.MenuItem) error {
		parsed, err := ParseMenuItemPathForMenu(menuCode, path)
		if err != nil {
			return err
		}
		item := byPath[parsed.Path]
		if item == nil {
			return fmt.Errorf("cms: menu item %q not found", parsed.Path)
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
	return s.reorderByPaths(ctx, menuCode, actor, func(_ uuid.UUID, items []*menus.MenuItem, byPath map[string]*menus.MenuItem) error {
		parentID, err := resolveParentIDForPath(menuCode, parentPath, byPath)
		if err != nil {
			return err
		}

		desired := make([]uuid.UUID, 0, len(siblingPaths))
		seen := make(map[string]struct{}, len(siblingPaths))
		for _, raw := range siblingPaths {
			parsed, err := ParseMenuItemPathForMenu(menuCode, raw)
			if err != nil {
				return err
			}
			if _, ok := seen[parsed.Path]; ok {
				return fmt.Errorf("cms: duplicate sibling path %q", parsed.Path)
			}
			seen[parsed.Path] = struct{}{}

			item := byPath[parsed.Path]
			if item == nil {
				return fmt.Errorf("cms: menu item %q not found", parsed.Path)
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

	parsed, err := ParseMenuItemPath(input.Path)
	if err != nil {
		return nil, err
	}

	parentCode := ""
	if trimmedParent := strings.TrimSpace(input.ParentPath); trimmedParent != "" {
		if trimmedParent != parsed.MenuCode {
			parentParsed, err := ParseMenuItemPathForMenu(parsed.MenuCode, trimmedParent)
			if err != nil {
				return nil, err
			}
			parentCode = parentParsed.Path
		}
	} else if parsed.ParentPath != "" && parsed.ParentPath != parsed.MenuCode {
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
		MenuCode:                 parsed.MenuCode,
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

	parsed, err := ParseMenuItemPathForMenu(menuCode, path)
	if err != nil {
		return nil, err
	}

	item, err := s.svc.GetMenuItemByExternalCode(ctx, menuCode, parsed.Path)
	if err != nil {
		return nil, err
	}

	var parentID *uuid.UUID
	if input.ParentPath != nil {
		trimmed := strings.TrimSpace(*input.ParentPath)
		switch {
		case trimmed == "" || trimmed == menuCode:
			parentID = nil
		default:
			parentParsed, err := ParseMenuItemPathForMenu(menuCode, trimmed)
			if err != nil {
				return nil, err
			}
			parent, err := s.svc.GetMenuItemByExternalCode(ctx, menuCode, parentParsed.Path)
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
	parsed, err := ParseMenuItemPathForMenu(menuCode, path)
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
	parsed, err := ParseMenuItemPathForMenu(menuCode, path)
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

func toPublicNavigationNode(node menus.NavigationNode) NavigationNode {
	out := NavigationNode{
		Type:          node.Type,
		Label:         node.Label,
		LabelKey:      node.LabelKey,
		GroupTitle:    node.GroupTitle,
		GroupTitleKey: node.GroupTitleKey,
		URL:           node.URL,
		Target:        node.Target,
		Icon:          node.Icon,
		Badge:         node.Badge,
		Permissions:   node.Permissions,
		Classes:       node.Classes,
		Styles:        node.Styles,
		Collapsible:   node.Collapsible,
		Collapsed:     node.Collapsed,
		Metadata:      node.Metadata,
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
	return s.reorderByPaths(ctx, menuCode, actor, func(_ uuid.UUID, items []*menus.MenuItem, byPath map[string]*menus.MenuItem) error {
		parsed, err := ParseMenuItemPathForMenu(menuCode, path)
		if err != nil {
			return err
		}
		anchorParsed, err := ParseMenuItemPathForMenu(menuCode, anchorPath)
		if err != nil {
			return err
		}

		item := byPath[parsed.Path]
		if item == nil {
			return fmt.Errorf("cms: menu item %q not found", parsed.Path)
		}
		anchor := byPath[anchorParsed.Path]
		if anchor == nil {
			return fmt.Errorf("cms: menu item %q not found", anchorParsed.Path)
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
			return fmt.Errorf("cms: failed to locate anchor %q in siblings", anchorParsed.Path)
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
		MenuID:     menu.ID,
		Items:      orders,
		UpdatedBy:  actor,
		Version:    nil,
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
	trimmed := strings.TrimSpace(parentPath)
	if trimmed == "" || trimmed == menuCode {
		return nil, nil
	}
	parsed, err := ParseMenuItemPathForMenu(menuCode, trimmed)
	if err != nil {
		return nil, err
	}
	parent := byPath[parsed.Path]
	if parent == nil {
		return nil, fmt.Errorf("cms: menu item %q not found", parsed.Path)
	}
	return normalizeUUIDPtr(&parent.ID), nil
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
