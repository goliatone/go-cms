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

func TestService_ListProfilesReturnsClones(t *testing.T) {
	repo := storageconfig.NewMemoryRepository()
	svc := NewService(repo, nil)

	profile := storage.Profile{
		Name:     "primary",
		Provider: "bun",
		Config: storage.Config{
			Name:   "primary",
			Driver: "bun",
			DSN:    "postgres://primary",
		},
		Labels: map[string]string{"tier": "rw"},
	}
	if _, err := repo.Upsert(context.Background(), profile); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	list, err := svc.ListProfiles(context.Background())
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(list))
	}

	list[0].Config.Name = "mutated"
	list[0].Labels["tier"] = "ro"

	stored, err := repo.Get(context.Background(), "primary")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Config.Name != "primary" {
		t.Fatalf("expected stored config name unchanged, got %q", stored.Config.Name)
	}
	if stored.Labels["tier"] != "rw" {
		t.Fatalf("expected stored labels unchanged, got %v", stored.Labels)
	}
}

func TestService_GetProfileReturnsClone(t *testing.T) {
	repo := storageconfig.NewMemoryRepository()
	svc := NewService(repo, nil)

	profile := storage.Profile{
		Name:     "primary",
		Provider: "bun",
		Config: storage.Config{
			Name:   "primary",
			Driver: "bun",
			DSN:    "postgres://primary",
		},
	}
	if _, err := repo.Upsert(context.Background(), profile); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	got, err := svc.GetProfile(context.Background(), "primary")
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	got.Config.Name = "mutated"
	original, err := repo.Get(context.Background(), "primary")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if original.Config.Name != "primary" {
		t.Fatalf("expected stored profile untouched, got %q", original.Config.Name)
	}
}

func TestService_ValidateProfileAliasCollision(t *testing.T) {
	svc := NewService(storageconfig.NewMemoryRepository(), nil)

	profile := storage.Profile{
		Name:     "primary",
		Provider: "bun",
		Config: storage.Config{
			Name:   "primary",
			Driver: "bun",
			DSN:    "postgres://primary",
		},
	}
	aliases := map[string]string{
		"primary": "primary",
	}
	err := svc.ValidateProfile(profile, aliases)
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("expected ErrConfigInvalid, got %v", err)
	}
}

func TestService_PreviewProfileRequiresPreviewer(t *testing.T) {
	svc := NewService(storageconfig.NewMemoryRepository(), nil)
	profile := storage.Profile{
		Name:     "primary",
		Provider: "bun",
		Config: storage.Config{
			Name:   "primary",
			Driver: "bun",
			DSN:    "postgres://primary",
		},
	}

	_, err := svc.PreviewProfile(context.Background(), profile)
	if !errors.Is(err, ErrPreviewUnsupported) {
		t.Fatalf("expected ErrPreviewUnsupported, got %v", err)
	}
}

func TestService_PreviewProfileSuccess(t *testing.T) {
	repo := storageconfig.NewMemoryRepository()
	invoked := false
	svc := NewService(repo, nil, WithPreviewer(func(ctx context.Context, profile storage.Profile) (PreviewResult, error) {
		invoked = true
		profile.Config.Name = "previewed"
		return PreviewResult{
			Profile: profile,
			Diagnostics: map[string]any{
				"verified": true,
			},
		}, nil
	}))

	profile := storage.Profile{
		Name:     "primary",
		Provider: "bun",
		Config: storage.Config{
			Name:   "primary",
			Driver: "bun",
			DSN:    "postgres://primary",
		},
	}

	result, err := svc.PreviewProfile(context.Background(), profile)
	if err != nil {
		t.Fatalf("PreviewProfile() error = %v", err)
	}
	if !invoked {
		t.Fatalf("expected previewer to be invoked")
	}
	if result.Profile.Config.Name != "previewed" {
		t.Fatalf("expected preview result to reflect preview mutation, got %q", result.Profile.Config.Name)
	}
	if verified, ok := result.Diagnostics["verified"]; !ok || verified != true {
		t.Fatalf("expected diagnostics to include verification flag, got %v", result.Diagnostics)
	}
	if profile.Config.Name != "primary" {
		t.Fatalf("expected original profile unchanged, got %q", profile.Config.Name)
	}
}

func TestService_Schemas(t *testing.T) {
	svc := NewService(storageconfig.NewMemoryRepository(), nil)
	schemas := svc.Schemas()
	if schemas.Config != storage.ConfigJSONSchema {
		t.Fatalf("unexpected config schema")
	}
	if schemas.Profile != storage.ProfileJSONSchema {
		t.Fatalf("unexpected profile schema")
	}
}
