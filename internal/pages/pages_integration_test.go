package pages_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/google/uuid"
)

func TestPagesIntegration_CreateAndFetchPage(t *testing.T) {
	contentFx := loadPhase1ContentFixture(t, "../content/testdata/phase1_contract.json")
	pageFx := loadPageContractFixture(t, "testdata/phase1_contract.json")

	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en", "es"}
	cfg.Cache.Enabled = false

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	typeRepo := container.ContentTypeRepository()
	for _, ct := range contentFx.ContentTypes {
		maybePutContentType(typeRepo, &content.ContentType{
			ID:     mustParseUUID(t, ct.ID),
			Name:   ct.Name,
			Slug:   ct.Name,
			Schema: ct.Schema,
		})
	}

	ctx := context.Background()

	contentSvc := container.ContentService()
	contentReq := buildContentCreateRequest(t, contentFx)
	contentRecord, err := contentSvc.Create(ctx, contentReq)
	if err != nil {
		t.Fatalf("create content via container: %v", err)
	}

	pageSvc := container.PageService()
	pageReq := buildPageCreateRequest(t, pageFx)
	pageReq.ContentID = contentRecord.ID

	pageRecord, err := pageSvc.Create(ctx, pageReq)
	if err != nil {
		t.Fatalf("create page via container: %v", err)
	}

	retrieved, err := pageSvc.Get(ctx, pageRecord.ID)
	if err != nil {
		t.Fatalf("fetch created page: %v", err)
	}

	if retrieved.ID != pageRecord.ID {
		t.Fatalf("expected retrieved ID %s got %s", pageRecord.ID, retrieved.ID)
	}

	list, err := pageSvc.List(ctx)
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 page in list got %d", len(list))
	}

	localeRepo := container.LocaleRepository()
	localeIndex := map[uuid.UUID]string{}
	for _, code := range cfg.I18N.Locales {
		loc, err := localeRepo.GetByCode(ctx, code)
		if err != nil {
			t.Fatalf("resolve locale %q: %v", code, err)
		}
		localeIndex[loc.ID] = code
	}

	got := projectPage(list[0], localeIndex)
	want := pageExpectationView{
		Slug:   pageFx.Expectation.Slug,
		Status: pageFx.Expectation.Status,
		Paths:  pageFx.Expectation.Paths,
	}

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("integration mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func maybePutContentType(repo content.ContentTypeRepository, ct *content.ContentType) {
	if seeder, ok := repo.(interface {
		Put(*content.ContentType) error
	}); ok {
		if err := seeder.Put(ct); err != nil {
			panic(err)
		}
	}
}
