package blocks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/google/uuid"
)

func TestBlockSchemaMigratorAppliesOrderedSteps(t *testing.T) {
	migrator := blocks.NewMigrator()
	if err := migrator.Register("hero", "hero@v1.0.0", "hero@v1.1.0", func(payload map[string]any) (map[string]any, error) {
		out := map[string]any{}
		for k, v := range payload {
			out[k] = v
		}
		out["step"] = "one"
		return out, nil
	}); err != nil {
		t.Fatalf("register step 1: %v", err)
	}
	if err := migrator.Register("hero", "hero@v1.1.0", "hero@v2.0.0", func(payload map[string]any) (map[string]any, error) {
		out := map[string]any{}
		for k, v := range payload {
			out[k] = v
		}
		out["step"] = "one-two"
		return out, nil
	}); err != nil {
		t.Fatalf("register step 2: %v", err)
	}

	result, err := migrator.Migrate("hero", "hero@v1.0.0", "hero@v2.0.0", map[string]any{"value": "start"})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if result["step"] != "one-two" {
		t.Fatalf("expected ordered migration, got %v", result["step"])
	}
}

func TestPublishDraftRejectsInvalidMigratedPayload(t *testing.T) {
	ctx := context.Background()
	migrator := blocks.NewMigrator()
	if err := migrator.Register("promo", "promo@v1.0.0", "promo@v2.0.0", func(payload map[string]any) (map[string]any, error) {
		out := map[string]any{}
		for k, v := range payload {
			out[k] = v
		}
		delete(out, "title")
		return out, nil
	}); err != nil {
		t.Fatalf("register migration: %v", err)
	}

	svc := newBlockService(
		blocks.WithVersioningEnabled(true),
		blocks.WithSchemaMigrator(migrator),
	)

	def, err := svc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name: "promo",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{"type": "string"},
			},
			"required": []string{"title"},
			"metadata": map[string]any{
				"schema_version": "promo@v2.0.0",
			},
		},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	instance, err := svc.CreateInstance(ctx, blocks.CreateInstanceInput{
		DefinitionID: def.ID,
		Region:       "main",
		Position:     0,
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	draft, err := svc.CreateDraft(ctx, blocks.CreateInstanceDraftRequest{
		InstanceID: instance.ID,
		Snapshot: blocks.BlockVersionSnapshot{
			Translations: []blocks.BlockVersionTranslationSnapshot{{
				Locale: "en",
				Content: map[string]any{
					"_schema": "promo@v1.0.0",
					"title":   "Hello",
				},
			}},
		},
		CreatedBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	if _, err := svc.PublishDraft(ctx, blocks.PublishInstanceDraftRequest{
		InstanceID:  instance.ID,
		Version:     draft.Version,
		PublishedBy: uuid.New(),
	}); err == nil || !errors.Is(err, blocks.ErrBlockSchemaValidationFailed) {
		t.Fatalf("expected validation failure, got %v", err)
	}
}
