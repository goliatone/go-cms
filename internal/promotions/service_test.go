package promotions_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/promotions"
	"github.com/goliatone/go-cms/internal/schema"
	"github.com/google/uuid"
)

func TestPromotionService_ContentTypePromotesBlocks(t *testing.T) {
	ctx := context.Background()

	envSvc := newEnvService(t)
	blockSvc := newBlockService(t, envSvc)
	typeRepo := content.NewMemoryContentTypeRepository()
	typeSvc := content.NewContentTypeService(typeRepo, content.WithContentTypeEnvironmentService(envSvc))

	sourceSchema := contentSchema("article", "article@v1.0.0", map[string]any{
		"title": map[string]any{"type": "string"},
	}, []string{"hero"})

	sourceType, err := typeSvc.Create(ctx, content.CreateContentTypeRequest{
		Name:           "Article",
		Slug:           "article",
		Status:         content.ContentTypeStatusActive,
		Schema:         sourceSchema,
		EnvironmentKey: "dev",
	})
	if err != nil {
		t.Fatalf("create source content type: %v", err)
	}

	if _, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:           "Hero",
		Slug:           "hero",
		Schema:         blockSchema("hero", "string"),
		EnvironmentKey: "dev",
	}); err != nil {
		t.Fatalf("register source block definition: %v", err)
	}

	if _, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:           "Hero Old",
		Slug:           "hero",
		Schema:         blockSchema("hero", "number"),
		EnvironmentKey: "prod",
	}); err != nil {
		t.Fatalf("register target block definition: %v", err)
	}

	promo := promotions.NewService(
		envSvc,
		typeRepo,
		content.NewMemoryContentRepository(),
		content.NewMemoryLocaleRepository(),
		promotions.WithBlockService(blockSvc),
	)

	item, err := promo.PromoteContentType(ctx, promotions.PromoteContentTypeRequest{
		ContentTypeID:     sourceType.ID,
		TargetEnvironment: "prod",
		Options: promotions.PromoteOptions{
			PromoteAsActive: true,
		},
	})
	if err != nil {
		t.Fatalf("promote content type: %v", err)
	}
	if item == nil || item.Status != "created" {
		t.Fatalf("expected created promotion item, got %+v", item)
	}

	targetEnv, _ := envSvc.GetEnvironmentByKey(ctx, "prod")
	targetType, err := typeRepo.GetBySlug(ctx, "article", targetEnv.ID.String())
	if err != nil {
		t.Fatalf("get target content type: %v", err)
	}
	if targetType.SchemaVersion != sourceType.SchemaVersion {
		t.Fatalf("expected schema_version %q, got %q", sourceType.SchemaVersion, targetType.SchemaVersion)
	}

	defs, err := blockSvc.ListDefinitions(ctx, "prod")
	if err != nil {
		t.Fatalf("list target block definitions: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 block definition, got %d", len(defs))
	}
	props, _ := defs[0].Schema["properties"].(map[string]any)
	title, _ := props["title"].(map[string]any)
	if title["type"] != "string" {
		t.Fatalf("expected target block schema to be updated")
	}
}

func TestPromotionService_ContentEntryMigratesSnapshot(t *testing.T) {
	ctx := context.Background()

	envSvc := newEnvService(t)
	typeRepo := content.NewMemoryContentTypeRepository()
	contentRepo := content.NewMemoryContentRepository()
	localeRepo := content.NewMemoryLocaleRepository()
	locale := &content.Locale{
		ID:        uuid.New(),
		Code:      "en",
		Display:   "English",
		IsActive:  true,
		IsDefault: true,
	}
	localeRepo.Put(locale)

	typeSvc := content.NewContentTypeService(typeRepo, content.WithContentTypeEnvironmentService(envSvc))
	devSchema := contentSchema("article", "article@v1.0.0", map[string]any{
		"title": map[string]any{"type": "string"},
	}, nil)
	prodSchema := contentSchema("article", "article@v2.0.0", map[string]any{
		"headline": map[string]any{"type": "string"},
	}, nil)

	devType, err := typeSvc.Create(ctx, content.CreateContentTypeRequest{
		Name:           "Article",
		Slug:           "article",
		Status:         content.ContentTypeStatusActive,
		Schema:         devSchema,
		EnvironmentKey: "dev",
	})
	if err != nil {
		t.Fatalf("create dev content type: %v", err)
	}

	prodType, err := typeSvc.Create(ctx, content.CreateContentTypeRequest{
		Name:           "Article",
		Slug:           "article",
		Status:         content.ContentTypeStatusActive,
		Schema:         prodSchema,
		EnvironmentKey: "prod",
	})
	if err != nil {
		t.Fatalf("create prod content type: %v", err)
	}

	now := time.Now().UTC()
	actor := uuid.New()
	contentID := uuid.New()
	source := &content.Content{
		ID:            contentID,
		ContentTypeID: devType.ID,
		EnvironmentID: cmsenv.IDForKey("dev"),
		Status:        string(domain.StatusPublished),
		Slug:          "hello",
		CreatedBy:     actor,
		UpdatedBy:     actor,
		CreatedAt:     now,
		UpdatedAt:     now,
		Translations: []*content.ContentTranslation{
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  locale.ID,
				Title:     "Hello",
				Content: map[string]any{
					"title":   "Hello",
					"_schema": "article@v1.0.0",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	if _, err := contentRepo.Create(ctx, source); err != nil {
		t.Fatalf("create source content: %v", err)
	}

	snapshot := content.ContentVersionSnapshot{
		Translations: []content.ContentVersionTranslationSnapshot{
			{
				Locale:  "en",
				Title:   "Hello",
				Content: map[string]any{"title": "Hello", "_schema": "article@v1.0.0"},
			},
		},
	}
	version := &content.ContentVersion{
		ID:          uuid.New(),
		ContentID:   contentID,
		Version:     1,
		Status:      domain.StatusPublished,
		Snapshot:    snapshot,
		CreatedBy:   actor,
		CreatedAt:   now,
		PublishedAt: &now,
		PublishedBy: &actor,
	}
	if _, err := contentRepo.CreateVersion(ctx, version); err != nil {
		t.Fatalf("create source version: %v", err)
	}

	source.CurrentVersion = 1
	source.PublishedVersion = intPtr(1)
	source.PublishedAt = &now
	source.PublishedBy = &actor
	if _, err := contentRepo.Update(ctx, source); err != nil {
		t.Fatalf("update source content: %v", err)
	}

	migrator := schema.NewMigrator()
	if err := migrator.Register("article", "article@v1.0.0", "article@v2.0.0", func(payload map[string]any) (map[string]any, error) {
		out := map[string]any{}
		for key, value := range payload {
			out[key] = value
		}
		if title, ok := out["title"]; ok {
			out["headline"] = title
			delete(out, "title")
		}
		return out, nil
	}); err != nil {
		t.Fatalf("register migrator: %v", err)
	}

	promo := promotions.NewService(envSvc, typeRepo, contentRepo, localeRepo, promotions.WithSchemaMigrator(migrator))
	item, err := promo.PromoteContentEntry(ctx, promotions.PromoteContentEntryRequest{
		ContentID:         contentID,
		TargetEnvironment: "prod",
		Options: promotions.PromoteOptions{
			PromoteAsPublished: true,
		},
	})
	if err != nil {
		t.Fatalf("promote content entry: %v", err)
	}
	if item == nil || item.Status != "created" {
		t.Fatalf("expected created promotion item, got %+v", item)
	}

	target, err := contentRepo.GetBySlug(ctx, "hello", prodType.ID, cmsenv.IDForKey("prod").String())
	if err != nil {
		t.Fatalf("get promoted content: %v", err)
	}
	if target.Status != string(domain.StatusPublished) {
		t.Fatalf("expected promoted content to be published, got %q", target.Status)
	}

	versions, err := contentRepo.ListVersions(ctx, target.ID)
	if err != nil || len(versions) == 0 {
		t.Fatalf("expected promoted versions, got %v", err)
	}
	translated := versions[len(versions)-1].Snapshot.Translations[0].Content
	if translated["headline"] != "Hello" {
		t.Fatalf("expected migrated headline, got %v", translated["headline"])
	}
	if translated["_schema"] != "article@v2.0.0" {
		t.Fatalf("expected schema version article@v2.0.0, got %v", translated["_schema"])
	}
}

func TestPromotionService_DryRunDoesNotPersist(t *testing.T) {
	ctx := context.Background()

	envSvc := newEnvService(t)
	blockSvc := newBlockService(t, envSvc)
	typeRepo := content.NewMemoryContentTypeRepository()
	typeSvc := content.NewContentTypeService(typeRepo, content.WithContentTypeEnvironmentService(envSvc))

	sourceSchema := contentSchema("article", "article@v1.0.0", map[string]any{
		"title": map[string]any{"type": "string"},
	}, []string{"hero"})

	sourceType, err := typeSvc.Create(ctx, content.CreateContentTypeRequest{
		Name:           "Article",
		Slug:           "article",
		Status:         content.ContentTypeStatusActive,
		Schema:         sourceSchema,
		EnvironmentKey: "dev",
	})
	if err != nil {
		t.Fatalf("create source content type: %v", err)
	}

	if _, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:           "Hero",
		Slug:           "hero",
		Schema:         blockSchema("hero", "string"),
		EnvironmentKey: "dev",
	}); err != nil {
		t.Fatalf("register source block definition: %v", err)
	}

	promo := promotions.NewService(
		envSvc,
		typeRepo,
		content.NewMemoryContentRepository(),
		content.NewMemoryLocaleRepository(),
		promotions.WithBlockService(blockSvc),
	)

	item, err := promo.PromoteContentType(ctx, promotions.PromoteContentTypeRequest{
		ContentTypeID:     sourceType.ID,
		TargetEnvironment: "prod",
		Options: promotions.PromoteOptions{
			DryRun: true,
		},
	})
	if err != nil {
		t.Fatalf("dry-run promote content type: %v", err)
	}
	if item == nil || item.Details["dry_run"] != true {
		t.Fatalf("expected dry_run details")
	}

	_, err = typeRepo.GetBySlug(ctx, "article", cmsenv.IDForKey("prod").String())
	var nf *content.NotFoundError
	if err == nil || !errors.As(err, &nf) {
		t.Fatalf("expected no content type persisted in dry-run")
	}

	defs, err := blockSvc.ListDefinitions(ctx, "prod")
	if err != nil {
		t.Fatalf("list target block definitions: %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("expected no block definitions created during dry-run")
	}
}

func newEnvService(t *testing.T) cmsenv.Service {
	t.Helper()
	repo := cmsenv.NewMemoryRepository()
	svc := cmsenv.NewService(repo)
	if _, err := svc.CreateEnvironment(context.Background(), cmsenv.CreateEnvironmentInput{
		Key:       "dev",
		Name:      "Dev",
		IsDefault: true,
	}); err != nil {
		t.Fatalf("create dev env: %v", err)
	}
	if _, err := svc.CreateEnvironment(context.Background(), cmsenv.CreateEnvironmentInput{
		Key:       "prod",
		Name:      "Prod",
		IsActive:  boolPtr(true),
		IsDefault: false,
	}); err != nil {
		t.Fatalf("create prod env: %v", err)
	}
	return svc
}

func newBlockService(t *testing.T, envSvc cmsenv.Service) blocks.Service {
	t.Helper()
	defRepo := blocks.NewMemoryDefinitionRepository()
	instRepo := blocks.NewMemoryInstanceRepository()
	trRepo := blocks.NewMemoryTranslationRepository()
	return blocks.NewService(defRepo, instRepo, trRepo, blocks.WithEnvironmentService(envSvc))
}

func contentSchema(slug, version string, properties map[string]any, allowBlocks []string) map[string]any {
	meta := map[string]any{
		"slug":           slug,
		"schema_version": version,
	}
	if len(allowBlocks) > 0 {
		meta["block_availability"] = map[string]any{
			"allow": allowBlocks,
		}
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"metadata":   meta,
	}
}

func blockSchema(slug string, valueType string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": valueType},
		},
		"metadata": map[string]any{
			"slug":           slug,
			"schema_version": slug + "@v1.0.0",
		},
	}
}

func intPtr(value int) *int {
	v := value
	return &v
}

func boolPtr(value bool) *bool {
	v := value
	return &v
}
