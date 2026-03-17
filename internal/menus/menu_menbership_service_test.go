package menus_test

import (
	"context"
	"maps"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/google/uuid"
)

type phase3MenuFixture struct {
	menuSvc      menus.Service
	contentSvc   content.Service
	contentTypes *content.MemoryContentTypeRepository
	contentType  *content.ContentType
	actor        uuid.UUID
}

func TestService_MenuByLocation_DefaultContentContributions(t *testing.T) {
	fixture := newPhase3MenuFixture(t, navigationCapabilities(true, content.NavigationMergeAppend))
	fixture.createMenuWithBinding(t, "site-main", "site.main", "Manual", "/manual")

	fixture.createContent(t, "alpha", "Alpha", "/alpha", map[string]any{"sort_order": 1})
	fixture.createContent(t, "beta", "Beta", "/beta", map[string]any{"sort_order": 2})

	resolved, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.main", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve menu by location: %v", err)
	}
	assertLabels(t, resolved.Items, []string{"Manual", "Alpha", "Beta"})
	if len(resolved.ContentMembership) != 2 {
		t.Fatalf("expected 2 content memberships, got %d", len(resolved.ContentMembership))
	}
	for _, membership := range resolved.ContentMembership {
		if membership.Origin != content.NavigationOriginDefault {
			t.Fatalf("expected default origin, got %q", membership.Origin)
		}
		if membership.Location != "site.main" {
			t.Fatalf("expected site.main location, got %q", membership.Location)
		}
	}
	alpha := findNodeByLabel(resolved.Items, "Alpha")
	if alpha == nil || !alpha.Contribution || alpha.ContributionOrigin != content.NavigationOriginDefault {
		t.Fatalf("expected Alpha contribution with default origin, got %#v", alpha)
	}
}

func TestService_MenuByLocation_PerEntryOverridesAcrossLocations(t *testing.T) {
	fixture := newPhase3MenuFixture(t, navigationCapabilities(true, content.NavigationMergeAppend))
	fixture.createMenuWithBinding(t, "site-main", "site.main", "Main", "/main")
	fixture.createMenuWithBinding(t, "site-footer", "site.footer", "Footer", "/footer")

	fixture.createContent(t, "home", "Home", "/home", nil)
	fixture.createContent(t, "legal", "Legal", "/legal", map[string]any{
		"_navigation": map[string]any{
			"site.main":   "hide",
			"site.footer": "show",
		},
	})

	mainResolved, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.main", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve site.main: %v", err)
	}
	assertLabels(t, mainResolved.Items, []string{"Main", "Home"})
	if node := findNodeByLabel(mainResolved.Items, "Legal"); node != nil {
		t.Fatalf("did not expect Legal in site.main, got %#v", node)
	}

	footerResolved, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.footer", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve site.footer: %v", err)
	}
	assertLabels(t, footerResolved.Items, []string{"Footer", "Legal"})
	legal := findNodeByLabel(footerResolved.Items, "Legal")
	if legal == nil || !legal.Contribution || legal.ContributionOrigin != content.NavigationOriginOverride {
		t.Fatalf("expected Legal override contribution in footer, got %#v", legal)
	}
}

func TestService_MenuByLocation_PerEntryOverridesNormalizeLocationCase(t *testing.T) {
	fixture := newPhase3MenuFixture(t, navigationCapabilities(true, content.NavigationMergeAppend))
	fixture.createMenuWithBinding(t, "site-main", "site.main", "Main", "/main")

	fixture.createContent(t, "home", "Home", "/home", map[string]any{
		"_navigation": map[string]any{
			"SITE.MAIN": "hide",
		},
	})

	mainResolved, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.main", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve site.main: %v", err)
	}
	assertLabels(t, mainResolved.Items, []string{"Main"})
}

func TestService_MenuByLocation_DuplicatePolicies(t *testing.T) {
	fixture := newPhase3MenuFixture(t, navigationCapabilities(true, content.NavigationMergeAppend))
	fixture.createMenuWithBinding(t, "site-main", "site.main", "Manual", "/same")
	fixture.createContent(t, "same", "Same", "/same", nil)

	byURL, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.main", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve by_url policy: %v", err)
	}
	assertLabels(t, byURL.Items, []string{"Manual"})

	policyNone := menus.MenuContributionDuplicateNone
	none, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.main", "en", menus.MenuQueryOptions{
		ContributionDuplicatePolicy: policyNone,
	})
	if err != nil {
		t.Fatalf("resolve none policy: %v", err)
	}
	assertLabels(t, none.Items, []string{"Manual", "Same"})

	byTargetPolicy := menus.MenuContributionDuplicateByTarget
	byTarget, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.main", "en", menus.MenuQueryOptions{
		ContributionDuplicatePolicy: byTargetPolicy,
	})
	if err != nil {
		t.Fatalf("resolve by_target policy: %v", err)
	}
	assertLabels(t, byTarget.Items, []string{"Manual", "Same"})
}

func TestService_MenuByLocation_MergeModes(t *testing.T) {
	fixture := newPhase3MenuFixture(t, navigationCapabilities(true, content.NavigationMergeAppend))
	fixture.createMenuWithBinding(t, "site-main", "site.main", "Manual", "/manual")
	fixture.createContent(t, "contrib", "Contrib", "/contrib", nil)

	appendResolved, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.main", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve append merge mode: %v", err)
	}
	assertLabels(t, appendResolved.Items, []string{"Manual", "Contrib"})

	fixture.setMergeMode(t, content.NavigationMergePrepend)
	prependResolved, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.main", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve prepend merge mode: %v", err)
	}
	assertLabels(t, prependResolved.Items, []string{"Contrib", "Manual"})

	fixture.setMergeMode(t, content.NavigationMergeReplace)
	replaceResolved, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.main", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve replace merge mode: %v", err)
	}
	assertLabels(t, replaceResolved.Items, []string{"Contrib"})
}

func newPhase3MenuFixture(t *testing.T, navigation map[string]any) *phase3MenuFixture {
	t.Helper()

	locale := content.Locale{
		ID:        uuid.New(),
		Code:      "en",
		Display:   "English",
		IsActive:  true,
		IsDefault: true,
	}

	contentRepo := content.NewMemoryContentRepository()
	contentTypeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()
	localeRepo.Put(&locale)

	actor := uuid.New()
	contentType := &content.ContentType{
		ID:   uuid.New(),
		Name: "page",
		Slug: "page",
		Schema: map[string]any{
			"fields": []map[string]any{{"name": "body", "type": "richtext"}},
		},
		Capabilities: map[string]any{
			"navigation": navigation,
		},
	}
	if err := contentTypeRepo.Put(contentType); err != nil {
		t.Fatalf("seed content type: %v", err)
	}

	contentSvc := content.NewService(contentRepo, contentTypeRepo, localeRepo)
	menuSvc := newServiceWithLocales(
		t,
		[]content.Locale{locale},
		func(menus.AddMenuItemInput) uuid.UUID { return uuid.New() },
		nil,
		menus.WithContentRepository(contentRepo),
		menus.WithContentTypeRepository(contentTypeRepo),
	)

	return &phase3MenuFixture{
		menuSvc:      menuSvc,
		contentSvc:   contentSvc,
		contentTypes: contentTypeRepo,
		contentType:  contentType,
		actor:        actor,
	}
}

func (f *phase3MenuFixture) createMenuWithBinding(t *testing.T, code, location, manualLabel, manualURL string) {
	t.Helper()
	ctx := context.Background()

	menu, err := f.menuSvc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      code,
		Location:  location,
		Status:    menus.MenuStatusPublished,
		CreatedBy: f.actor,
		UpdatedBy: f.actor,
	})
	if err != nil {
		t.Fatalf("create menu %s: %v", code, err)
	}
	_, err = f.menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		Position:     0,
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "external", "url": manualURL},
		Translations: []menus.MenuItemTranslationInput{{Locale: "en", Label: manualLabel}},
		CreatedBy:    f.actor,
		UpdatedBy:    f.actor,
	})
	if err != nil {
		t.Fatalf("add manual menu item: %v", err)
	}
	_, err = f.menuSvc.UpsertMenuLocationBinding(ctx, menus.UpsertMenuLocationBindingInput{
		Location: location,
		MenuCode: code,
		Priority: 10,
		Status:   menus.MenuStatusPublished,
		Actor:    f.actor,
	})
	if err != nil {
		t.Fatalf("upsert location binding: %v", err)
	}
}

func (f *phase3MenuFixture) createContent(t *testing.T, slug, title, path string, metadata map[string]any) *content.Content {
	t.Helper()
	payload := map[string]any{"body": "content"}
	entryMetadata := map[string]any{"path": path}
	maps.Copy(entryMetadata, metadata)

	record, err := f.contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: f.contentType.ID,
		Slug:          slug,
		Status:        menus.MenuStatusPublished,
		CreatedBy:     f.actor,
		UpdatedBy:     f.actor,
		Metadata:      entryMetadata,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   title,
				Content: payload,
			},
		},
	})
	if err != nil {
		t.Fatalf("create content %s: %v", slug, err)
	}
	return record
}

func (f *phase3MenuFixture) setMergeMode(t *testing.T, mode string) {
	t.Helper()
	updated := *f.contentType
	updated.Capabilities = map[string]any{
		"navigation": navigationCapabilities(true, mode),
	}
	if err := f.contentTypes.Put(&updated); err != nil {
		t.Fatalf("update content type merge mode: %v", err)
	}
	f.contentType = &updated
}

func navigationCapabilities(allowOverride bool, mergeMode string) map[string]any {
	return map[string]any{
		"enabled":                 true,
		"eligible_locations":      []any{"site.main", "site.footer"},
		"default_locations":       []any{"site.main"},
		"default_visible":         true,
		"allow_instance_override": allowOverride,
		"label_field":             "title",
		"url_field":               "path",
		"merge_mode":              mergeMode,
	}
}

func assertLabels(t *testing.T, nodes []menus.NavigationNode, expected []string) {
	t.Helper()
	if len(nodes) != len(expected) {
		t.Fatalf("expected %d nodes got %d (%#v)", len(expected), len(nodes), nodes)
	}
	for i := range expected {
		if nodes[i].Label != expected[i] {
			t.Fatalf("expected node[%d] label %q got %q", i, expected[i], nodes[i].Label)
		}
	}
}

func findNodeByLabel(nodes []menus.NavigationNode, label string) *menus.NavigationNode {
	for i := range nodes {
		if nodes[i].Label == label {
			return &nodes[i]
		}
	}
	return nil
}
