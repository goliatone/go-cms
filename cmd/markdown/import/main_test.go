package main

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms/cmd/markdown/internal/bootstrap"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

type stubMarkdownService struct {
	importCalls int
	importDir   string
}

func (s *stubMarkdownService) Load(context.Context, string, interfaces.LoadOptions) (*interfaces.Document, error) {
	return nil, nil
}

func (s *stubMarkdownService) LoadDirectory(context.Context, string, interfaces.LoadOptions) ([]*interfaces.Document, error) {
	return nil, nil
}

func (s *stubMarkdownService) Render(context.Context, []byte, interfaces.ParseOptions) ([]byte, error) {
	return nil, nil
}

func (s *stubMarkdownService) RenderDocument(context.Context, *interfaces.Document, interfaces.ParseOptions) ([]byte, error) {
	return nil, nil
}

func (s *stubMarkdownService) Import(context.Context, *interfaces.Document, interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	return nil, nil
}

func (s *stubMarkdownService) ImportDirectory(_ context.Context, dir string, _ interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	s.importCalls++
	s.importDir = dir
	return &interfaces.ImportResult{}, nil
}

func (s *stubMarkdownService) Sync(context.Context, string, interfaces.SyncOptions) (*interfaces.SyncResult, error) {
	return nil, nil
}

func TestRunImportUsesCommandHandler(t *testing.T) {
	original := moduleBuilder
	defer func() { moduleBuilder = original }()

	svc := &stubMarkdownService{}
	moduleBuilder = func(bootstrap.Options) (*bootstrap.Module, error) {
		return &bootstrap.Module{
			Service: svc,
			Logger:  logging.NoOp(),
		}, nil
	}

	contentType := uuid.New().String()
	if err := runImport([]string{
		"-directory", "docs",
		"-content-type", contentType,
	}); err != nil {
		t.Fatalf("runImport returned error: %v", err)
	}
	if svc.importCalls != 1 {
		t.Fatalf("expected import to be called once, got %d", svc.importCalls)
	}
	if svc.importDir != "docs" {
		t.Fatalf("expected import directory docs, got %s", svc.importDir)
	}
}
