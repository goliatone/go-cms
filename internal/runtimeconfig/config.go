package runtimeconfig

import (
	"errors"
	"fmt"
	"strings"
	"time"

	urlkit "github.com/goliatone/go-urlkit"
)

// ErrThemesFeatureRequired indicates inconsistent theme configuration.
var ErrThemesFeatureRequired = errors.New("cms config: themes feature must be enabled to configure themes")

// ErrSchedulingFeatureRequiresVersioning ensures scheduling stays behind the versioning flag.
var ErrSchedulingFeatureRequiresVersioning = errors.New("cms config: scheduling feature requires versioning to be enabled")

// ErrAdvancedCacheRequiresEnabledCache ensures advanced cache builds only when cache is enabled.
var ErrAdvancedCacheRequiresEnabledCache = errors.New("cms config: advanced cache feature requires cache to be enabled")

// ErrCommandsCronRequiresScheduling ensures automatic cron wiring only runs when scheduling is enabled.
var ErrCommandsCronRequiresScheduling = errors.New("cms config: command cron auto-registration requires scheduling to be enabled")
var ErrMarkdownFeatureRequired = errors.New("cms config: markdown feature must be enabled to configure markdown")
var ErrMarkdownContentDirRequired = errors.New("cms config: markdown content directory is required when markdown is enabled")
var ErrGeneratorOutputDirRequired = errors.New("cms config: generator output directory is required when generator is enabled")
var ErrLoggingProviderRequired = errors.New("cms config: logging provider is required when logging feature is enabled")
var ErrLoggingProviderUnknown = errors.New("cms config: logging provider is invalid")
var ErrLoggingLevelInvalid = errors.New("cms config: logging level is invalid")
var ErrLoggingFormatInvalid = errors.New("cms config: logging format is invalid")
var ErrVersionRetentionLimitInvalid = errors.New("cms config: version retention limit must be zero or positive")

// Config aggregates feature flags and adapter bindings for the CMS module.
// Fields intentionally use simple types so host applications can extend them later.
type Config struct {
	Enabled       bool
	DefaultLocale string
	Content       ContentConfig
	I18N          I18NConfig
	Storage       StorageConfig
	Cache         CacheConfig
	Navigation    NavigationConfig
	Themes        ThemeConfig
	Widgets       WidgetConfig
	Retention     RetentionConfig
	Features      Features
	Commands      CommandsConfig
	Markdown      MarkdownConfig
	Generator     GeneratorConfig
	Logging       LoggingConfig
}

// ContentConfig captures configuration for the core content module.
type ContentConfig struct {
	PageHierarchy bool
}

// I18NConfig wires go-i18n options through the CMS wrapper.
type I18NConfig struct {
	Enabled bool
	Locales []string
}

// StorageConfig lists identifiers for storage-related dependencies.
type StorageConfig struct {
	Provider string
}

// CacheConfig captures cache behaviour toggles.
type CacheConfig struct {
	Enabled    bool
	DefaultTTL time.Duration
}

// NavigationConfig captures routing configuration for menu URL resolution.
type NavigationConfig struct {
	RouteConfig *urlkit.Config
	URLKit      URLKitResolverConfig
}

// ThemeConfig captures configuration for the themes module.
type ThemeConfig struct {
	BasePath     string
	DefaultTheme string
}

// URLKitResolverConfig configures the go-urlkit based resolver.
type URLKitResolverConfig struct {
	DefaultGroup  string
	LocaleGroups  map[string]string
	DefaultRoute  string
	SlugParam     string
	LocaleParam   string
	LocaleIDParam string
	RouteField    string
	ParamsField   string
	QueryField    string
}

// Features toggles module functionality.
type Features struct {
	Widgets       bool
	Themes        bool
	Versioning    bool
	Scheduling    bool
	MediaLibrary  bool
	AdvancedCache bool
	Markdown      bool
	Logger        bool
}

// LoggingConfig captures provider-specific options for runtime logging.
type LoggingConfig struct {
	Provider  string
	Level     string
	Format    string
	AddSource bool
	Focus     []string
}

// WidgetConfig controls registry bootstrapping.
type WidgetConfig struct {
	Definitions []WidgetDefinitionConfig
}

// WidgetDefinitionConfig mirrors the minimal RegisterDefinitionInput requirements.
type WidgetDefinitionConfig struct {
	Name        string
	Description string
	Schema      map[string]any
	Defaults    map[string]any
	Category    string
	Icon        string
}

// RetentionConfig captures per-module version retention limits.
type RetentionConfig struct {
	Content int
	Pages   int
	Blocks  int
}

// CommandsConfig captures optional command-layer behaviour.
type CommandsConfig struct {
	Enabled                bool
	AutoRegisterDispatcher bool
	AutoRegisterCron       bool
	CleanupAuditCron       string
}

// MarkdownConfig captures filesystem and parser behaviour for Markdown ingestion.
type MarkdownConfig struct {
	Enabled        bool
	ContentDir     string
	Pattern        string
	Recursive      bool
	LocalePatterns map[string]string
	DefaultLocale  string
	Locales        []string
	Parser         MarkdownParserConfig
}

// MarkdownParserConfig mirrors interfaces.ParseOptions for runtime configuration.
type MarkdownParserConfig struct {
	Extensions []string
	Sanitize   bool
	HardWraps  bool
	SafeMode   bool
}

// GeneratorConfig captures behaviour for the static site generator.
type GeneratorConfig struct {
	Enabled          bool
	OutputDir        string
	BaseURL          string
	CleanBuild       bool
	Incremental      bool
	CopyAssets       bool
	GenerateSitemap  bool
	GenerateRobots   bool
	GenerateFeeds    bool
	Workers          int
	Menus            map[string]string
	RenderTimeout    time.Duration
	AssetCopyTimeout time.Duration
}

// DefaultConfig returns opinionated defaults matching Phase 1 expectations.
func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		DefaultLocale: "en",
		Content: ContentConfig{
			PageHierarchy: true,
		},
		Retention: RetentionConfig{},
		I18N: I18NConfig{
			Enabled: true,
			Locales: []string{"en"},
		},
		Storage: StorageConfig{
			Provider: "bun",
		},
		Cache: CacheConfig{
			Enabled:    true,
			DefaultTTL: time.Minute,
		},
		Navigation: NavigationConfig{},
		Themes: ThemeConfig{
			BasePath: "themes",
		},
		Widgets: WidgetConfig{
			Definitions: []WidgetDefinitionConfig{
				{
					Name:        "newsletter_signup",
					Description: "Captures visitor email addresses with a simple form",
					Schema: map[string]any{
						"fields": []any{
							map[string]any{"name": "headline", "type": "text"},
							map[string]any{"name": "subheadline", "type": "text"},
							map[string]any{"name": "cta_text", "type": "text"},
							map[string]any{"name": "success_message", "type": "text"},
						},
					},
					Defaults: map[string]any{
						"cta_text":        "Subscribe",
						"success_message": "Thanks for subscribing!",
					},
					Category: "marketing",
					Icon:     "envelope-open",
				},
			},
		},
		Features: Features{},
		Commands: CommandsConfig{},
		Markdown: MarkdownConfig{
			ContentDir:     "content",
			Pattern:        "*.md",
			Recursive:      true,
			LocalePatterns: map[string]string{},
		},
		Generator: GeneratorConfig{
			OutputDir:        "dist",
			CleanBuild:       true,
			Incremental:      false,
			CopyAssets:       true,
			GenerateSitemap:  true,
			GenerateRobots:   false,
			GenerateFeeds:    false,
			Workers:          0,
			Menus:            map[string]string{},
			RenderTimeout:    0,
			AssetCopyTimeout: 0,
		},
		Logging: LoggingConfig{
			Provider: "console",
			Level:    "info",
			Format:   "",
		},
	}
}

// Validate performs high-level consistency checks.
func (cfg Config) Validate() error {
	if !cfg.Features.Themes {
		if strings.TrimSpace(cfg.Themes.DefaultTheme) != "" {
			return ErrThemesFeatureRequired
		}
	}
	if cfg.Features.Scheduling && !cfg.Features.Versioning {
		return ErrSchedulingFeatureRequiresVersioning
	}
	if cfg.Features.AdvancedCache && !cfg.Cache.Enabled {
		return ErrAdvancedCacheRequiresEnabledCache
	}
	if cfg.Commands.AutoRegisterCron && !cfg.Features.Scheduling {
		return ErrCommandsCronRequiresScheduling
	}
	if cfg.Markdown.Enabled {
		if !cfg.Features.Markdown {
			return ErrMarkdownFeatureRequired
		}
		if strings.TrimSpace(cfg.Markdown.ContentDir) == "" {
			return ErrMarkdownContentDirRequired
		}
	}
	if cfg.Generator.Enabled {
		if strings.TrimSpace(cfg.Generator.OutputDir) == "" {
			return ErrGeneratorOutputDirRequired
		}
	}
	if cfg.Retention.Content < 0 {
		return fmt.Errorf("%w: content", ErrVersionRetentionLimitInvalid)
	}
	if cfg.Retention.Pages < 0 {
		return fmt.Errorf("%w: pages", ErrVersionRetentionLimitInvalid)
	}
	if cfg.Retention.Blocks < 0 {
		return fmt.Errorf("%w: blocks", ErrVersionRetentionLimitInvalid)
	}
	if cfg.Features.Logger {
		provider := normalizeProvider(cfg.Logging.Provider)
		if provider == "" {
			return ErrLoggingProviderRequired
		}
		if !isSupportedProvider(provider) {
			return fmt.Errorf("%w: %s", ErrLoggingProviderUnknown, provider)
		}
		if level := strings.TrimSpace(cfg.Logging.Level); level != "" && !isSupportedLevel(level) {
			return fmt.Errorf("%w: %s", ErrLoggingLevelInvalid, level)
		}
		if provider == "gologger" {
			if format := strings.TrimSpace(cfg.Logging.Format); format != "" && !isSupportedFormat(format) {
				return fmt.Errorf("%w: %s", ErrLoggingFormatInvalid, format)
			}
		}
	}
	return nil
}

func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func isSupportedProvider(provider string) bool {
	switch provider {
	case "console", "gologger":
		return true
	default:
		return false
	}
}

func isSupportedLevel(level string) bool {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "trace", "debug", "info", "warn", "warning", "error", "fatal":
		return true
	default:
		return false
	}
}

func isSupportedFormat(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json", "console", "pretty":
		return true
	default:
		return false
	}
}
