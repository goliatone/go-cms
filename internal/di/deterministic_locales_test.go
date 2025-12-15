package di

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms/internal/identity"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
)

func TestSeedLocalesDeterministicIDs(t *testing.T) {
	ctx := context.Background()

	cfg := runtimeconfig.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Locales = []string{"en", "es"}

	c1, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("create container 1: %v", err)
	}
	en1, err := c1.memoryLocaleRepo.GetByCode(ctx, "en")
	if err != nil {
		t.Fatalf("get locale en from container 1: %v", err)
	}
	es1, err := c1.memoryLocaleRepo.GetByCode(ctx, "es")
	if err != nil {
		t.Fatalf("get locale es from container 1: %v", err)
	}

	c2, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("create container 2: %v", err)
	}
	en2, err := c2.memoryLocaleRepo.GetByCode(ctx, "en")
	if err != nil {
		t.Fatalf("get locale en from container 2: %v", err)
	}
	es2, err := c2.memoryLocaleRepo.GetByCode(ctx, "es")
	if err != nil {
		t.Fatalf("get locale es from container 2: %v", err)
	}

	expectedEN := identity.LocaleUUID("en")
	if en1.ID != expectedEN || en2.ID != expectedEN {
		t.Fatalf("unexpected en locale ids: got %s and %s want %s", en1.ID, en2.ID, expectedEN)
	}

	expectedES := identity.LocaleUUID("es")
	if es1.ID != expectedES || es2.ID != expectedES {
		t.Fatalf("unexpected es locale ids: got %s and %s want %s", es1.ID, es2.ID, expectedES)
	}
}
