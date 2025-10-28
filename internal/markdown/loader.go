package markdown

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// LoaderConfig configures how Markdown files are discovered within a base directory.
type LoaderConfig struct {
	// BasePath is the root directory where Markdown documents live.
	BasePath string
	// DefaultLocale is used when no locale can be inferred from the file path.
	DefaultLocale string
	// Locales enumerates the known locales (e.g. ["en", "es"]) for quick directory matching.
	Locales []string
	// LocalePatterns maps locale identifiers to glob expressions relative to BasePath.
	LocalePatterns map[string]string
	// Pattern limits discovered files to those matching the supplied glob (defaults to "*.md").
	Pattern string
	// Recursive controls whether sub-directories are traversed.
	Recursive bool
}

// Loader turns filesystem paths into Markdown documents with metadata.
type Loader struct {
	fs             fs.FS
	basePath       string
	defaultLocale  string
	locales        []string
	localePatterns map[string]string
	pattern        string
	recursive      bool
}

// NewLoader constructs a Loader using the provided filesystem and configuration.
func NewLoader(filesystem fs.FS, cfg LoaderConfig) *Loader {
	pattern := cfg.Pattern
	if strings.TrimSpace(pattern) == "" {
		pattern = "*.md"
	}

	return &Loader{
		fs:             filesystem,
		basePath:       filepath.Clean(cfg.BasePath),
		defaultLocale:  cfg.DefaultLocale,
		locales:        append([]string(nil), cfg.Locales...),
		localePatterns: cloneStringMap(cfg.LocalePatterns),
		pattern:        pattern,
		recursive:      cfg.Recursive,
	}
}

// LoadFile reads and parses a single Markdown document.
func (l *Loader) LoadFile(ctx context.Context, path string, opts LoadParams) (*DocumentResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	rel, err := l.makeRelative(path)
	if err != nil {
		return nil, err
	}
	rel = filepath.ToSlash(rel)

	data, err := fs.ReadFile(l.fs, rel)
	if err != nil {
		return nil, fmt.Errorf("markdown loader read %s: %w", rel, err)
	}

	info, err := fs.Stat(l.fs, rel)
	if err != nil {
		return nil, fmt.Errorf("markdown loader stat %s: %w", rel, err)
	}

	doc, err := BuildDocument(rel, l.detectLocale(rel, opts.LocalePatterns), data, info.ModTime())
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	doc.Checksum = sum[:]

	return &DocumentResult{
		Document: doc,
		Source:   data,
	}, nil
}

// LoadDirectory discovers Markdown files under dir and returns parsed documents.
func (l *Loader) LoadDirectory(ctx context.Context, dir string, opts LoadParams) ([]*DocumentResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	root, err := l.makeRelative(dir)
	if err != nil {
		return nil, err
	}

	root = filepath.Clean(root)
	if root == "." {
		root = "."
	}

	var results []*DocumentResult

	walkErr := fs.WalkDir(l.fs, root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			if !l.shouldRecurse(root, path, opts.Recursive) {
				return fs.SkipDir
			}
			return nil
		}

		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		rel := filepath.ToSlash(path)
		if !l.matchesPattern(rel, opts.Pattern) {
			return nil
		}

		result, err := l.LoadFile(ctx, rel, opts)
		if err != nil {
			return err
		}
		results = append(results, result)
		return nil
	})

	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Document.FilePath < results[j].Document.FilePath
	})

	return results, nil
}

func (l *Loader) shouldRecurse(root, current string, override *bool) bool {
	recursive := l.recursive
	if override != nil {
		recursive = *override
	}
	if recursive {
		return true
	}
	// If recursion is disabled, only walk the root directory.
	cleanRoot := filepath.Clean(root)
	cleanCurrent := filepath.Clean(current)
	return cleanRoot == cleanCurrent
}

func (l *Loader) matchesPattern(path string, override string) bool {
	pattern := override
	if strings.TrimSpace(pattern) == "" {
		pattern = l.pattern
	}
	// Normalise to slash as fs.WalkDir returns slash-separated paths for DirFS.
	pattern = filepath.ToSlash(pattern)
	if strings.Contains(pattern, "**") {
		// Basic support for ** by stripping repeated separators.
		pattern = strings.ReplaceAll(pattern, "**/", "")
	}
	var target string
	if strings.Contains(pattern, "/") {
		target = path
	} else {
		target = filepath.Base(path)
	}
	match, err := filepath.Match(pattern, target)
	if err != nil {
		return false
	}
	return match
}

func (l *Loader) detectLocale(path string, overrides map[string]string) string {
	path = filepath.ToSlash(path)

	if locale := matchLocalePattern(path, overrides); locale != "" {
		return locale
	}
	if locale := matchLocalePattern(path, l.localePatterns); locale != "" {
		return locale
	}

	segments := strings.Split(path, "/")
	if len(segments) > 0 {
		first := segments[0]
		for _, locale := range l.locales {
			if first == locale {
				return locale
			}
		}
	}

	if l.defaultLocale != "" {
		return l.defaultLocale
	}
	return ""
}

func matchLocalePattern(path string, patterns map[string]string) string {
	for locale, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		pattern = filepath.ToSlash(pattern)
		if strings.Contains(pattern, "**") {
			pattern = strings.ReplaceAll(pattern, "**/", "")
		}
		match, err := filepath.Match(pattern, path)
		if err != nil {
			continue
		}
		if match {
			return locale
		}
	}
	return ""
}

func (l *Loader) makeRelative(path string) (string, error) {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return clean, nil
	}
	if l.basePath == "" {
		return "", fmt.Errorf("markdown loader: absolute path %s provided without base path", path)
	}
	rel, err := filepath.Rel(l.basePath, clean)
	if err != nil {
		return "", fmt.Errorf("markdown loader: make relative %s: %w", path, err)
	}
	return rel, nil
}

// DocumentResult carries the parsed document along with the raw source.
type DocumentResult struct {
	Document *interfaces.Document
	Source   []byte
}

// LoadParams provide call-specific overrides for locale detection and pattern matching.
type LoadParams struct {
	Pattern        string
	LocalePatterns map[string]string
	Recursive      *bool
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
