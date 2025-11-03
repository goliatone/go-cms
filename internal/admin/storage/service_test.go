package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
	"github.com/goliatone/go-cms/internal/storageconfig"
	"github.com/goliatone/go-cms/pkg/storage"
)

var fixedTime = time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)

func TestService_ApplyConfigCreatesAndUpdatesProfiles(t *testing.T) {
	repo := storageconfig.NewMemoryRepository()
	recorder := jobs.NewInMemoryAuditRecorder()
	svc := NewService(repo, recorder, WithClock(func() time.Time { return fixedTime }))

	cfg := runtimeconfig.StorageConfig{
		Profiles: []storage.Profile{
			{
				Name:        "primary",
				Description: "Primary DB",
				Provider:    "bun",
				Config: storage.Config{
					Name:   "primary",
					Driver: "bun",
					DSN:    "postgres://primary",
				},
				Default: true,
				Labels: map[string]string{
					"tier": "rw",
				},
			},
		},
		Aliases: map[string]string{
			"content": "primary",
		},
	}

	ctx := context.Background()
	if err := svc.ApplyConfig(ctx, cfg); err != nil {
		t.Fatalf("ApplyConfig() error = %v", err)
	}

	stored, err := repo.Get(ctx, "primary")
	if err != nil {
		t.Fatalf("repo.Get() error = %v", err)
	}
	if stored.Description != "Primary DB" || stored.Default != true {
		t.Fatalf("unexpected stored profile %+v", stored)
	}

	if target, ok := svc.ResolveAlias("content"); !ok || target != "primary" {
		t.Fatalf("expected alias to resolve to primary, got %q %v", target, ok)
	}

	events := recorder.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 audit events, got %d", len(events))
	}
	if events[0].Action != "storage_profile_created" || events[0].OccurredAt != fixedTime {
		t.Fatalf("unexpected audit event %+v", events[0])
	}
	if events[1].Action != "storage_profile_aliases_updated" {
		t.Fatalf("unexpected alias event %+v", events[1])
	}

	// Update configuration: flip default, change description, remove alias.
	cfg.Profiles[0].Description = "Primary DB updated"
	cfg.Profiles[0].Default = false
	cfg.Aliases = map[string]string{}

	if err := svc.ApplyConfig(ctx, cfg); err != nil {
		t.Fatalf("ApplyConfig() update error = %v", err)
	}

	events = recorder.Events()
	if len(events) != 4 {
		t.Fatalf("expected 4 audit events, got %d", len(events))
	}
	if events[2].Action != "storage_profile_updated" {
		t.Fatalf("expected update action, got %+v", events[2])
	}
	if events[3].Action != "storage_profile_aliases_updated" {
		t.Fatalf("expected alias update action, got %+v", events[3])
	}
}

func TestService_ApplyConfigDeletesMissingProfiles(t *testing.T) {
	repo := storageconfig.NewMemoryRepository()
	recorder := jobs.NewInMemoryAuditRecorder()
	svc := NewService(repo, recorder, WithClock(func() time.Time { return fixedTime }))

	initial := runtimeconfig.StorageConfig{
		Profiles: []storage.Profile{
			{
				Name:     "primary",
				Provider: "bun",
				Config: storage.Config{
					Name:   "primary",
					Driver: "bun",
					DSN:    "postgres://primary",
				},
			},
			{
				Name:     "replica",
				Provider: "bun",
				Config: storage.Config{
					Name:   "replica",
					Driver: "bun",
					DSN:    "postgres://replica",
				},
			},
		},
	}
	if err := svc.ApplyConfig(context.Background(), initial); err != nil {
		t.Fatalf("ApplyConfig() initial error = %v", err)
	}

	update := runtimeconfig.StorageConfig{
		Profiles: []storage.Profile{
			{
				Name:     "replica",
				Provider: "bun",
				Config: storage.Config{
					Name:   "replica",
					Driver: "bun",
					DSN:    "postgres://replica",
				},
				Default: true,
			},
		},
	}
	if err := svc.ApplyConfig(context.Background(), update); err != nil {
		t.Fatalf("ApplyConfig() update error = %v", err)
	}

	if _, err := repo.Get(context.Background(), "primary"); !errors.Is(err, storageconfig.ErrProfileNotFound) {
		t.Fatalf("expected primary profile to be deleted, got %v", err)
	}

	events := recorder.Events()
	foundDelete := false
	for _, evt := range events {
		if evt.Action == "storage_profile_deleted" && evt.EntityID == "primary" {
			foundDelete = true
			break
		}
	}
	if !foundDelete {
		t.Fatalf("expected delete audit event, got %+v", events)
	}
}

func TestService_ApplyConfigValidationError(t *testing.T) {
	repo := storageconfig.NewMemoryRepository()
	svc := NewService(repo, nil)

	cfg := runtimeconfig.StorageConfig{
		Profiles: []storage.Profile{
			{
				Name: "invalid",
				Config: storage.Config{
					Name:   "invalid",
					Driver: "bun",
					DSN:    "postgres://invalid",
				},
			},
		},
	}

	err := svc.ApplyConfig(context.Background(), cfg)
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("expected ErrConfigInvalid, got %v", err)
	}

	if _, err := repo.List(context.Background()); err != nil {
		t.Fatalf("List() unexpected error %v", err)
	}
}

func TestService_AliasesReturnsCopy(t *testing.T) {
	repo := storageconfig.NewMemoryRepository()
	svc := NewService(repo, nil)

	cfg := runtimeconfig.StorageConfig{
		Profiles: []storage.Profile{
			{
				Name:     "primary",
				Provider: "bun",
				Config: storage.Config{
					Name:   "primary",
					Driver: "bun",
					DSN:    "postgres://primary",
				},
			},
		},
		Aliases: map[string]string{
			"content": "primary",
		},
	}
	if err := svc.ApplyConfig(context.Background(), cfg); err != nil {
		t.Fatalf("ApplyConfig() error = %v", err)
	}

	aliases := svc.Aliases()
	aliases["content"] = "mutated"

	other := svc.Aliases()
	if target, ok := other["content"]; !ok || target != "primary" {
		t.Fatalf("expected original alias retained, got %q %v", target, ok)
	}
}
