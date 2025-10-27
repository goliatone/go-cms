package blocks_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

func TestServiceRegisterDefinition(t *testing.T) {
	svc := newBlockService()

	def, err := svc.RegisterDefinition(context.Background(), blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}
	if def.Name != "hero" {
		t.Fatalf("expected name hero got %s", def.Name)
	}

	if _, err := svc.RegisterDefinition(context.Background(), blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	}); !errors.Is(err, blocks.ErrDefinitionExists) {
		t.Fatalf("expected ErrDefinitionExists got %v", err)
	}
}

func TestServiceCreateInstance(t *testing.T) {
	svc := newBlockService()
	def, err := svc.RegisterDefinition(context.Background(), blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	pageID := uuid.MustParse("00000000-0000-0000-0000-00000000aaaa")
	inst, err := svc.CreateInstance(context.Background(), blocks.CreateInstanceInput{
		DefinitionID: def.ID,
		PageID:       &pageID,
		Region:       "hero",
		Position:     0,
		Configuration: map[string]any{
			"layout": "full",
		},
		CreatedBy: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		UpdatedBy: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if inst.Region != "hero" {
		t.Fatalf("expected region hero got %s", inst.Region)
	}

	if _, err := svc.CreateInstance(context.Background(), blocks.CreateInstanceInput{}); !errors.Is(err, blocks.ErrInstanceDefinitionRequired) {
		t.Fatalf("expected ErrInstanceDefinitionRequired got %v", err)
	}
}

func TestAddTranslationResolvesMedia(t *testing.T) {
	ctx := context.Background()
	provider := &blockMediaProvider{
		assets: map[string]*interfaces.MediaAsset{
			"asset-1": {
				Reference: interfaces.MediaReference{ID: "asset-1"},
				Metadata:  interfaces.MediaMetadata{ID: "asset-1"},
				Source:    &interfaces.MediaResource{URL: "https://cdn.local/full.jpg"},
				Renditions: map[string]*interfaces.MediaResource{
					"thumb": {URL: "https://cdn.local/thumb.jpg"},
				},
			},
		},
	}
	svc := newBlockService(blocks.WithMediaService(media.NewService(provider)))
	def, err := svc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}
	pageID := uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	inst, err := svc.CreateInstance(ctx, blocks.CreateInstanceInput{
		DefinitionID: def.ID,
		PageID:       &pageID,
		Region:       "hero",
		Position:     0,
		Configuration: map[string]any{
			"variant": "primary",
		},
		CreatedBy: uuid.MustParse("00000000-0000-0000-0000-000000000101"),
		UpdatedBy: uuid.MustParse("00000000-0000-0000-0000-000000000101"),
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	localeID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	translation, err := svc.AddTranslation(ctx, blocks.AddTranslationInput{
		BlockInstanceID: inst.ID,
		LocaleID:        localeID,
		Content: map[string]any{
			"title": "Hero block",
		},
		MediaBindings: media.BindingSet{
			"image": {{
				Slot:      "image",
				Reference: interfaces.MediaReference{ID: "asset-1"},
				Required:  []string{"thumb"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("add translation: %v", err)
	}
	if translation.ResolvedMedia == nil || translation.ResolvedMedia["image"] == nil {
		t.Fatalf("expected resolved media to be populated")
	}
	if translation.ResolvedMedia["image"][0].Renditions["thumb"].URL != "https://cdn.local/thumb.jpg" {
		t.Fatalf("expected resolved thumb rendition")
	}

	instances, err := svc.ListPageInstances(ctx, pageID)
	if err != nil {
		t.Fatalf("list page instances: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected one instance, got %d", len(instances))
	}
	gotTranslations := instances[0].Translations
	if len(gotTranslations) != 1 || gotTranslations[0].ResolvedMedia["image"][0].Metadata.ID != "asset-1" {
		t.Fatalf("expected resolved media propagated in list")
	}
}

func TestServiceAddTranslation(t *testing.T) {
	svc := newBlockService()
	def, err := svc.RegisterDefinition(context.Background(), blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	inst, err := svc.CreateInstance(context.Background(), blocks.CreateInstanceInput{
		DefinitionID: def.ID,
		Region:       "hero",
		CreatedBy:    uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		UpdatedBy:    uuid.MustParse("11111111-1111-1111-1111-111111111111"),
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	localeID := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	tr, err := svc.AddTranslation(context.Background(), blocks.AddTranslationInput{
		BlockInstanceID: inst.ID,
		LocaleID:        localeID,
		Content: map[string]any{
			"title": "Hello",
		},
	})
	if err != nil {
		t.Fatalf("add translation: %v", err)
	}
	if tr.LocaleID != localeID {
		t.Fatalf("expected locale %s", localeID)
	}

	if _, err := svc.AddTranslation(context.Background(), blocks.AddTranslationInput{
		BlockInstanceID: inst.ID,
		LocaleID:        localeID,
		Content:         map[string]any{"title": "Duplicate"},
	}); !errors.Is(err, blocks.ErrTranslationExists) {
		t.Fatalf("expected ErrTranslationExists got %v", err)
	}
}

func TestRegistrySeedsDefinitions(t *testing.T) {
	reg := blocks.NewRegistry()
	reg.Register(blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})

	svc := newBlockService(blocks.WithRegistry(reg))

	defs, err := svc.ListDefinitions(context.Background())
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected registry definition to seed, got %d", len(defs))
	}
}

func TestBlockServiceInstanceVersionLifecycle(t *testing.T) {
	svc := newBlockService(blocks.WithVersioningEnabled(true), blocks.WithVersionRetentionLimit(5))

	def, err := svc.RegisterDefinition(context.Background(), blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	authorID := uuid.New()
	instance, err := svc.CreateInstance(context.Background(), blocks.CreateInstanceInput{
		DefinitionID: def.ID,
		Region:       "hero",
		Position:     0,
		Configuration: map[string]any{
			"layout": "full",
		},
		IsGlobal:  true,
		CreatedBy: authorID,
		UpdatedBy: authorID,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	draftSnapshot := blocks.BlockVersionSnapshot{
		Configuration: map[string]any{"title": "Draft"},
		Translations: []blocks.BlockVersionTranslationSnapshot{{
			Locale:  "en",
			Content: map[string]any{"body": "Draft copy"},
		}},
	}

	ctx := context.Background()
	draft, err := svc.CreateDraft(ctx, blocks.CreateInstanceDraftRequest{
		InstanceID: instance.ID,
		Snapshot:   draftSnapshot,
		CreatedBy:  authorID,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if draft.Version != 1 {
		t.Fatalf("expected version 1 got %d", draft.Version)
	}
	if draft.Status != domain.StatusDraft {
		t.Fatalf("expected draft status got %s", draft.Status)
	}

	publisher := uuid.New()
	firstPublish, err := svc.PublishDraft(ctx, blocks.PublishInstanceDraftRequest{
		InstanceID:  instance.ID,
		Version:     draft.Version,
		PublishedBy: publisher,
	})
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}
	if firstPublish.Status != domain.StatusPublished {
		t.Fatalf("expected published status got %s", firstPublish.Status)
	}

	base := firstPublish.Version
	secondDraft, err := svc.CreateDraft(ctx, blocks.CreateInstanceDraftRequest{
		InstanceID:  instance.ID,
		Snapshot:    blocks.BlockVersionSnapshot{Configuration: map[string]any{"title": "Updated"}},
		CreatedBy:   authorID,
		UpdatedBy:   authorID,
		BaseVersion: &base,
	})
	if err != nil {
		t.Fatalf("create second draft: %v", err)
	}
	if secondDraft.Version != 2 {
		t.Fatalf("expected version 2 got %d", secondDraft.Version)
	}

	secondPublisher := uuid.New()
	secondPublish, err := svc.PublishDraft(ctx, blocks.PublishInstanceDraftRequest{
		InstanceID:  instance.ID,
		Version:     secondDraft.Version,
		PublishedBy: secondPublisher,
	})
	if err != nil {
		t.Fatalf("publish second draft: %v", err)
	}
	if secondPublish.Status != domain.StatusPublished {
		t.Fatalf("expected second version published got %s", secondPublish.Status)
	}

	versions, err := svc.ListVersions(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions got %d", len(versions))
	}
	if versions[0].Status != domain.StatusArchived {
		t.Fatalf("expected first version archived got %s", versions[0].Status)
	}
	if versions[1].Status != domain.StatusPublished {
		t.Fatalf("expected second version published got %s", versions[1].Status)
	}

	restored, err := svc.RestoreVersion(ctx, blocks.RestoreInstanceVersionRequest{
		InstanceID: instance.ID,
		Version:    1,
		RestoredBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("restore version: %v", err)
	}
	if restored.Version != 3 {
		t.Fatalf("expected restored version 3 got %d", restored.Version)
	}
	if restored.Status != domain.StatusDraft {
		t.Fatalf("expected restored status draft got %s", restored.Status)
	}

	allVersions, err := svc.ListVersions(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list versions after restore: %v", err)
	}
	if len(allVersions) != 3 {
		t.Fatalf("expected 3 versions got %d", len(allVersions))
	}
	if allVersions[2].Status != domain.StatusDraft {
		t.Fatalf("expected newest version draft got %s", allVersions[2].Status)
	}

	globals, err := svc.ListGlobalInstances(ctx)
	if err != nil {
		t.Fatalf("list global instances: %v", err)
	}
	if len(globals) != 1 {
		t.Fatalf("expected 1 global instance got %d", len(globals))
	}
	if globals[0].PublishedVersion == nil || *globals[0].PublishedVersion != 2 {
		t.Fatalf("expected published version pointer 2 got %v", globals[0].PublishedVersion)
	}
	if globals[0].CurrentVersion != 3 {
		t.Fatalf("expected current version 3 got %d", globals[0].CurrentVersion)
	}
}

func newBlockService(opts ...blocks.ServiceOption) blocks.Service {
	defRepo := blocks.NewMemoryDefinitionRepository()
	instRepo := blocks.NewMemoryInstanceRepository()
	versionRepo := blocks.NewMemoryInstanceVersionRepository()
	trRepo := blocks.NewMemoryTranslationRepository()

	counter := 0
	idFn := func() uuid.UUID {
		counter++
		return uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012x", counter))
	}

	baseOpts := []blocks.ServiceOption{
		blocks.WithClock(func() time.Time { return time.Unix(0, 0) }),
		blocks.WithIDGenerator(idFn),
		blocks.WithInstanceVersionRepository(versionRepo),
	}
	baseOpts = append(baseOpts, opts...)
	return blocks.NewService(defRepo, instRepo, trRepo, baseOpts...)
}

type blockMediaProvider struct {
	assets map[string]*interfaces.MediaAsset
}

func (p *blockMediaProvider) Resolve(_ context.Context, req interfaces.MediaResolveRequest) (*interfaces.MediaAsset, error) {
	if p.assets == nil {
		return nil, nil
	}
	if req.Reference.Locale != "" {
		if asset, ok := p.assets[req.Reference.ID+":"+req.Reference.Locale]; ok {
			return asset, nil
		}
	}
	asset, ok := p.assets[req.Reference.ID]
	if !ok {
		return nil, nil
	}
	return asset, nil
}

func (p *blockMediaProvider) ResolveBatch(ctx context.Context, reqs []interfaces.MediaResolveRequest) (map[string]*interfaces.MediaAsset, error) {
	result := make(map[string]*interfaces.MediaAsset, len(reqs))
	for _, req := range reqs {
		asset, _ := p.Resolve(ctx, req)
		result[req.Reference.ID] = asset
	}
	return result, nil
}

func (p *blockMediaProvider) Invalidate(context.Context, ...interfaces.MediaReference) error {
	return nil
}
