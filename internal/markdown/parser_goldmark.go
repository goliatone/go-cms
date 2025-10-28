package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// GoldmarkParser implements interfaces.MarkdownParser using the goldmark engine.
// The parser is intentionally stateless so callers can reuse a single instance
// across requests without additional locking.
type GoldmarkParser struct {
	defaultOptions interfaces.ParseOptions
}

// NewGoldmarkParser constructs a parser with sensible defaults (GFM extensions,
// hard wraps disabled, unsafe HTML allowed). Callers can override behaviour per
// invocation through ParseWithOptions to stay aligned with Option 1's flexible
// configuration goals.
func NewGoldmarkParser(defaults interfaces.ParseOptions) *GoldmarkParser {
	return &GoldmarkParser{
		defaultOptions: defaults,
	}
}

// Parse satisfies interfaces.MarkdownParser by rendering Markdown into HTML
// using the parser's default configuration.
func (p *GoldmarkParser) Parse(markdown []byte) ([]byte, error) {
	return p.ParseWithOptions(markdown, p.defaultOptions)
}

// ParseWithOptions renders Markdown into HTML using the provided options.
// Future phases may replace the per-call goldmark instantiation with a pool to
// reduce allocations once performance measurements are available (see TODO).
func (p *GoldmarkParser) ParseWithOptions(markdown []byte, opts interfaces.ParseOptions) ([]byte, error) {
	engine := newGoldmarkEngine(opts)
	var buf bytes.Buffer
	if err := engine.Convert(markdown, &buf); err != nil {
		return nil, fmt.Errorf("markdown parse: %w", err)
	}
	return buf.Bytes(), nil
}

// newGoldmarkEngine builds a goldmark.Markdown configured based on the supplied
// parse options. The mapping is intentionally conservative; unsupported
// extension names are ignored.
func newGoldmarkEngine(opts interfaces.ParseOptions) goldmark.Markdown {
	exts := collectExtensions(opts.Extensions)

	parserOptions := []parser.Option{
		parser.WithAutoHeadingID(),
	}

	rendererOptions := []renderer.Option{}

	if opts.HardWraps {
		rendererOptions = append(rendererOptions, html.WithHardWraps())
	}

	// Treat both SafeMode and Sanitize as signals to avoid emitting raw HTML.
	// TODO: introduce a dedicated sanitiser to scrub HTML when Sanitize is true.
	if !opts.SafeMode && !opts.Sanitize {
		rendererOptions = append(rendererOptions, html.WithUnsafe())
	}

	engineOptions := []goldmark.Option{
		goldmark.WithParserOptions(parserOptions...),
	}

	if len(rendererOptions) > 0 {
		engineOptions = append(engineOptions, goldmark.WithRendererOptions(rendererOptions...))
	}

	if len(exts) > 0 {
		engineOptions = append(engineOptions, goldmark.WithExtensions(exts...))
	}

	return goldmark.New(engineOptions...)
}

var extensionRegistry = map[string]goldmark.Extender{
	"gfm":           extension.GFM,
	"table":         extension.Table,
	"tables":        extension.Table,
	"strikethrough": extension.Strikethrough,
	"linkify":       extension.Linkify,
	"autolink":      extension.Linkify,
	"tasklist":      extension.TaskList,
	"definition":    extension.DefinitionList,
	"footnote":      extension.Footnote,
}

func collectExtensions(names []string) []goldmark.Extender {
	if len(names) == 0 {
		return []goldmark.Extender{
			extension.GFM,
			extension.Linkify,
			extension.TaskList,
		}
	}

	var extenders []goldmark.Extender
	seen := map[string]struct{}{}

	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}

		if _, ok := seen[key]; ok {
			continue
		}

		ext, ok := extensionRegistry[key]
		if !ok {
			continue
		}

		extenders = append(extenders, ext)
		seen[key] = struct{}{}
	}

	return extenders
}
