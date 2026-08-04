package cms_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestModule_Menus_UpdateMenuItemByPath_CacheCriteriaIsolation(t *testing.T) {
	t.Run("valid parent lookup does not reuse child result", func(t *testing.T) {
		ctx, module, db, fixture := newCachedMenuUpdateFixture(t)

		parentPath := fixture.parentPath
		if _, err := module.Menus().UpdateMenuItemByPath(ctx, fixture.menuCode, fixture.childPath, cms.UpdateMenuItemByPathInput{
			ParentPath: &parentPath,
			Actor:      fixture.actor,
		}); err != nil {
			t.Fatalf("update child with its existing parent: %v", err)
		}

		var stored struct {
			ID       uuid.UUID  `bun:"id"`
			ParentID *uuid.UUID `bun:"parent_id"`
		}
		if err := db.NewSelect().
			Table("menu_items").
			Column("id", "parent_id").
			Where("id = ?", fixture.childID).
			Scan(ctx, &stored); err != nil {
			t.Fatalf("reload child hierarchy: %v", err)
		}
		if stored.ID != fixture.childID {
			t.Fatalf("expected child id %s, got %s", fixture.childID, stored.ID)
		}
		if stored.ParentID == nil || *stored.ParentID != fixture.parentID {
			t.Fatalf("expected child parent %s, got %v", fixture.parentID, stored.ParentID)
		}
	})

	t.Run("self parent remains rejected", func(t *testing.T) {
		ctx, module, _, fixture := newCachedMenuUpdateFixture(t)

		parentPath := fixture.parentPath
		_, err := module.Menus().UpdateMenuItemByPath(ctx, fixture.menuCode, fixture.parentPath, cms.UpdateMenuItemByPathInput{
			ParentPath: &parentPath,
			Actor:      fixture.actor,
		})
		if !errors.Is(err, menus.ErrMenuItemCycle) {
			t.Fatalf("expected ErrMenuItemCycle for self parent, got %v", err)
		}
	})

	t.Run("descendant parent remains rejected", func(t *testing.T) {
		ctx, module, _, fixture := newCachedMenuUpdateFixture(t)

		childPath := fixture.childPath
		_, err := module.Menus().UpdateMenuItemByPath(ctx, fixture.menuCode, fixture.parentPath, cms.UpdateMenuItemByPathInput{
			ParentPath: &childPath,
			Actor:      fixture.actor,
		})
		if !errors.Is(err, menus.ErrMenuItemCycle) {
			t.Fatalf("expected ErrMenuItemCycle for descendant parent, got %v", err)
		}
	})
}

type cachedMenuUpdateFixture struct {
	menuCode   string
	parentPath string
	childPath  string
	parentID   uuid.UUID
	childID    uuid.UUID
	actor      uuid.UUID
}

func newCachedMenuUpdateFixture(t *testing.T) (context.Context, *cms.Module, *bun.DB, cachedMenuUpdateFixture) {
	t.Helper()

	ctx := context.Background()
	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	db := bun.NewDB(sqlDB, sqlitedialect.New())
	db.SetMaxOpenConns(1)
	for _, model := range []any{
		(*environments.Environment)(nil),
		(*content.Locale)(nil),
		(*menus.Menu)(nil),
		(*menus.MenuItem)(nil),
		(*menus.MenuItemTranslation)(nil),
	} {
		if _, err := db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
			t.Fatalf("create table %T: %v", model, err)
		}
	}

	cfg := cms.DefaultConfig()
	cfg.Cache.Enabled = true
	cfg.Features.AdvancedCache = true
	module, err := cms.New(cfg, di.WithBunDB(db))
	if err != nil {
		t.Fatalf("new cached cms module: %v", err)
	}

	fixture := cachedMenuUpdateFixture{
		menuCode:   "primary",
		parentPath: "primary.parent",
		childPath:  "primary.parent.child",
		parentID:   uuid.New(),
		childID:    uuid.New(),
		actor:      uuid.New(),
	}
	environmentID := uuid.MustParse(environments.DefaultID)
	menuID := uuid.New()
	menu := &menus.Menu{
		ID:            menuID,
		Code:          fixture.menuCode,
		Status:        menus.MenuStatusPublished,
		EnvironmentID: environmentID,
		CreatedBy:     fixture.actor,
		UpdatedBy:     fixture.actor,
	}
	if _, err := db.NewInsert().Model(menu).Exec(ctx); err != nil {
		t.Fatalf("insert menu: %v", err)
	}

	parent := &menus.MenuItem{
		ID:            fixture.parentID,
		MenuID:        menuID,
		ExternalCode:  fixture.parentPath,
		Position:      0,
		Type:          menus.MenuItemTypeItem,
		Target:        map[string]any{"type": "url", "url": "/parent"},
		Metadata:      map[string]any{},
		EnvironmentID: environmentID,
		CreatedBy:     fixture.actor,
		UpdatedBy:     fixture.actor,
	}
	child := &menus.MenuItem{
		ID:            fixture.childID,
		MenuID:        menuID,
		ParentID:      &fixture.parentID,
		ExternalCode:  fixture.childPath,
		Position:      0,
		Type:          menus.MenuItemTypeItem,
		Target:        map[string]any{"type": "url", "url": "/child"},
		Metadata:      map[string]any{},
		EnvironmentID: environmentID,
		CreatedBy:     fixture.actor,
		UpdatedBy:     fixture.actor,
	}
	if _, err := db.NewInsert().Model(parent).Exec(ctx); err != nil {
		t.Fatalf("insert parent menu item: %v", err)
	}
	if _, err := db.NewInsert().Model(child).Exec(ctx); err != nil {
		t.Fatalf("insert child menu item: %v", err)
	}

	var locale content.Locale
	if err := db.NewSelect().Model(&locale).Where("code = ?", "en").Scan(ctx); err != nil {
		t.Fatalf("load seeded locale: %v", err)
	}
	translations := []*menus.MenuItemTranslation{
		{ID: uuid.New(), MenuItemID: fixture.parentID, LocaleID: locale.ID, Label: "Parent"},
		{ID: uuid.New(), MenuItemID: fixture.childID, LocaleID: locale.ID, Label: "Child"},
	}
	if _, err := db.NewInsert().Model(&translations).Exec(ctx); err != nil {
		t.Fatalf("insert menu item translations: %v", err)
	}

	return ctx, module, db, fixture
}
