package genutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

const (
	opEnsureDir = "generator.ensure_dir"
	opWrite     = "generator.write"
	opRead      = "generator.read"
	opRemove    = "generator.remove"
)

// NewGoTemplateRenderer returns a generator-compatible template renderer backed by html/template.
func NewGoTemplateRenderer(baseDir string) (interfaces.TemplateRenderer, error) {
	info, err := os.Stat(baseDir)
	if err != nil {
		return nil, fmt.Errorf("inspect template directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("template path %q is not a directory", baseDir)
	}
	return &goTemplateRenderer{baseDir: baseDir}, nil
}

// NewFilesystemStorage returns an interfaces.StorageProvider that writes artifacts to disk.
// The base argument should match the generator OutputDir so duplicated prefixes are trimmed.
func NewFilesystemStorage(root, base string) interfaces.StorageProvider {
	base = filepath.ToSlash(filepath.Clean(base))
	return &filesystemStorage{root: root, base: base}
}

// NewThemeAssetResolver returns an AssetResolver that resolves theme-relative files.
func NewThemeAssetResolver() generator.AssetResolver {
	return &themeAssetResolver{}
}

type goTemplateRenderer struct {
	baseDir string
	once    sync.Once
	tpl     *template.Template
	err     error
}

func (r *goTemplateRenderer) ensureTemplates() (*template.Template, error) {
	r.once.Do(func() {
		var files []string
		err := filepath.WalkDir(r.baseDir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".html" && ext != ".tmpl" {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			r.err = err
			return
		}
		if len(files) == 0 {
			r.err = fmt.Errorf("no templates found in %s", r.baseDir)
			return
		}

		funcs := template.FuncMap{
			"safeHTML": func(value any) template.HTML { return toHTML(value) },
			"blockContent": func(inst *blocks.Instance, localeID uuid.UUID) map[string]any {
				if inst == nil {
					return map[string]any{}
				}
				for _, tr := range inst.Translations {
					if tr == nil || tr.LocaleID != localeID {
						continue
					}
					cloned := make(map[string]any, len(tr.Content))
					for k, v := range tr.Content {
						cloned[k] = v
					}
					return cloned
				}
				return map[string]any{}
			},
			"blockName": func(inst *blocks.Instance) string {
				if inst != nil && inst.Definition != nil {
					return inst.Definition.Name
				}
				return ""
			},
			"widgetConfig": func(resolved *widgets.ResolvedWidget, localeID uuid.UUID) map[string]any {
				merged := map[string]any{}
				if resolved == nil || resolved.Instance == nil {
					return merged
				}
				for k, v := range resolved.Instance.Configuration {
					merged[k] = v
				}
				for _, tr := range resolved.Instance.Translations {
					if tr == nil || tr.LocaleID != localeID {
						continue
					}
					for k, v := range tr.Content {
						merged[k] = v
					}
					break
				}
				return merged
			},
			"widgetName": func(resolved *widgets.ResolvedWidget) string {
				if resolved != nil && resolved.Instance != nil && resolved.Instance.Definition != nil {
					return resolved.Instance.Definition.Name
				}
				return ""
			},
		}
		r.tpl, r.err = template.New("generator-theme").Funcs(funcs).ParseFiles(files...)
	})
	return r.tpl, r.err
}

func (r *goTemplateRenderer) Render(name string, data any, out ...io.Writer) (string, error) {
	return r.RenderTemplate(name, data, out...)
}

func (r *goTemplateRenderer) RenderTemplate(name string, data any, out ...io.Writer) (string, error) {
	tpl, err := r.ensureTemplates()
	if err != nil {
		return "", err
	}
	if tpl.Lookup(name) == nil {
		return "", fmt.Errorf("template %q not found", name)
	}

	var writer io.Writer
	var buffer *bytes.Buffer
	if len(out) > 0 && out[0] != nil {
		writer = out[0]
	} else {
		buffer = &bytes.Buffer{}
		writer = buffer
	}

	if err := tpl.ExecuteTemplate(writer, name, data); err != nil {
		return "", err
	}
	if buffer != nil {
		return buffer.String(), nil
	}
	return "", nil
}

func (r *goTemplateRenderer) RenderString(content string, data any, out ...io.Writer) (string, error) {
	tpl, err := template.New("inline").Funcs(template.FuncMap{
		"safeHTML": func(value any) template.HTML { return toHTML(value) },
	}).Parse(content)
	if err != nil {
		return "", err
	}

	var writer io.Writer
	var buffer *bytes.Buffer
	if len(out) > 0 && out[0] != nil {
		writer = out[0]
	} else {
		buffer = &bytes.Buffer{}
		writer = buffer
	}

	if err := tpl.Execute(writer, data); err != nil {
		return "", err
	}
	if buffer != nil {
		return buffer.String(), nil
	}
	return "", nil
}

func (r *goTemplateRenderer) RegisterFilter(string, func(any, any) (any, error)) error {
	return nil
}

func (r *goTemplateRenderer) GlobalContext(any) error {
	return nil
}

func toHTML(value any) template.HTML {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case template.HTML:
		return v
	case string:
		return template.HTML(v)
	default:
		return template.HTML(fmt.Sprint(v))
	}
}

type filesystemStorage struct {
	root string
	base string
}

func (s *filesystemStorage) Query(_ context.Context, query string, args ...any) (interfaces.Rows, error) {
	if query != opRead || len(args) == 0 {
		return nil, nil
	}
	target := s.normalizePath(args[0])
	data, err := os.ReadFile(s.abs(target))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &fileRows{data: data}, nil
}

func (s *filesystemStorage) Exec(_ context.Context, query string, args ...any) (interfaces.Result, error) {
	switch query {
	case opEnsureDir:
		if len(args) == 0 {
			return emptyResult{}, fmt.Errorf("ensure_dir requires path")
		}
		path := s.normalizePath(args[0])
		return emptyResult{}, os.MkdirAll(s.abs(path), 0o755)
	case opWrite:
		if len(args) < 2 {
			return emptyResult{}, fmt.Errorf("write requires path and reader")
		}
		path := s.normalizePath(args[0])
		reader, ok := args[1].(io.Reader)
		if !ok || reader == nil {
			return emptyResult{}, fmt.Errorf("write expects io.Reader content")
		}
		full := s.abs(path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return emptyResult{}, err
		}
		file, err := os.Create(full)
		if err != nil {
			return emptyResult{}, err
		}
		defer file.Close()
		if _, err := io.Copy(file, reader); err != nil {
			return emptyResult{}, err
		}
		return emptyResult{}, nil
	case opRemove:
		if len(args) == 0 {
			return emptyResult{}, fmt.Errorf("remove requires path")
		}
		path := s.normalizePath(args[0])
		err := os.RemoveAll(s.abs(path))
		if errors.Is(err, os.ErrNotExist) {
			return emptyResult{}, nil
		}
		return emptyResult{}, err
	default:
		return emptyResult{}, nil
	}
}

func (s *filesystemStorage) Transaction(ctx context.Context, fn func(tx interfaces.Transaction) error) error {
	if fn == nil {
		return nil
	}
	return fn(&filesystemTx{storage: s})
}

func (s *filesystemStorage) abs(rel string) string {
	if rel == "" {
		return s.root
	}
	return filepath.Join(s.root, filepath.FromSlash(rel))
}

func (s *filesystemStorage) normalizePath(arg any) string {
	path, _ := arg.(string)
	path = filepath.ToSlash(filepath.Clean(path))
	if s.base != "" && strings.HasPrefix(path, s.base) {
		path = strings.TrimPrefix(path, s.base)
		path = strings.TrimPrefix(path, "/")
	}
	return path
}

type filesystemTx struct {
	storage *filesystemStorage
}

func (tx *filesystemTx) Query(ctx context.Context, query string, args ...any) (interfaces.Rows, error) {
	return tx.storage.Query(ctx, query, args...)
}

func (tx *filesystemTx) Exec(ctx context.Context, query string, args ...any) (interfaces.Result, error) {
	return tx.storage.Exec(ctx, query, args...)
}

func (tx *filesystemTx) Transaction(context.Context, func(interfaces.Transaction) error) error {
	return errors.New("nested transactions not supported")
}

func (tx *filesystemTx) Commit() error {
	return nil
}

func (tx *filesystemTx) Rollback() error {
	return nil
}

type emptyResult struct{}

func (emptyResult) RowsAffected() (int64, error) { return 0, nil }
func (emptyResult) LastInsertId() (int64, error) { return 0, nil }

type fileRows struct {
	data []byte
	read bool
}

func (r *fileRows) Next() bool {
	if r.read {
		return false
	}
	r.read = true
	return true
}

func (r *fileRows) Scan(dest ...any) error {
	if len(dest) == 0 {
		return fmt.Errorf("scan requires destination")
	}
	bytesDest, ok := dest[0].(*[]byte)
	if !ok {
		return fmt.Errorf("unsupported scan destination %T", dest[0])
	}
	*bytesDest = append((*bytesDest)[:0], r.data...)
	return nil
}

func (r *fileRows) Close() error {
	return nil
}

type themeAssetResolver struct{}

func (themeAssetResolver) Open(_ context.Context, theme *themes.Theme, asset string) (io.ReadCloser, error) {
	if theme == nil {
		return nil, fmt.Errorf("theme required")
	}
	full := filepath.Join(theme.ThemePath, filepath.FromSlash(asset))
	return os.Open(full)
}

func (themeAssetResolver) ResolvePath(_ *themes.Theme, asset string) (string, error) {
	return filepath.ToSlash(asset), nil
}
