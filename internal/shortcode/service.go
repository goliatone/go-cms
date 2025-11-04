package shortcode

import (
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/logging"
	parserpkg "github.com/goliatone/go-cms/internal/shortcode/parser"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

// placeholderFormat is the marker emitted by the parser when extracting shortcodes.
const placeholderFormat = "<!-- shortcode:%d -->"

// Service orchestrates shortcode parsing and rendering for arbitrary content.
type Service struct {
	registry         interfaces.ShortcodeRegistry
	renderer         interfaces.ShortcodeRenderer
	parser           interfaces.ShortcodeParser
	preprocessor     *parserpkg.WordPressPreprocessor
	defaultSanitizer interfaces.ShortcodeSanitizer
	defaultCache     interfaces.CacheProvider
	logger           interfaces.Logger
	metrics          interfaces.ShortcodeMetrics
	wordpressEnabled bool
}

// ServiceOption customises service behaviour.
type ServiceOption func(*Service)

// WithWordPressSyntax toggles support for the WordPress-style [] shortcode syntax.
func WithWordPressSyntax(enabled bool) ServiceOption {
	return func(s *Service) {
		s.wordpressEnabled = enabled
	}
}

// WithDefaultSanitizer overrides the fallback sanitizer used when none is supplied at call time.
func WithDefaultSanitizer(sanitizer interfaces.ShortcodeSanitizer) ServiceOption {
	return func(s *Service) {
		if sanitizer != nil {
			s.defaultSanitizer = sanitizer
		}
	}
}

// WithDefaultCache overrides the fallback cache provider used when none is supplied at call time.
func WithDefaultCache(cache interfaces.CacheProvider) ServiceOption {
	return func(s *Service) {
		if cache != nil {
			s.defaultCache = cache
		}
	}
}

// WithLogger attaches a logger used for structured diagnostics.
func WithLogger(logger interfaces.Logger) ServiceOption {
	return func(s *Service) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// WithMetrics wires the metrics recorder used for telemetry.
func WithMetrics(metrics interfaces.ShortcodeMetrics) ServiceOption {
	return func(s *Service) {
		if metrics != nil {
			s.metrics = metrics
		}
	}
}

// WithWordPressPreprocessor allows callers to supply a custom WordPress preprocessor.
func WithWordPressPreprocessor(pre *parserpkg.WordPressPreprocessor) ServiceOption {
	return func(s *Service) {
		if pre != nil {
			s.preprocessor = pre
		}
	}
}

// WithParser overrides the Hugo-style parser used to extract shortcodes.
func WithParser(parser interfaces.ShortcodeParser) ServiceOption {
	return func(s *Service) {
		if parser != nil {
			s.parser = parser
		}
	}
}

// NewService constructs a shortcode service using the supplied registry and renderer.
func NewService(registry interfaces.ShortcodeRegistry, renderer interfaces.ShortcodeRenderer, opts ...ServiceOption) *Service {
	service := &Service{
		registry:         registry,
		renderer:         renderer,
		parser:           parserpkg.NewHugoParser(),
		preprocessor:     parserpkg.NewWordPressPreprocessor(),
		defaultSanitizer: NewSanitizer(),
		logger:           logging.NoOp(),
		metrics:          NoOpMetrics(),
	}

	for _, opt := range opts {
		opt(service)
	}
	return service
}

// Process renders any shortcodes found within the content string, returning the resulting HTML.
func (s *Service) Process(ctx context.Context, content string, opts interfaces.ShortcodeProcessOptions) (string, error) {
	if strings.TrimSpace(content) == "" {
		return content, nil
	}
	if s.renderer == nil || s.parser == nil {
		return "", fmt.Errorf("shortcode: service not initialised")
	}

	logger := logging.WithFields(s.baseLogger(ctx), map[string]any{
		"operation": "shortcode.process",
	})

	material := content
	if (s.wordpressEnabled || opts.EnableWordPress) && s.preprocessor != nil {
		material = s.preprocessor.Process(material)
	}

	transformed, parsed, err := s.parser.Extract(material)
	if err != nil {
		logging.WithFields(logger, map[string]any{
			"error": err,
		}).Error("shortcode.service.parse_failed")
		return "", err
	}
	if len(parsed) == 0 {
		return transformed, nil
	}

	shortcodeCtx := interfaces.ShortcodeContext{
		Context:   ctx,
		Locale:    opts.Locale,
		Cache:     opts.Cache,
		Sanitizer: opts.Sanitizer,
	}
	if shortcodeCtx.Context == nil {
		shortcodeCtx.Context = context.Background()
	}
	if shortcodeCtx.Sanitizer == nil {
		shortcodeCtx.Sanitizer = s.defaultSanitizer
	}
	if shortcodeCtx.Cache == nil {
		shortcodeCtx.Cache = s.defaultCache
	}

	output := transformed
	for idx, sc := range parsed {
		start := time.Now()
		rendered, err := s.renderer.Render(shortcodeCtx, sc.Name, sc.Params, sc.Inner)
		elapsed := time.Since(start)
		s.metrics.ObserveRenderDuration(sc.Name, elapsed)

		entryFields := map[string]any{
			"shortcode":   sc.Name,
			"index":       idx,
			"duration_ms": elapsed.Milliseconds(),
		}
		if err != nil {
			s.metrics.IncrementRenderError(sc.Name)
			entryFields["error"] = err
			logging.WithFields(logger, entryFields).Error("shortcode.service.render_failed")
			return "", err
		}
		logging.WithFields(logger, entryFields).Debug("shortcode.service.render_succeeded")

		placeholder := fmt.Sprintf(placeholderFormat, idx)
		output = strings.ReplaceAll(output, placeholder, string(rendered))
	}

	logging.WithFields(logger, map[string]any{
		"shortcodes": len(parsed),
	}).Debug("shortcode.service.process_completed")
	return output, nil
}

// Render executes a single shortcode definition and returns the HTML output.
func (s *Service) Render(ctx interfaces.ShortcodeContext, shortcode string, params map[string]any, inner string) (template.HTML, error) {
	if s.renderer == nil {
		return "", fmt.Errorf("shortcode: service not initialised")
	}
	if ctx.Context == nil {
		ctx.Context = context.Background()
	}
	if ctx.Sanitizer == nil {
		ctx.Sanitizer = s.defaultSanitizer
	}
	if ctx.Cache == nil {
		ctx.Cache = s.defaultCache
	}

	logger := logging.WithFields(s.baseLogger(ctx.Context), map[string]any{
		"operation": "shortcode.render",
		"shortcode": shortcode,
	})

	start := time.Now()
	result, err := s.renderer.Render(ctx, shortcode, params, inner)
	elapsed := time.Since(start)
	s.metrics.ObserveRenderDuration(shortcode, elapsed)

	fields := map[string]any{
		"duration_ms": elapsed.Milliseconds(),
	}
	if err != nil {
		s.metrics.IncrementRenderError(shortcode)
		fields["error"] = err
		logging.WithFields(logger, fields).Error("shortcode.service.render_failed")
		return "", err
	}
	logging.WithFields(logger, fields).Debug("shortcode.service.render_succeeded")

	return result, nil
}

// Registry exposes the underlying shortcode registry.
func (s *Service) Registry() interfaces.ShortcodeRegistry {
	return s.registry
}

// Ensure Service complies with interfaces.ShortcodeService.
var _ interfaces.ShortcodeService = (*Service)(nil)

type noOpService struct{}

// NewNoOpService returns a shortcode service that leaves content untouched.
func NewNoOpService() interfaces.ShortcodeService {
	return noOpService{}
}

func (noOpService) Process(_ context.Context, content string, _ interfaces.ShortcodeProcessOptions) (string, error) {
	return content, nil
}

func (noOpService) Render(_ interfaces.ShortcodeContext, _ string, _ map[string]any, _ string) (template.HTML, error) {
	return template.HTML(""), nil
}

func (s *Service) baseLogger(ctx context.Context) interfaces.Logger {
	logger := s.logger
	if logger == nil {
		logger = logging.NoOp()
	}
	if ctx != nil {
		logger = logger.WithContext(ctx)
	}
	return logger
}
