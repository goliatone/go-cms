package translations

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/translationconfig"
)

var fixedTime = time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)

func TestService_ApplySettingsCreatesAndUpdates(t *testing.T) {
	repo := translationconfig.NewMemoryRepository()
	recorder := jobs.NewInMemoryAuditRecorder()
	svc := NewService(repo, recorder, WithClock(func() time.Time { return fixedTime }))

	ctx := context.Background()
	if err := svc.ApplySettings(ctx, translationconfig.Settings{
		TranslationsEnabled: true,
		RequireTranslations: true,
	}); err != nil {
		t.Fatalf("ApplySettings() error = %v", err)
	}

	settings, err := svc.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	if !settings.TranslationsEnabled || !settings.RequireTranslations {
		t.Fatalf("unexpected settings %+v", settings)
	}

	if err := svc.ApplySettings(ctx, translationconfig.Settings{
		TranslationsEnabled: true,
		RequireTranslations: false,
	}); err != nil {
		t.Fatalf("ApplySettings() update error = %v", err)
	}

	events := recorder.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 audit events, got %d", len(events))
	}
	if events[0].Action != "translation_settings_created" || events[0].OccurredAt != fixedTime {
		t.Fatalf("unexpected create audit event %+v", events[0])
	}
	if events[1].Action != "translation_settings_updated" || events[1].OccurredAt != fixedTime {
		t.Fatalf("unexpected update audit event %+v", events[1])
	}
}

func TestService_ResetDeletesSettings(t *testing.T) {
	repo := translationconfig.NewMemoryRepository()
	recorder := jobs.NewInMemoryAuditRecorder()
	svc := NewService(repo, recorder, WithClock(func() time.Time { return fixedTime }))

	ctx := context.Background()
	if err := svc.ApplySettings(ctx, translationconfig.Settings{
		TranslationsEnabled: true,
		RequireTranslations: false,
	}); err != nil {
		t.Fatalf("ApplySettings() error = %v", err)
	}

	if err := svc.Reset(ctx); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	if _, err := svc.GetSettings(ctx); !errors.Is(err, translationconfig.ErrSettingsNotFound) {
		t.Fatalf("expected ErrSettingsNotFound, got %v", err)
	}

	events := recorder.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 audit events, got %d", len(events))
	}
	if events[1].Action != "translation_settings_deleted" {
		t.Fatalf("unexpected delete audit event %+v", events[1])
	}
}

func TestService_RequiresRepository(t *testing.T) {
	svc := NewService(nil, nil)

	if err := svc.ApplySettings(context.Background(), translationconfig.Settings{}); !errors.Is(err, ErrRepositoryRequired) {
		t.Fatalf("expected ErrRepositoryRequired, got %v", err)
	}
	if err := svc.Reset(context.Background()); !errors.Is(err, ErrRepositoryRequired) {
		t.Fatalf("expected ErrRepositoryRequired, got %v", err)
	}
}
