package themes

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
)

type stubTemplateRepo struct {
	template *Template
	err      error
}

func (s stubTemplateRepo) Create(ctx context.Context, template *Template) (*Template, error) {
	return nil, nil
}
func (s stubTemplateRepo) Update(ctx context.Context, template *Template) (*Template, error) {
	return nil, nil
}
func (s stubTemplateRepo) GetByID(ctx context.Context, id uuid.UUID) (*Template, error) {
	return nil, nil
}
func (s stubTemplateRepo) GetBySlug(ctx context.Context, themeID uuid.UUID, slug string) (*Template, error) {
	return s.template, s.err
}
func (s stubTemplateRepo) ListByTheme(ctx context.Context, themeID uuid.UUID) ([]*Template, error) {
	return nil, nil
}
func (s stubTemplateRepo) ListAll(ctx context.Context) ([]*Template, error) { return nil, nil }

func TestValidateRegisterTemplate_UniqueSlug(t *testing.T) {
	ctx := context.Background()
	repo := stubTemplateRepo{err: &NotFoundError{}}
	input := RegisterTemplateInput{
		ThemeID:      uuid.New(),
		Name:         "Landing",
		Slug:         "landing",
		TemplatePath: "templates/landing.html.tmpl",
		Regions: map[string]TemplateRegion{
			"hero": {Name: "Hero", AcceptsBlocks: true},
		},
	}

	if err := ValidateRegisterTemplate(ctx, repo, input); err != nil {
		t.Fatalf("validate register: %v", err)
	}

	repo.template = &Template{}
	repo.err = nil
	if err := ValidateRegisterTemplate(ctx, repo, input); err != ErrTemplateSlugConflict {
		t.Fatalf("expected slug conflict, got %v", err)
	}
}

func TestParseManifest(t *testing.T) {
	payload := []byte(`{
		"name": "aurora",
		"version": "1.0.0",
		"widget_areas": [{"code":"header","name":"Header"}],
		"menu_locations": [{"code":"primary","name":"Primary"}],
		"assets": {"styles":["css/site.css"]}
	}`)

	manifest, err := ParseManifest(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	input, err := ManifestToThemeInput("themes/aurora", manifest)
	if err != nil {
		t.Fatalf("manifest to input: %v", err)
	}
	if input.Name != "aurora" || input.Version != "1.0.0" {
		t.Fatalf("unexpected input %+v", input)
	}
	if len(input.Config.WidgetAreas) != 1 || input.Config.WidgetAreas[0].Code != "header" {
		t.Fatalf("widget areas not copied")
	}
	if input.Config.Assets == nil || len(input.Config.Assets.Styles) != 1 {
		t.Fatalf("assets not copied")
	}
}
