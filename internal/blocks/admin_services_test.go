package blocks

import (
	"context"
	"errors"
	"testing"

	internalcontent "github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

func TestAdminBlockReadServiceListDefinitions(t *testing.T) {
	t.Parallel()

	defRepo := NewMemoryDefinitionRepository()
	instRepo := NewMemoryInstanceRepository()
	trRepo := NewMemoryTranslationRepository()
	service := NewService(defRepo, instRepo, trRepo)
	adminRead := NewAdminBlockReadService(service, nil, nil, nil)

	if _, err := service.RegisterDefinition(context.Background(), RegisterDefinitionInput{
		Name:   "Hero",
		Slug:   "hero",
		Schema: map[string]any{"x-schema-version": "v1"},
	}); err != nil {
		t.Fatalf("register definition: %v", err)
	}

	records, total, err := adminRead.ListDefinitions(context.Background(), interfaces.AdminBlockDefinitionListOptions{})
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	if total != 1 || len(records) != 1 {
		t.Fatalf("expected one definition, got total=%d len=%d", total, len(records))
	}
	if records[0].Slug != "hero" || records[0].SchemaVersion == "" {
		t.Fatalf("unexpected record: %+v", records[0])
	}
}

func TestAdminBlockWriteServiceCreateDefinition(t *testing.T) {
	t.Parallel()

	defRepo := NewMemoryDefinitionRepository()
	instRepo := NewMemoryInstanceRepository()
	trRepo := NewMemoryTranslationRepository()
	service := NewService(defRepo, instRepo, trRepo)
	adminWrite := NewAdminBlockWriteService(service, nil, nil, nil)

	record, err := adminWrite.CreateDefinition(context.Background(), interfaces.AdminBlockDefinitionCreateRequest{
		Name:   "Hero",
		Slug:   "hero",
		Schema: map[string]any{"x-schema-version": "v1"},
	})
	if err != nil {
		t.Fatalf("create definition: %v", err)
	}
	if record == nil || record.ID == uuid.Nil {
		t.Fatalf("expected created definition record")
	}
}

func TestAdminBlockWriteServiceSaveBlockRequiresResolvedPage(t *testing.T) {
	t.Parallel()

	defRepo := NewMemoryDefinitionRepository()
	instRepo := NewMemoryInstanceRepository()
	trRepo := NewMemoryTranslationRepository()
	service := NewService(defRepo, instRepo, trRepo)
	adminWrite := NewAdminBlockWriteService(service, nil, nil, func(context.Context, uuid.UUID, string) ([]uuid.UUID, error) {
		return nil, nil
	})

	definition, err := service.RegisterDefinition(context.Background(), RegisterDefinitionInput{
		Name:   "Hero",
		Slug:   "hero",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	_, err = adminWrite.SaveBlock(context.Background(), interfaces.AdminBlockSaveRequest{
		DefinitionID: definition.ID,
		ContentID:    uuid.New(),
		Region:       "main",
		Locale:       "en",
		Data:         map[string]any{"headline": "Hello"},
		UpdatedBy:    uuid.New(),
	})
	var notFound *internalcontent.NotFoundError
	if err == nil || !errors.As(err, &notFound) {
		t.Fatalf("expected missing page error, got %v", err)
	}
}
