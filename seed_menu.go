package cms

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

var ErrSeedMenuServiceRequired = errors.New("cms: menu service is required")

type SeedMenuOptions struct {
	Menus             MenuService
	MenuCode          string
	Description       *string
	Locale            string
	Actor             uuid.UUID
	Items             []SeedMenuItem
	AutoCreateParents bool
	// Ensure makes SeedMenu converge persisted menu items onto the provided spec.
	// It performs reconciliation and enforces deterministic sibling ordering from the spec.
	Ensure bool
	// PruneUnspecified deletes menu items (by path) that exist in persistence but are not present in the spec.
	// Deletions are performed by path and cascade through any descendant items.
	PruneUnspecified bool
}

type SeedMenuItem struct {
	Path        string
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
}

func SeedMenu(ctx context.Context, opts SeedMenuOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Menus == nil {
		return ErrSeedMenuServiceRequired
	}

	menuCode := strings.TrimSpace(opts.MenuCode)
	if menuCode == "" {
		return ErrMenuCodeRequired
	}

	if _, err := opts.Menus.UpsertMenu(ctx, menuCode, opts.Description, opts.Actor); err != nil {
		return err
	}

	defaultLocale := strings.TrimSpace(opts.Locale)

	explicit := make(map[string]SeedMenuItem, len(opts.Items))
	for _, item := range opts.Items {
		path := strings.TrimSpace(item.Path)
		if path == "" {
			return ErrMenuItemPathRequired
		}

		parsed, err := ParseMenuItemPathForMenu(menuCode, path)
		if err != nil {
			return err
		}

		item.Path = parsed.Path
		if _, exists := explicit[item.Path]; exists {
			return fmt.Errorf("cms: duplicate seed menu item path %q", item.Path)
		}

		if len(item.Translations) > 0 {
			if defaultLocale == "" {
				return fmt.Errorf("cms: seed menu default locale is required for item %q", item.Path)
			}
			for idx := range item.Translations {
				if strings.TrimSpace(item.Translations[idx].Locale) == "" {
					item.Translations[idx].Locale = defaultLocale
				}
			}
		}

		explicit[item.Path] = item
	}

	type seedDefinition struct {
		item  SeedMenuItem
		depth int
	}

	desired := make(map[string]seedDefinition, len(explicit))
	for path, item := range explicit {
		desired[path] = seedDefinition{item: item, depth: pathDepth(path)}
	}

	if opts.AutoCreateParents {
		if defaultLocale == "" {
			return errors.New("cms: seed menu locale is required for AutoCreateParents")
		}

		for path := range explicit {
			parts := strings.Split(path, ".")
			for i := 2; i < len(parts); i++ {
				parentPath := strings.Join(parts[:i], ".")
				if _, ok := desired[parentPath]; ok {
					continue
				}
				title := humanizePathSegment(parts[i-1])
				desired[parentPath] = seedDefinition{
					item: SeedMenuItem{
						Path: parentPath,
						Type: "group",
						Translations: []MenuItemTranslationInput{
							{
								Locale:     defaultLocale,
								GroupTitle: title,
							},
						},
					},
					depth: i,
				}
			}
		}
	} else {
		for path := range explicit {
			parsed, err := ParseMenuItemPathForMenu(menuCode, path)
			if err != nil {
				return err
			}
			if parsed.ParentPath == "" || parsed.ParentPath == menuCode {
				continue
			}
			if _, ok := desired[parsed.ParentPath]; !ok {
				return fmt.Errorf("cms: seed menu item %q references missing parent %q", parsed.Path, parsed.ParentPath)
			}
		}
	}

	seedOrder := make([]seedDefinition, 0, len(desired))
	for _, def := range desired {
		seedOrder = append(seedOrder, def)
	}
	sort.Slice(seedOrder, func(i, j int) bool {
		if seedOrder[i].depth != seedOrder[j].depth {
			return seedOrder[i].depth < seedOrder[j].depth
		}
		return seedOrder[i].item.Path < seedOrder[j].item.Path
	})

	for _, def := range seedOrder {
		item := def.item

		if strings.TrimSpace(item.Path) == "" {
			return ErrMenuItemPathRequired
		}

		if len(item.Translations) > 0 && defaultLocale == "" {
			return fmt.Errorf("cms: seed menu default locale is required for item %q", item.Path)
		}
		for idx := range item.Translations {
			if strings.TrimSpace(item.Translations[idx].Locale) == "" {
				item.Translations[idx].Locale = defaultLocale
			}
		}

		if _, err := opts.Menus.UpsertMenuItemByPath(ctx, UpsertMenuItemByPathInput{
			Path:                     item.Path,
			Position:                 item.Position,
			Type:                     item.Type,
			Target:                   item.Target,
			Icon:                     item.Icon,
			Badge:                    item.Badge,
			Permissions:              item.Permissions,
			Classes:                  item.Classes,
			Styles:                   item.Styles,
			Collapsible:              item.Collapsible,
			Collapsed:                item.Collapsed,
			Metadata:                 item.Metadata,
			Translations:             item.Translations,
			AllowMissingTranslations: item.AllowMissingTranslations,
			Actor:                    opts.Actor,
		}); err != nil {
			return err
		}
	}

	ensure := opts.Ensure || opts.PruneUnspecified
	if ensure {
		if _, err := opts.Menus.ReconcileMenuByCode(ctx, menuCode, opts.Actor); err != nil {
			return err
		}

		childrenByParent := make(map[string][]SeedMenuItem)
		for path, def := range desired {
			parsed, err := ParseMenuItemPathForMenu(menuCode, path)
			if err != nil {
				return err
			}
			childrenByParent[parsed.ParentPath] = append(childrenByParent[parsed.ParentPath], def.item)
		}

		parentPaths := make([]string, 0, len(childrenByParent))
		for parentPath := range childrenByParent {
			parentPaths = append(parentPaths, parentPath)
		}
		sort.Slice(parentPaths, func(i, j int) bool {
			di, dj := pathDepth(parentPaths[i]), pathDepth(parentPaths[j])
			if di != dj {
				return di < dj
			}
			return parentPaths[i] < parentPaths[j]
		})

		for _, parentPath := range parentPaths {
			children := childrenByParent[parentPath]
			sort.Slice(children, func(i, j int) bool {
				ipos := int(^uint(0) >> 1)
				if children[i].Position != nil {
					ipos = *children[i].Position
				}
				jpos := int(^uint(0) >> 1)
				if children[j].Position != nil {
					jpos = *children[j].Position
				}
				if ipos != jpos {
					return ipos < jpos
				}
				return children[i].Path < children[j].Path
			})

			ordered := make([]string, 0, len(children))
			for _, child := range children {
				ordered = append(ordered, child.Path)
			}

			if err := opts.Menus.SetMenuSiblingOrder(ctx, menuCode, parentPath, ordered, opts.Actor); err != nil {
				return err
			}
		}
	}

	if opts.PruneUnspecified {
		existing, err := opts.Menus.ListMenuItemsByCode(ctx, menuCode)
		if err != nil {
			return err
		}

		stale := make([]string, 0)
		for _, item := range existing {
			if item == nil || strings.TrimSpace(item.Path) == "" {
				continue
			}
			parsed, err := ParseMenuItemPathForMenu(menuCode, item.Path)
			if err != nil {
				continue
			}
			if _, ok := desired[parsed.Path]; ok {
				continue
			}
			stale = append(stale, parsed.Path)
		}

		sort.Slice(stale, func(i, j int) bool {
			di, dj := pathDepth(stale[i]), pathDepth(stale[j])
			if di != dj {
				return di > dj
			}
			return stale[i] < stale[j]
		})

		for _, path := range stale {
			if err := opts.Menus.DeleteMenuItemByPath(ctx, menuCode, path, opts.Actor, true); err != nil {
				return err
			}
		}
	}

	return nil
}

func pathDepth(path string) int {
	if strings.TrimSpace(path) == "" {
		return 0
	}
	return len(strings.Split(path, "."))
}

func humanizePathSegment(seg string) string {
	trimmed := strings.TrimSpace(seg)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "_", " ")
	trimmed = strings.ReplaceAll(trimmed, "-", " ")
	parts := strings.Fields(trimmed)
	for i, part := range parts {
		parts[i] = upperFirst(part)
	}
	return strings.Join(parts, " ")
}

func upperFirst(value string) string {
	if value == "" {
		return ""
	}
	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
