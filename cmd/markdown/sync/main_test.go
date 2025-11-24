package main

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms/cmd/markdown/internal/bootstrap"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

type stubMarkdownSyncService struct {
	syncCalls int
	syncDir   string
}

func (s *stubMarkdownSyncService) Load(context.Context, string, interfaces.LoadOptions) (*interfaces.Document, error) {
	return nil, nil
}

func (s *stubMarkdownSyncService) LoadDirectory(context.Context, string, interfaces.LoadOptions) ([]*interfaces.Document, error) {
	return nil, nil
}

func (s *stubMarkdownSyncService) Render(context.Context, []byte, interfaces.ParseOptions) ([]byte, error) {
	return nil, nil
}

func (s *stubMarkdownSyncService) RenderDocument(context.Context, *interfaces.Document, interfaces.ParseOptions) ([]byte, error) {
	return nil, nil
}

func (s *stubMarkdownSyncService) Import(context.Context, *interfaces.Document, interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	return nil, nil
}

func (s *stubMarkdownSyncService) ImportDirectory(context.Context, string, interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	return nil, nil
}

func (s *stubMarkdownSyncService) Sync(_ context.Context, dir string, _ interfaces.SyncOptions) (*interfaces.SyncResult, error) {
	s.syncCalls++
	s.syncDir = dir
	return &interfaces.SyncResult{}, nil
}

func TestRunSyncUsesCommandHandler(t *testing.T) {
	original := moduleBuilder
	defer func() { moduleBuilder = original }()

	svc := &stubMarkdownSyncService{}
	moduleBuilder = func(bootstrap.Options) (*bootstrap.Module, error) {
		return &bootstrap.Module{
			Service: svc,
			Logger:  logging.NoOp(),
		}, nil
	}

	contentType := uuid.New().String()
	if err := runSync([]string{
		"-directory", "docs",
		"-content-type", contentType,
	}); err != nil {
		t.Fatalf("runSync returned error: %v", err)
	}
	if svc.syncCalls != 1 {
		t.Fatalf("expected sync to be called once, got %d", svc.syncCalls)
	}
	if svc.syncDir != "docs" {
		t.Fatalf("expected sync directory docs, got %s", svc.syncDir)
	}
}
