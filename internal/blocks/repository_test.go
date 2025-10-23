package blocks_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms/internal/blocks"
)

func TestMemoryRepositories_FixtureContract(t *testing.T) {
	t.Skip("pending block service implementation")

	defRepo := blocks.NewMemoryDefinitionRepository()
	instRepo := blocks.NewMemoryInstanceRepository()
	trRepo := blocks.NewMemoryTranslationRepository()

	ctx := context.Background()

	_ = defRepo
	_ = instRepo
	_ = trRepo
	_ = ctx
}
