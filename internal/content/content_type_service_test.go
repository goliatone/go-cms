package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/pkg/activity"
	"github.com/google/uuid"
)

type staticNormalizer struct {
	value string
	err   error
}

func (n staticNormalizer) Normalize(string) (string, error) {
	return n.value, n.err
}

func TestContentTypeServiceCreateDerivesSlugFromName(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	ct, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Landing Page",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	if ct.Slug != "landing-page" {
		t.Fatalf("expected slug landing-page, got %q", ct.Slug)
	}
}

func TestContentTypeServiceCreateUsesSchemaSlug(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	ct, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Article",
		Schema: map[string]any{"metadata": map[string]any{"slug": "Story"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	if ct.Slug != "story" {
		t.Fatalf("expected schema slug story, got %q", ct.Slug)
	}
}

func TestContentTypeServiceCreateNormalizesSlugRules(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	ct, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Blog_Post Draft",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	if ct.Slug != "blog-post-draft" {
		t.Fatalf("expected normalized slug blog-post-draft, got %q", ct.Slug)
	}
}

func TestContentTypeServiceCreateRejectsDuplicateSlug(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	_, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Article",
		Slug:   "article",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	_, err = svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Article Two",
		Slug:   "article",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if !errors.Is(err, content.ErrContentTypeSlugExists) {
		t.Fatalf("expected ErrContentTypeSlugExists, got %v", err)
	}
}

func TestContentTypeServiceCreateDefaultsStatusAndSchemaVersion(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	ct, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Landing Page",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	if ct.Status != content.ContentTypeStatusDraft {
		t.Fatalf("expected status draft, got %q", ct.Status)
	}
	if ct.SchemaVersion != "landing-page@v1.0.0" {
		t.Fatalf("expected schema_version landing-page@v1.0.0, got %q", ct.SchemaVersion)
	}
}

func TestContentTypeServiceGetBySlugNormalizesInput(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	created, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Landing Page",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	got, err := svc.GetBySlug(context.Background(), "Landing Page")
	if err != nil {
		t.Fatalf("get by slug: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected slug lookup to return created content type")
	}
}

func TestContentTypeServiceUpdateRejectsSlugConflict(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	first, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Article",
		Slug:   "article",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	second, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "News",
		Slug:   "news",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	_, err = svc.Update(context.Background(), content.UpdateContentTypeRequest{
		ID:   second.ID,
		Slug: &first.Slug,
	})
	if !errors.Is(err, content.ErrContentTypeSlugExists) {
		t.Fatalf("expected ErrContentTypeSlugExists, got %v", err)
	}
}

func TestContentTypeServiceUpdateRejectsInvalidStatusTransition(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	created, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Article",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	active := string(content.ContentTypeStatusActive)
	if _, err := svc.Update(context.Background(), content.UpdateContentTypeRequest{
		ID:        created.ID,
		Status:    &active,
		UpdatedBy: uuid.New(),
	}); err != nil {
		t.Fatalf("update status to active: %v", err)
	}

	draft := string(content.ContentTypeStatusDraft)
	if _, err := svc.Update(context.Background(), content.UpdateContentTypeRequest{
		ID:        created.ID,
		Status:    &draft,
		UpdatedBy: uuid.New(),
	}); !errors.Is(err, content.ErrContentTypeStatusChange) {
		t.Fatalf("expected ErrContentTypeStatusChange, got %v", err)
	}
}

func TestContentTypeServiceUpdateEmitsPublishActivity(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	hook := &activity.CaptureHook{}
	emitter := activity.NewEmitter(activity.Hooks{hook}, activity.Config{Enabled: true, Channel: "cms"})
	svc := content.NewContentTypeService(repo, content.WithContentTypeActivityEmitter(emitter))

	actor := uuid.New()
	created, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:      "News",
		Schema:    map[string]any{"fields": []any{"title"}},
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	active := string(content.ContentTypeStatusActive)
	if _, err := svc.Update(context.Background(), content.UpdateContentTypeRequest{
		ID:        created.ID,
		Status:    &active,
		UpdatedBy: actor,
	}); err != nil {
		t.Fatalf("update content type: %v", err)
	}

	if len(hook.Events) != 2 {
		t.Fatalf("expected 2 activity events, got %d", len(hook.Events))
	}
	event := hook.Events[1]
	if event.Verb != "publish" {
		t.Fatalf("expected verb publish, got %q", event.Verb)
	}
	if event.ObjectType != "content_type" || event.ObjectID != created.ID.String() {
		t.Fatalf("unexpected object data: %s %s", event.ObjectType, event.ObjectID)
	}
	if event.Channel != "cms" {
		t.Fatalf("expected channel cms, got %q", event.Channel)
	}
	if status, ok := event.Metadata["status"].(string); !ok || status != content.ContentTypeStatusActive {
		t.Fatalf("expected status metadata active, got %v", event.Metadata["status"])
	}
}

func TestContentTypeServiceSoftDeleteAllowsSlugReuse(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	first, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Feature",
		Slug:   "feature",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	if err := svc.Delete(context.Background(), content.DeleteContentTypeRequest{
		ID:         first.ID,
		DeletedBy:  uuid.New(),
		HardDelete: false,
	}); err != nil {
		t.Fatalf("soft delete content type: %v", err)
	}

	if _, err := svc.Get(context.Background(), first.ID); err == nil {
		t.Fatalf("expected deleted content type to be hidden")
	}

	if _, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Feature Reloaded",
		Slug:   "feature",
		Schema: map[string]any{"fields": []any{"title"}},
	}); err != nil {
		t.Fatalf("expected slug reuse after soft delete, got %v", err)
	}
}

func TestContentTypeServiceCreateValidationHook(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	sentinel := errors.New("validator boom")
	called := false
	svc := content.NewContentTypeService(repo, content.WithContentTypeValidators(func(_ context.Context, _ *content.ContentType) error {
		called = true
		return sentinel
	}))

	_, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Article",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if !called {
		t.Fatalf("expected validator to be called")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected validator error, got %v", err)
	}
}

func TestContentTypeServiceCreateRejectsInvalidSlug(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo, content.WithContentTypeSlugNormalizer(staticNormalizer{value: "bad@slug"}))

	_, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Article",
		Slug:   "Article",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if !errors.Is(err, content.ErrContentTypeSlugInvalid) {
		t.Fatalf("expected ErrContentTypeSlugInvalid, got %v", err)
	}
}

func TestContentTypeServiceCreateRejectsInvalidSchema(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	_, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Broken",
		Schema: map[string]any{"type": "object", "properties": 12},
	})
	if !errors.Is(err, content.ErrContentTypeSchemaInvalid) {
		t.Fatalf("expected ErrContentTypeSchemaInvalid, got %v", err)
	}
}

func TestContentTypeServiceUpdateBumpsVersionForMinorChange(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	created, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name: "Landing Page",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{"type": "string"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	if len(created.SchemaHistory) != 1 {
		t.Fatalf("expected initial schema history, got %d", len(created.SchemaHistory))
	}

	updated, err := svc.Update(context.Background(), content.UpdateContentTypeRequest{
		ID: created.ID,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":   map[string]any{"type": "string"},
				"summary": map[string]any{"type": "string"},
			},
		},
		UpdatedBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("update content type: %v", err)
	}
	if updated.SchemaVersion != "landing-page@v1.1.0" {
		t.Fatalf("expected schema_version landing-page@v1.1.0, got %q", updated.SchemaVersion)
	}
	if len(updated.SchemaHistory) != 2 {
		t.Fatalf("expected schema history to append, got %d", len(updated.SchemaHistory))
	}
	if updated.SchemaHistory[1].Version != updated.SchemaVersion {
		t.Fatalf("expected history version %q, got %q", updated.SchemaVersion, updated.SchemaHistory[1].Version)
	}
}

func TestContentTypeServiceUpdateBumpsVersionForUISchemaChange(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	created, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name: "Article",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{"type": "string"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	updated, err := svc.Update(context.Background(), content.UpdateContentTypeRequest{
		ID:        created.ID,
		UISchema:  map[string]any{"layout": "stacked"},
		UpdatedBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("update content type: %v", err)
	}
	if updated.SchemaVersion != "article@v1.0.1" {
		t.Fatalf("expected schema_version article@v1.0.1, got %q", updated.SchemaVersion)
	}
}

func TestContentTypeServicePublishBlocksBreakingChanges(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	created, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name: "Article",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{"type": "string"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	active := string(content.ContentTypeStatusActive)
	_, err = svc.Update(context.Background(), content.UpdateContentTypeRequest{
		ID:     created.ID,
		Status: &active,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{"type": "number"},
			},
		},
		UpdatedBy: uuid.New(),
	})
	if !errors.Is(err, content.ErrContentTypeSchemaBreaking) {
		t.Fatalf("expected ErrContentTypeSchemaBreaking, got %v", err)
	}

	updated, err := svc.Update(context.Background(), content.UpdateContentTypeRequest{
		ID:     created.ID,
		Status: &active,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{"type": "number"},
			},
		},
		UpdatedBy:            uuid.New(),
		AllowBreakingChanges: true,
	})
	if err != nil {
		t.Fatalf("update with breaking override: %v", err)
	}
	if updated.SchemaVersion != "article@v2.0.0" {
		t.Fatalf("expected schema_version article@v2.0.0, got %q", updated.SchemaVersion)
	}
}
