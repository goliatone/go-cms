package environments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceCreateEnvironment(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	id := uuid.MustParse("00000000-0000-0000-0000-00000000a001")

	svc := NewService(repo,
		WithIDDeriver(func(key string) uuid.UUID { return id }),
		WithNow(func() time.Time { return now }),
	)

	env, err := svc.CreateEnvironment(ctx, CreateEnvironmentInput{
		Key:         "staging",
		Name:        "Staging",
		Description: stringPtr("Staging env"),
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	if env.ID != id {
		t.Fatalf("expected id %s, got %s", id, env.ID)
	}
	if env.Key != "staging" {
		t.Fatalf("expected key staging, got %s", env.Key)
	}
	if env.Name != "Staging" {
		t.Fatalf("expected name Staging, got %s", env.Name)
	}
	if env.Description == nil || *env.Description != "Staging env" {
		t.Fatalf("expected description")
	}
	if !env.IsActive {
		t.Fatalf("expected active environment")
	}
	if !env.IsDefault {
		t.Fatalf("expected default environment")
	}
	if !env.CreatedAt.Equal(now) || !env.UpdatedAt.Equal(now) {
		t.Fatalf("unexpected timestamps")
	}
}

func TestServiceCreateEnvironmentDerivesName(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	svc := NewService(repo)

	env, err := svc.CreateEnvironment(ctx, CreateEnvironmentInput{Key: "dev"})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	if env.Name != "Dev" {
		t.Fatalf("expected derived name Dev, got %s", env.Name)
	}
}

func TestServiceCreateEnvironmentRejectsDuplicateKey(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	svc := NewService(repo)

	if _, err := svc.CreateEnvironment(ctx, CreateEnvironmentInput{Key: "prod"}); err != nil {
		t.Fatalf("create environment: %v", err)
	}
	if _, err := svc.CreateEnvironment(ctx, CreateEnvironmentInput{Key: "prod"}); err != ErrEnvironmentKeyExists {
		t.Fatalf("expected key exists error, got %v", err)
	}
}

func TestServiceUpdateEnvironmentDefault(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()

	idSequence := []uuid.UUID{
		uuid.MustParse("00000000-0000-0000-0000-00000000b001"),
		uuid.MustParse("00000000-0000-0000-0000-00000000c001"),
	}
	idx := 0
	svc := NewService(repo, WithIDDeriver(func(key string) uuid.UUID {
		id := idSequence[idx]
		idx++
		return id
	}))

	first, err := svc.CreateEnvironment(ctx, CreateEnvironmentInput{Key: "dev", IsDefault: true})
	if err != nil {
		t.Fatalf("create dev: %v", err)
	}
	second, err := svc.CreateEnvironment(ctx, CreateEnvironmentInput{Key: "prod"})
	if err != nil {
		t.Fatalf("create prod: %v", err)
	}

	updated, err := svc.UpdateEnvironment(ctx, UpdateEnvironmentInput{ID: second.ID, IsDefault: boolPtr(true)})
	if err != nil {
		t.Fatalf("update default: %v", err)
	}
	if !updated.IsDefault {
		t.Fatalf("expected prod to be default")
	}

	firstReloaded, err := svc.GetEnvironment(ctx, first.ID)
	if err != nil {
		t.Fatalf("get dev: %v", err)
	}
	if firstReloaded.IsDefault {
		t.Fatalf("expected dev to be non-default")
	}
}

func TestServiceDeleteEnvironment(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	svc := NewService(repo)

	env, err := svc.CreateEnvironment(ctx, CreateEnvironmentInput{Key: "qa"})
	if err != nil {
		t.Fatalf("create qa: %v", err)
	}
	if err := svc.DeleteEnvironment(ctx, env.ID); err != nil {
		t.Fatalf("delete env: %v", err)
	}
	if _, err := svc.GetEnvironment(ctx, env.ID); !errors.Is(err, ErrEnvironmentNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}
