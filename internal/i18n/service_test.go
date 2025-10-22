package i18n

import (
	"context"
	"path/filepath"
	"testing"
)

func TestServiceTranslateWithFallback(t *testing.T) {
	svc, fixture := mustLoadFixtureService(t)

	translator := svc.Translator()

	t.Run("falls back to regional parent", func(t *testing.T) {
		got, err := translator.Translate("es-mx", "landing.headline")
		if err != nil {
			t.Fatalf("translate: %v", err)
		}
		if got != "Bienvenido" {
			t.Fatalf("expected Spanish translation, got %q", got)
		}
	})

	t.Run("falls back to default locale", func(t *testing.T) {
		got, err := translator.Translate("es-mx", "landing.tagline")
		if err != nil {
			t.Fatalf("translate: %v", err)
		}
		if got != "Build once, publish everywhere" {
			t.Fatalf("expected English fallback, got %q", got)
		}
	})

	t.Run("formats arguments", func(t *testing.T) {
		got, err := translator.Translate("es-mx", "landing.greeting", "Codex")
		if err != nil {
			t.Fatalf("translate: %v", err)
		}
		if got != "Hola, Codex!" {
			t.Fatalf("expected formatted greeting, got %q", got)
		}
	})

	t.Run("defaults locale when empty", func(t *testing.T) {
		got, err := translator.Translate("", "landing.tagline")
		if err != nil {
			t.Fatalf("translate: %v", err)
		}
		if got != "Build once, publish everywhere" {
			t.Fatalf("expected default locale fallback, got %q", got)
		}
	})

	if svc.DefaultLocale() != fixture.Config.DefaultLocale {
		t.Fatalf("expected default locale %q got %q", fixture.Config.DefaultLocale, svc.DefaultLocale())
	}
}

func TestTemplateHelpersHandleMissingKeys(t *testing.T) {
	svc, fixture := mustLoadFixtureService(t)

	helpers := svc.TemplateHelpers(fixture.Config.HelperConfig())
	translateHelper, ok := helpers["translate"]
	if !ok {
		t.Fatalf("expected translate helper to be registered")
	}

	translateFn, ok := translateHelper.(func(string, string, ...any) string)
	if !ok {
		t.Fatalf("translate helper has unexpected signature %T", translateHelper)
	}

	got := translateFn("es", "unknown.key")
	if got != "unknown.key" {
		t.Fatalf("missing translation should return key, got %q", got)
	}
}

func mustLoadFixtureService(t *testing.T) (Service, *Fixture) {
	t.Helper()

	path := filepath.Join("testdata", "translations_fixture.json")
	loader := NewLoader(path)

	fixture, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	service, err := NewInMemoryService(fixture.Config, fixture.Translations)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	return service, fixture
}
