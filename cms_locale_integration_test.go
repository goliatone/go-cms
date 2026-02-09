package cms_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/google/uuid"
)

func TestModule_Locales_ResolveByCode(t *testing.T) {
	t.Parallel()

	cfg := cms.DefaultConfig()
	cfg.I18N.Locales = []string{"en", "es"}

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	ctx := context.Background()
	expected, err := module.Container().LocaleRepository().GetByCode(ctx, "es")
	if err != nil {
		t.Fatalf("resolve locale from repository: %v", err)
	}

	record, err := module.Locales().ResolveByCode(ctx, "es")
	if err != nil {
		t.Fatalf("resolve locale by code: %v", err)
	}
	if record.ID == uuid.Nil {
		t.Fatalf("expected non-empty locale id")
	}
	if record.ID != expected.ID {
		t.Fatalf("expected locale id %s, got %s", expected.ID, record.ID)
	}
	if record.Code != "es" {
		t.Fatalf("expected locale code es, got %q", record.Code)
	}
}

func TestModule_Locales_ResolveByCodeUnknownReturnsSentinel(t *testing.T) {
	t.Parallel()

	module, err := cms.New(cms.DefaultConfig())
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	_, err = module.Locales().ResolveByCode(context.Background(), "zz")
	if !errors.Is(err, cms.ErrUnknownLocale) {
		t.Fatalf("expected ErrUnknownLocale, got %v", err)
	}

	var localeNotFound *cms.LocaleNotFoundError
	if !errors.As(err, &localeNotFound) {
		t.Fatalf("expected LocaleNotFoundError, got %T", err)
	}
}

func TestModule_Locales_ResolveByCodeRequiresCode(t *testing.T) {
	t.Parallel()

	module, err := cms.New(cms.DefaultConfig())
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	_, err = module.Locales().ResolveByCode(context.Background(), " ")
	if !errors.Is(err, cms.ErrLocaleCodeRequired) {
		t.Fatalf("expected ErrLocaleCodeRequired, got %v", err)
	}
}
