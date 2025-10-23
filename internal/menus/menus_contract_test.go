package menus_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
)

type menuContractFixture struct {
	Locales []struct {
		ID      string `json:"id"`
		Code    string `json:"code"`
		Display string `json:"display"`
	} `json:"locales"`
	Menu struct {
		ID          string            `json:"id"`
		Code        string            `json:"code"`
		Description string            `json:"description"`
		CreatedBy   string            `json:"created_by"`
		UpdatedBy   string            `json:"updated_by"`
		Items       []menuItemFixture `json:"items"`
	} `json:"menu"`
	Expectation struct {
		Code      string   `json:"code"`
		Locales   []string `json:"locales"`
		RootItems []string `json:"root_items"`
	} `json:"expectation"`
}

type menuItemFixture struct {
	ID           string                 `json:"id"`
	ParentID     *string                `json:"parent_id"`
	Position     int                    `json:"position"`
	Target       map[string]any         `json:"target"`
	CreatedBy    string                 `json:"created_by"`
	UpdatedBy    string                 `json:"updated_by"`
	Translations []menuTranslationEntry `json:"translations"`
	Children     []menuItemFixture      `json:"children,omitempty"`
}

type menuTranslationEntry struct {
	Locale string `json:"locale"`
	Label  string `json:"label"`
}

type itemProjection struct {
	ParentID *string
}

func TestMenuContractFixture_Structure(t *testing.T) {
	fx := loadMenuContractFixture(t, "testdata/phase3_contract.json")

	if fx.Menu.Code == "" {
		t.Fatalf("menu code must be set")
	}
	if _, err := uuid.Parse(fx.Menu.ID); err != nil {
		t.Fatalf("menu id must be valid uuid: %s", fx.Menu.ID)
	}
	if _, err := uuid.Parse(fx.Menu.CreatedBy); err != nil {
		t.Fatalf("menu created_by must be valid uuid: %s", fx.Menu.CreatedBy)
	}
	if _, err := uuid.Parse(fx.Menu.UpdatedBy); err != nil {
		t.Fatalf("menu updated_by must be valid uuid: %s", fx.Menu.UpdatedBy)
	}

	localeCodes := make(map[string]struct{}, len(fx.Locales))
	for _, locale := range fx.Locales {
		if locale.Code == "" {
			t.Fatalf("locale code must be set: %+v", locale)
		}
		if _, seen := localeCodes[locale.Code]; seen {
			t.Fatalf("duplicate locale code detected: %s", locale.Code)
		}
		if _, err := uuid.Parse(locale.ID); err != nil {
			t.Fatalf("locale id must be valid uuid: %s", locale.ID)
		}
		localeCodes[locale.Code] = struct{}{}
	}

	if len(localeCodes) == 0 {
		t.Fatalf("expected at least one locale in fixture")
	}

	expectLocales := slices.Clone(fx.Expectation.Locales)
	slices.Sort(expectLocales)

	actualLocales := make([]string, 0, len(localeCodes))
	for code := range localeCodes {
		actualLocales = append(actualLocales, code)
	}
	slices.Sort(actualLocales)
	if !slices.Equal(expectLocales, actualLocales) {
		t.Fatalf("expectation locales mismatch\nwant: %v\ngot:  %v", expectLocales, actualLocales)
	}

	itemIndex := make(map[string]itemProjection)
	rootIDs := make([]string, 0)
	walkMenuItems(fx.Menu.Items, nil, func(item menuItemFixture, parent *menuItemFixture) {
		if _, exists := itemIndex[item.ID]; exists {
			t.Fatalf("duplicate menu item id detected: %s", item.ID)
		}

		if _, err := uuid.Parse(item.ID); err != nil {
			t.Fatalf("menu item id must be valid uuid: %s", item.ID)
		}

		if parent == nil {
			if item.ParentID != nil {
				t.Fatalf("root item %s should not carry parent_id", item.ID)
			}
			rootIDs = append(rootIDs, item.ID)
		} else {
			if item.ParentID == nil || *item.ParentID != parent.ID {
				t.Fatalf("child item %s must declare parent_id %s", item.ID, parent.ID)
			}
			if _, err := uuid.Parse(*item.ParentID); err != nil {
				t.Fatalf("child item parent_id must be valid uuid: %s", *item.ParentID)
			}
		}

		if item.Target == nil {
			t.Fatalf("menu item %s missing target", item.ID)
		}
		if _, ok := item.Target["type"]; !ok {
			t.Fatalf("menu item %s target missing type", item.ID)
		}

		if len(item.Translations) == 0 {
			t.Fatalf("menu item %s should have translations", item.ID)
		}
		for _, tr := range item.Translations {
			if _, ok := localeCodes[tr.Locale]; !ok {
				t.Fatalf("menu item %s translation references unknown locale %q", item.ID, tr.Locale)
			}
			if tr.Label == "" {
				t.Fatalf("menu item %s translation for %s missing label", item.ID, tr.Locale)
			}
		}

		itemIndex[item.ID] = itemProjection{
			ParentID: item.ParentID,
		}
	})

	if len(rootIDs) == 0 {
		t.Fatalf("expected root items in fixture")
	}

	if fx.Expectation.Code != fx.Menu.Code {
		t.Fatalf("expectation code %q does not match menu code %q", fx.Expectation.Code, fx.Menu.Code)
	}

	expectRoot := slices.Clone(fx.Expectation.RootItems)
	slices.Sort(expectRoot)
	gotRoot := slices.Clone(rootIDs)
	slices.Sort(gotRoot)
	if !slices.Equal(expectRoot, gotRoot) {
		t.Fatalf("root item mismatch\nwant: %v\ngot:  %v", expectRoot, gotRoot)
	}
}

func loadMenuContractFixture(t *testing.T, path string) menuContractFixture {
	t.Helper()
	raw, err := testsupport.LoadFixture(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	var fx menuContractFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("unmarshal menu fixture: %v", err)
	}
	return fx
}

func walkMenuItems(items []menuItemFixture, parent *menuItemFixture, fn func(menuItemFixture, *menuItemFixture)) {
	for i := range items {
		item := items[i]
		fn(item, parent)
		if len(item.Children) > 0 {
			walkMenuItems(item.Children, &item, fn)
		}
	}
}
