package generator

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

const (
	storageOpEnsureDir = "generator.ensure_dir"
	storageOpWrite     = "generator.write"
	storageOpRead      = "generator.read"
	storageOpRemove    = "generator.remove"
)

type writeCategory string

const (
	categoryPage     writeCategory = "page"
	categoryAsset    writeCategory = "asset"
	categorySitemap  writeCategory = "sitemap"
	categoryRobots   writeCategory = "robots"
	categoryManifest writeCategory = "manifest"
)

// writeFileRequest describes a file write operation routed through the artifact writer.
type writeFileRequest struct {
	Path        string
	Content     io.Reader
	Size        int64
	Locale      string
	Category    writeCategory
	ContentType string
	Checksum    string
	Metadata    map[string]string
}

// artifactWriter abstracts storage provider specifics for generator outputs.
type artifactWriter interface {
	EnsureDir(ctx context.Context, path string) error
	WriteFile(ctx context.Context, req writeFileRequest) error
}

func newArtifactWriter(storage interfaces.StorageProvider) artifactWriter {
	if storage == nil {
		return noopWriter{}
	}
	return &storageWriter{storage: storage}
}

type storageWriter struct {
	storage interfaces.StorageProvider
}

func (w *storageWriter) EnsureDir(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" || path == "." {
		return nil
	}
	_, err := w.storage.Exec(ctx, storageOpEnsureDir, path)
	return err
}

func (w *storageWriter) WriteFile(ctx context.Context, req writeFileRequest) error {
	if req.Content == nil {
		return errors.New("generator: write requires content reader")
	}
	if strings.TrimSpace(req.Path) == "" {
		return errors.New("generator: write requires path")
	}
	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	args := []any{
		req.Path,
		req.Content,
		req.Size,
		string(req.Category),
		req.ContentType,
		req.Locale,
		req.Checksum,
		req.Metadata,
	}
	_, err := w.storage.Exec(ctx, storageOpWrite, args...)
	return err
}

type noopWriter struct{}

func (noopWriter) EnsureDir(context.Context, string) error { return nil }

func (noopWriter) WriteFile(context.Context, writeFileRequest) error { return nil }
