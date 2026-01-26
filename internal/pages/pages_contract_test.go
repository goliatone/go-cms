package pages_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
)

type phase1ContentFixture struct {
	Locales []struct {
		ID      string `json:"id"`
		Code    string `json:"code"`
		Display string `json:"display"`
	} `json:"locales"`
	ContentTypes []struct {
		ID     string         `json:"id"`
		Name   string         `json:"name"`
		Schema map[string]any `json:"schema"`
	} `json:"content_types"`
	Request struct {
		ContentTypeID string `json:"content_type_id"`
		Slug          string `json:"slug"`
		Status        string `json:"status"`
		CreatedBy     string `json:"created_by"`
		UpdatedBy     string `json:"updated_by"`
		Translations  []struct {
			Locale  string         `json:"locale"`
			Title   string         `json:"title"`
			Summary string         `json:"summary"`
			Content map[string]any `json:"content"`
		} `json:"translations"`
	} `json:"request"`
}

type pageContractFixture struct {
	TemplateID   string `json:"template_id"`
	Slug         string `json:"slug"`
	Status       string `json:"status"`
	CreatedBy    string `json:"created_by"`
	UpdatedBy    string `json:"updated_by"`
	Translations []struct {
		Locale string `json:"locale"`
		Title  string `json:"title"`
		Path   string `json:"path"`
	} `json:"translations"`
	Expectation struct {
		Slug   string            `json:"slug"`
		Status string            `json:"status"`
		Paths  map[string]string `json:"paths"`
	} `json:"expectation"`
}

type pageExpectationView struct {
	Slug   string
	Status string
	Paths  map[string]string
}

func TestPageServiceContract_Phase1Fixture(t *testing.T) {
	contentFx := loadPhase1ContentFixture(t, "../content/testdata/phase1_contract.json")
	pageFx := loadPageContractFixture(t, "testdata/phase1_contract.json")

	contentRepo := content.NewMemoryContentRepository()
	contentTypeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()
	pageRepo := pages.NewMemoryPageRepository()

	localeIndex := make(map[uuid.UUID]string, len(contentFx.Locales))
	for _, loc := range contentFx.Locales {
		id := mustParseUUID(t, loc.ID)
		localeRepo.Put(&content.Locale{
			ID:      id,
			Code:    loc.Code,
			Display: loc.Display,
		})
		localeIndex[id] = loc.Code
	}

	for _, ct := range contentFx.ContentTypes {
		if err := contentTypeRepo.Put(&content.ContentType{
			ID:     mustParseUUID(t, ct.ID),
			Name:   ct.Name,
			Slug:   ct.Name,
			Schema: ct.Schema,
		}); err != nil {
			t.Fatalf("seed content type: %v", err)
		}
	}

	contentSvc := content.NewService(contentRepo, contentTypeRepo, localeRepo, content.WithClock(func() time.Time {
		return time.Unix(0, 0)
	}))

	contentReq := buildContentCreateRequest(t, contentFx)

	createdContent, err := contentSvc.Create(context.Background(), contentReq)
	if err != nil {
		t.Fatalf("seed content: %v", err)
	}

	pageSvc := pages.NewService(pageRepo, contentRepo, localeRepo, pages.WithPageClock(func() time.Time {
		return time.Unix(0, 0)
	}))

	pageReq := buildPageCreateRequest(t, pageFx)
	pageReq.ContentID = createdContent.ID

	pageRecord, err := pageSvc.Create(context.Background(), pageReq)
	if err != nil {
		t.Fatalf("Create page: %v", err)
	}

	got := projectPage(pageRecord, localeIndex)
	want := pageExpectationView{
		Slug:   pageFx.Expectation.Slug,
		Status: pageFx.Expectation.Status,
		Paths:  pageFx.Expectation.Paths,
	}

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("contract mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func loadPhase1ContentFixture(t *testing.T, path string) phase1ContentFixture {
	t.Helper()
	raw, err := testsupport.LoadFixture(path)
	if err != nil {
		t.Fatalf("load content fixture: %v", err)
	}
	var fx phase1ContentFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("unmarshal content fixture: %v", err)
	}
	return fx
}

func loadPageContractFixture(t *testing.T, path string) pageContractFixture {
	t.Helper()
	raw, err := testsupport.LoadFixture(path)
	if err != nil {
		t.Fatalf("load page fixture: %v", err)
	}
	var fx pageContractFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("unmarshal page fixture: %v", err)
	}
	return fx
}

func buildContentCreateRequest(t *testing.T, fx phase1ContentFixture) content.CreateContentRequest {
	req := content.CreateContentRequest{
		ContentTypeID: mustParseUUID(t, fx.Request.ContentTypeID),
		Slug:          fx.Request.Slug,
		Status:        fx.Request.Status,
		CreatedBy:     mustParseUUID(t, fx.Request.CreatedBy),
		UpdatedBy:     mustParseUUID(t, fx.Request.UpdatedBy),
	}
	for _, tr := range fx.Request.Translations {
		summary := tr.Summary
		req.Translations = append(req.Translations, content.ContentTranslationInput{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: &summary,
			Content: tr.Content,
		})
	}
	return req
}

func buildPageCreateRequest(t *testing.T, fx pageContractFixture) pages.CreatePageRequest {
	req := pages.CreatePageRequest{
		TemplateID: mustParseUUID(t, fx.TemplateID),
		Slug:       fx.Slug,
		Status:     fx.Status,
		CreatedBy:  mustParseUUID(t, fx.CreatedBy),
		UpdatedBy:  mustParseUUID(t, fx.UpdatedBy),
	}
	for _, tr := range fx.Translations {
		req.Translations = append(req.Translations, pages.PageTranslationInput{
			Locale: tr.Locale,
			Title:  tr.Title,
			Path:   tr.Path,
		})
	}
	return req
}

func projectPage(pageRecord *pages.Page, localeIndex map[uuid.UUID]string) pageExpectationView {
	view := pageExpectationView{
		Slug:   pageRecord.Slug,
		Status: pageRecord.Status,
		Paths:  make(map[string]string, len(pageRecord.Translations)),
	}

	for _, tr := range pageRecord.Translations {
		code := localeIndex[tr.LocaleID]
		if code == "" {
			continue
		}
		view.Paths[code] = tr.Path
	}

	return view
}

func mustParseUUID(t *testing.T, value string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(value)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", value, err)
	}
	return id
}
