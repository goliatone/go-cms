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

	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Config controls how the Markdown service discovers and parses files.
type Config struct {
	BasePath          string
	DefaultLocale     string
	Locales           []string
	LocalePatterns    map[string]string
	Pattern           string
	Recursive         bool
	Parser            interfaces.ParseOptions
	ProcessShortcodes bool
}

// Service implements interfaces.MarkdownService for filesystem-backed documents.
type Service struct {
	cfg        Config
	parser     interfaces.MarkdownParser
	loader     *Loader
	content    interfaces.ContentService
	logger     interfaces.Logger
	importer   *Importer
	shortcodes interfaces.ShortcodeService
}

// ServiceOption configures optional dependencies for the markdown service.
type ServiceOption func(*Service)

// WithContentService wires the content service used during import/sync.
func WithContentService(svc interfaces.ContentService) ServiceOption {
	return func(s *Service) {
		s.content = svc
	}
}

// WithLogger attaches a logger for importer diagnostics.
func WithLogger(logger interfaces.Logger) ServiceOption {
	return func(s *Service) {
		s.logger = logger
	}
}

// WithShortcodeService wires the shortcode renderer used during parsing.
func WithShortcodeService(svc interfaces.ShortcodeService) ServiceOption {
	return func(s *Service) {
		s.shortcodes = svc
	}
}

// NewService constructs a Markdown service using an underlying loader. When parser
// is nil, a Goldmark parser with the provided default options is created.
func NewService(cfg Config, parser interfaces.MarkdownParser, opts ...ServiceOption) (*Service, error) {
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

	svc := &Service{
		cfg:    cfg,
		parser: parser,
		loader: loader,
	}

	for _, opt := range opts {
		opt(svc)
	}

	if svc.logger == nil {
		svc.logger = logging.NoOp()
	}

	svc.importer = NewImporter(ImporterConfig{
		Content: svc.content,
		Logger:  svc.logger,
	})

	return svc, nil
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
	parseOpts := mergeParseOptions(s.cfg.Parser, opts)
	processed, err := s.applyShortcodes(ctx, markdown, parseOpts)
	if err != nil {
		return nil, err
	}
	return s.parser.ParseWithOptions(processed, parseOpts)
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
	if s.importer == nil {
		return nil, errors.New("markdown service: importer not configured")
	}
	if doc == nil {
		logging.WithFields(logging.WithMarkdownContext(s.logger, "", "", "import"), map[string]any{
			"error": "document_nil",
		}).Error("markdown.service.import.failed")
		return nil, errors.New("markdown service: document is nil")
	}
	logger := logging.WithMarkdownContext(s.logger, doc.FilePath, doc.Locale, "import")

	if err := s.renderDocument(ctx, doc, interfaces.ParseOptions{
		ProcessShortcodes: opts.ProcessShortcodes,
		ShortcodeOptions: interfaces.ShortcodeProcessOptions{
			Locale: doc.Locale,
		},
	}); err != nil {
		logging.WithFields(logger, map[string]any{
			"error": err,
		}).Error("markdown.service.import.render_failed")
		return nil, err
	}

	result, err := s.importer.ImportDocument(ctx, doc, opts)
	if err != nil {
		logging.WithFields(logger, map[string]any{
			"error": err,
		}).Error("markdown.service.import.failed")
		return nil, err
	}

	logging.WithFields(logger, map[string]any{
		"created": len(result.CreatedContentIDs),
		"updated": len(result.UpdatedContentIDs),
		"skipped": len(result.SkippedContentIDs),
		"dry_run": opts.DryRun,
	}).Info("markdown.service.import.completed")

	return result, nil
}

func (s *Service) ImportDirectory(ctx context.Context, dir string, opts interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	if s.importer == nil {
		return nil, errors.New("markdown service: importer not configured")
	}

	loadOpts := interfaces.LoadOptions{
		Parser: interfaces.ParseOptions{
			ProcessShortcodes: opts.ProcessShortcodes,
		},
	}
	docs, err := s.LoadDirectory(ctx, dir, loadOpts)
	if err != nil {
		logging.WithFields(logging.WithMarkdownContext(s.logger, dir, "", "import_directory"), map[string]any{
			"error": err,
		}).Error("markdown.service.import_directory.failed")
		return nil, err
	}
	logger := logging.WithMarkdownContext(s.logger, dir, "", "import_directory")
	result, err := s.importer.ImportDocuments(ctx, docs, opts)
	if err != nil {
		logging.WithFields(logger, map[string]any{
			"error": err,
		}).Error("markdown.service.import_directory.failed")
		return nil, err
	}

	logging.WithFields(logger, map[string]any{
		"documents": len(docs),
		"created":   len(result.CreatedContentIDs),
		"updated":   len(result.UpdatedContentIDs),
		"skipped":   len(result.SkippedContentIDs),
		"dry_run":   opts.DryRun,
	}).Info("markdown.service.import_directory.completed")

	return result, nil
}

func (s *Service) Sync(ctx context.Context, dir string, opts interfaces.SyncOptions) (*interfaces.SyncResult, error) {
	if s.importer == nil {
		return nil, errors.New("markdown service: importer not configured")
	}

	loadOpts := interfaces.LoadOptions{
		Parser: interfaces.ParseOptions{
			ProcessShortcodes: opts.ProcessShortcodes,
		},
	}
	docs, err := s.LoadDirectory(ctx, dir, loadOpts)
	if err != nil {
		logging.WithFields(logging.WithMarkdownContext(s.logger, dir, "", "sync_directory"), map[string]any{
			"error": err,
		}).Error("markdown.service.sync.failed")
		return nil, err
	}
	logger := logging.WithMarkdownContext(s.logger, dir, "", "sync_directory")
	result, err := s.importer.SyncDocuments(ctx, docs, opts)
	if err != nil {
		logging.WithFields(logger, map[string]any{
			"error": err,
		}).Error("markdown.service.sync.failed")
		return nil, err
	}

	logging.WithFields(logger, map[string]any{
		"documents":       len(docs),
		"created":         result.Created,
		"updated":         result.Updated,
		"deleted":         result.Deleted,
		"skipped":         result.Skipped,
		"dry_run":         opts.DryRun,
		"delete_orphans":  opts.DeleteOrphaned,
		"update_existing": opts.UpdateExisting,
	}).Info("markdown.service.sync.completed")

	return result, nil
}

func (s *Service) renderDocument(ctx context.Context, doc *interfaces.Document, overrides interfaces.ParseOptions) error {
	if doc == nil {
		return nil
	}
	if strings.TrimSpace(overrides.ShortcodeOptions.Locale) == "" && strings.TrimSpace(doc.Locale) != "" {
		overrides.ShortcodeOptions.Locale = doc.Locale
	}
	html, err := s.Render(ctx, doc.Body, overrides)
	if err != nil {
		return fmt.Errorf("markdown render document %s: %w", doc.FilePath, err)
	}
	doc.BodyHTML = html
	return nil
}

func (s *Service) applyShortcodes(ctx context.Context, content []byte, opts interfaces.ParseOptions) ([]byte, error) {
	if s.shortcodes == nil {
		return content, nil
	}
	if !(s.cfg.ProcessShortcodes || opts.ProcessShortcodes) {
		return content, nil
	}

	shortcodeOpts := opts.ShortcodeOptions
	if strings.TrimSpace(shortcodeOpts.Locale) == "" {
		shortcodeOpts.Locale = s.cfg.DefaultLocale
	}

	rendered, err := s.shortcodes.Process(ctx, string(content), shortcodeOpts)
	if err != nil {
		return nil, err
	}
	return []byte(rendered), nil
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
	if override.ProcessShortcodes {
		result.ProcessShortcodes = true
	}
	result.ShortcodeOptions = mergeShortcodeOptions(result.ShortcodeOptions, override.ShortcodeOptions)
	return result
}

func mergeShortcodeOptions(base, override interfaces.ShortcodeProcessOptions) interfaces.ShortcodeProcessOptions {
	if strings.TrimSpace(override.Locale) != "" {
		base.Locale = override.Locale
	}
	if override.Cache != nil {
		base.Cache = override.Cache
	}
	if override.Sanitizer != nil {
		base.Sanitizer = override.Sanitizer
	}
	if override.EnableWordPress {
		base.EnableWordPress = true
	}
	return base
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
