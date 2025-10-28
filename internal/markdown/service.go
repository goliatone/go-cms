package markdown

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Config controls how the Markdown service discovers and parses files.
type Config struct {
	BasePath       string
	DefaultLocale  string
	Locales        []string
	LocalePatterns map[string]string
	Pattern        string
	Recursive      bool
	Parser         interfaces.ParseOptions
}

// Service implements interfaces.MarkdownService for filesystem-backed documents.
type Service struct {
	cfg    Config
	parser interfaces.MarkdownParser
	loader *Loader
}

// NewService constructs a Markdown service using an underlying loader. When parser
// is nil, a Goldmark parser with the provided default options is created.
func NewService(cfg Config, parser interfaces.MarkdownParser) (*Service, error) {
	filesystem, err := prepareFilesystem(cfg.BasePath)
	if err != nil {
		return nil, err
	}

	if parser == nil {
		parser = NewGoldmarkParser(cfg.Parser)
	}

	loader := NewLoader(filesystem, LoaderConfig{
		BasePath:       cfg.BasePath,
		DefaultLocale:  cfg.DefaultLocale,
		Locales:        cfg.Locales,
		LocalePatterns: cfg.LocalePatterns,
		Pattern:        cfg.Pattern,
		Recursive:      cfg.Recursive,
	})

	return &Service{
		cfg:    cfg,
		parser: parser,
		loader: loader,
	}, nil
}

// Load reads a single Markdown document relative to the configured base path.
func (s *Service) Load(ctx context.Context, path string, opts interfaces.LoadOptions) (*interfaces.Document, error) {
	result, err := s.loader.LoadFile(ctx, s.normalisePath(path), toLoaderParams(opts))
	if err != nil {
		return nil, err
	}
	if err := s.renderDocument(ctx, result.Document, opts.Parser); err != nil {
		return nil, err
	}
	return result.Document, nil
}

// LoadDirectory reads every Markdown document within the supplied directory.
func (s *Service) LoadDirectory(ctx context.Context, dir string, opts interfaces.LoadOptions) ([]*interfaces.Document, error) {
	results, err := s.loader.LoadDirectory(ctx, s.normalisePath(dir), toLoaderParams(opts))
	if err != nil {
		return nil, err
	}

	docs := make([]*interfaces.Document, 0, len(results))
	for _, result := range results {
		if err := s.renderDocument(ctx, result.Document, opts.Parser); err != nil {
			return nil, err
		}
		docs = append(docs, result.Document)
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].FilePath < docs[j].FilePath
	})
	return docs, nil
}

// Render parses Markdown bytes into HTML using the configured parser.
func (s *Service) Render(ctx context.Context, markdown []byte, opts interfaces.ParseOptions) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return s.parser.ParseWithOptions(markdown, mergeParseOptions(s.cfg.Parser, opts))
}

// RenderDocument converts the document's Markdown body into HTML using the configured parser.
func (s *Service) RenderDocument(ctx context.Context, doc *interfaces.Document, opts interfaces.ParseOptions) ([]byte, error) {
	if doc == nil {
		return nil, errors.New("markdown service: document is nil")
	}
	html, err := s.Render(ctx, doc.Body, opts)
	if err != nil {
		return nil, err
	}
	doc.BodyHTML = html
	return html, nil
}

func (s *Service) Import(ctx context.Context, doc *interfaces.Document, opts interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	return nil, errors.New("markdown service: Import not implemented (see Phase 3)")
}

func (s *Service) ImportDirectory(ctx context.Context, dir string, opts interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	return nil, errors.New("markdown service: ImportDirectory not implemented (see Phase 3)")
}

func (s *Service) Sync(ctx context.Context, dir string, opts interfaces.SyncOptions) (*interfaces.SyncResult, error) {
	return nil, errors.New("markdown service: Sync not implemented (see Phase 3)")
}

func (s *Service) renderDocument(ctx context.Context, doc *interfaces.Document, overrides interfaces.ParseOptions) error {
	if doc == nil {
		return nil
	}
	html, err := s.Render(ctx, doc.Body, overrides)
	if err != nil {
		return fmt.Errorf("markdown render document %s: %w", doc.FilePath, err)
	}
	doc.BodyHTML = html
	return nil
}

func (s *Service) normalisePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "."
	}
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) && strings.TrimSpace(s.cfg.BasePath) != "" {
		if rel, err := filepath.Rel(s.cfg.BasePath, clean); err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(clean)
}

func mergeParseOptions(base, override interfaces.ParseOptions) interfaces.ParseOptions {
	result := base
	if len(override.Extensions) > 0 {
		result.Extensions = append([]string(nil), override.Extensions...)
	}
	if override.Sanitize {
		result.Sanitize = true
	}
	if override.HardWraps {
		result.HardWraps = true
	}
	if override.SafeMode {
		result.SafeMode = true
	}
	return result
}

func toLoaderParams(opts interfaces.LoadOptions) LoadParams {
	return LoadParams{
		Pattern:        opts.Pattern,
		LocalePatterns: opts.LocalePatterns,
		Recursive:      opts.Recursive,
	}
}

func prepareFilesystem(basePath string) (fs.FS, error) {
	if strings.TrimSpace(basePath) == "" {
		basePath = "."
	}
	if _, err := os.Stat(basePath); err != nil {
		return nil, fmt.Errorf("markdown service: stat base path %s: %w", basePath, err)
	}
	return os.DirFS(basePath), nil
}
