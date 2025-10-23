package blocks_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
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

func newBlockService(opts ...blocks.ServiceOption) blocks.Service {
	defRepo := blocks.NewMemoryDefinitionRepository()
	instRepo := blocks.NewMemoryInstanceRepository()
	trRepo := blocks.NewMemoryTranslationRepository()

	counter := 0
	idFn := func() uuid.UUID {
		counter++
		return uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012x", counter))
	}

	baseOpts := []blocks.ServiceOption{
		blocks.WithClock(func() time.Time { return time.Unix(0, 0) }),
		blocks.WithIDGenerator(idFn),
	}
	baseOpts = append(baseOpts, opts...)
	return blocks.NewService(defRepo, instRepo, trRepo, baseOpts...)
}
