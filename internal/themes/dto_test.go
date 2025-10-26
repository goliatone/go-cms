package themes

import (
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
func (s stubTemplateRepo) Delete(ctx context.Context, id uuid.UUID) error   { return nil }

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
