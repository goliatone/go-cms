package content_test

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
)

type contentContractFixture struct {
	Locales []struct {
		ID      string `json:"id"`
		Code    string `json:"code"`
		Display string `json:"display"`
	} `json:"locales"`
	ContentTypes []struct {
		ID     string         `json:"id"`
		Name   string         `json:"name"`
		Slug   string         `json:"slug"`
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
	Expectation struct {
		Slug    string            `json:"slug"`
		Status  string            `json:"status"`
		Titles  map[string]string `json:"titles"`
		Locales []string          `json:"locales"`
	} `json:"expectation"`
}

type contentExpectationView struct {
	Slug    string
	Status  string
	Locales []string
	Titles  map[string]string
}

func TestContentServiceContract_Phase1Fixture(t *testing.T) {
	fixture := loadContentContractFixture(t, "testdata/phase1_contract.json")

	contentRepo := content.NewMemoryContentRepository()
	typeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeIndex := make(map[uuid.UUID]string, len(fixture.Locales))
	for _, loc := range fixture.Locales {
		id := mustParseUUID(t, loc.ID)
		localeRepo.Put(&content.Locale{
			ID:      id,
			Code:    loc.Code,
			Display: loc.Display,
		})
		localeIndex[id] = loc.Code
	}

	for _, ct := range fixture.ContentTypes {
		slug := ct.Slug
		if slug == "" {
			slug = ct.Name
		}
		if err := typeRepo.Put(&content.ContentType{
			ID:     mustParseUUID(t, ct.ID),
			Name:   ct.Name,
			Slug:   slug,
			Schema: ct.Schema,
		}); err != nil {
			t.Fatalf("seed content type: %v", err)
		}
	}

	svc := content.NewService(contentRepo, typeRepo, localeRepo, content.WithClock(func() time.Time {
		return time.Unix(0, 0)
	}))

	req := content.CreateContentRequest{
		ContentTypeID: mustParseUUID(t, fixture.Request.ContentTypeID),
		Slug:          fixture.Request.Slug,
		Status:        fixture.Request.Status,
		CreatedBy:     mustParseUUID(t, fixture.Request.CreatedBy),
		UpdatedBy:     mustParseUUID(t, fixture.Request.UpdatedBy),
	}

	for _, tr := range fixture.Request.Translations {
		summary := tr.Summary
		req.Translations = append(req.Translations, content.ContentTranslationInput{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: &summary,
			Content: tr.Content,
		})
	}

	result, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got := projectContent(result, localeIndex)
	want := contentExpectationView{
		Slug:    fixture.Expectation.Slug,
		Status:  fixture.Expectation.Status,
		Locales: append([]string{}, fixture.Expectation.Locales...),
		Titles:  fixture.Expectation.Titles,
	}
	sort.Strings(want.Locales)

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("contract mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func loadContentContractFixture(t *testing.T, path string) contentContractFixture {
	t.Helper()

	raw, err := testsupport.LoadFixture(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	var fx contentContractFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return fx
}

func projectContent(contentRecord *content.Content, localeIndex map[uuid.UUID]string) contentExpectationView {
	view := contentExpectationView{
		Slug:   contentRecord.Slug,
		Status: contentRecord.Status,
		Titles: make(map[string]string, len(contentRecord.Translations)),
	}

	seen := map[string]struct{}{}
	for _, tr := range contentRecord.Translations {
		code := localeIndex[tr.LocaleID]
		if code == "" {
			continue
		}
		view.Titles[code] = tr.Title
		if _, ok := seen[code]; !ok {
			view.Locales = append(view.Locales, code)
			seen[code] = struct{}{}
		}
	}

	sort.Strings(view.Locales)
	return view
}

func mustParseUUID(t *testing.T, value string) uuid.UUID {
	t.Helper()
	parsed, err := uuid.Parse(value)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", value, err)
	}
	return parsed
}
