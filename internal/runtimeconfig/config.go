package runtimeconfig

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/goliatone/go-cms/pkg/storage"
	urlkit "github.com/goliatone/go-urlkit"
)

// ErrThemesFeatureRequired indicates inconsistent theme configuration.
var ErrThemesFeatureRequired = errors.New("cms config: themes feature must be enabled to configure themes")

// ErrSchedulingFeatureRequiresVersioning ensures scheduling stays behind the versioning flag.
var ErrSchedulingFeatureRequiresVersioning = errors.New("cms config: scheduling feature requires versioning to be enabled")

// ErrAdvancedCacheRequiresEnabledCache ensures advanced cache builds only when cache is enabled.
var ErrAdvancedCacheRequiresEnabledCache = errors.New("cms config: advanced cache feature requires cache to be enabled")

var ErrDefaultLocaleRequired = errors.New("cms config: default locale is required when translations are enforced")
var ErrShortcodesFeatureRequired = errors.New("cms config: shortcodes feature must be enabled to configure shortcodes")
var ErrMarkdownFeatureRequired = errors.New("cms config: markdown feature must be enabled to configure markdown")
var ErrMarkdownContentDirRequired = errors.New("cms config: markdown content directory is required when markdown is enabled")
var ErrGeneratorOutputDirRequired = errors.New("cms config: generator output directory is required when generator is enabled")
var ErrLoggingProviderRequired = errors.New("cms config: logging provider is required when logging feature is enabled")
var ErrLoggingProviderUnknown = errors.New("cms config: logging provider is invalid")
var ErrLoggingLevelInvalid = errors.New("cms config: logging level is invalid")
var ErrLoggingFormatInvalid = errors.New("cms config: logging format is invalid")
var ErrVersionRetentionLimitInvalid = errors.New("cms config: version retention limit must be zero or positive")
var ErrWorkflowProviderUnknown = errors.New("cms config: workflow provider is invalid")
var ErrWorkflowProviderConfiguredWhenDisabled = errors.New("cms config: workflow provider configured while workflow disabled")
var ErrStorageProfileNameRequired = errors.New("cms config: storage profile name is required")
var ErrStorageProfileNameInvalid = errors.New("cms config: storage profile name is invalid")
var ErrStorageProfileDuplicateName = errors.New("cms config: storage profile name must be unique")
var ErrStorageProfileProviderRequired = errors.New("cms config: storage profile provider is required")
var ErrStorageProfileConfigNameRequired = errors.New("cms config: storage profile config name is required")
var ErrStorageProfileConfigDriverRequired = errors.New("cms config: storage profile config driver is required")
var ErrStorageProfileConfigDSNRequired = errors.New("cms config: storage profile config DSN is required")
var ErrStorageProfileMultipleDefaults = errors.New("cms config: storage profile default must be unique")
var ErrStorageProfileFallbackEmpty = errors.New("cms config: storage profile fallback cannot be empty")
var ErrStorageProfileFallbackSelf = errors.New("cms config: storage profile fallback cannot reference the same profile")
var ErrStorageProfileFallbackUnknown = errors.New("cms config: storage profile fallback references unknown profile")
var ErrStorageProfileAliasNameRequired = errors.New("cms config: storage profile alias name is required")
var ErrStorageProfileAliasInvalid = errors.New("cms config: storage profile alias is invalid")
var ErrStorageProfileAliasTargetRequired = errors.New("cms config: storage profile alias target is required")
var ErrStorageProfileAliasTargetUnknown = errors.New("cms config: storage profile alias target is unknown")
var ErrStorageProfileAliasDuplicate = errors.New("cms config: storage profile alias must be unique")
var ErrStorageProfileAliasCollides = errors.New("cms config: storage profile alias collides with existing profile")

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
	Shortcodes    ShortcodeConfig
	Markdown      MarkdownConfig
	Generator     GeneratorConfig
	Logging       LoggingConfig
	Workflow      WorkflowConfig
}

// ContentConfig captures configuration for the core content module.
type ContentConfig struct {
	PageHierarchy bool
}

// I18NConfig wires go-i18n options through the CMS wrapper.
type I18NConfig struct {
	Enabled               bool
	Locales               []string
	RequireTranslations   bool
	DefaultLocaleRequired bool
}

// StorageConfig lists identifiers for storage-related dependencies.
type StorageConfig struct {
	Provider string
	Profiles []storage.Profile
	Aliases  map[string]string
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
	BasePath          string
	DefaultTheme      string
	DefaultVariant    string
	PartialFallbacks  map[string]string
	CSSVariablePrefix string
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
	Shortcodes    bool
}

// LoggingConfig captures provider-specific options for runtime logging.
type LoggingConfig struct {
	Provider  string
	Level     string
	Format    string
	AddSource bool
	Focus     []string
}

// WorkflowConfig captures workflow engine configuration.
type WorkflowConfig struct {
	Enabled     bool
	Provider    string
	Definitions []WorkflowDefinitionConfig
}

// WorkflowDefinitionConfig documents a workflow definition sourced from configuration.
type WorkflowDefinitionConfig struct {
	Entity      string
	Description string
	States      []WorkflowStateConfig
	Transitions []WorkflowTransitionConfig
}

// WorkflowStateConfig describes a workflow state.
type WorkflowStateConfig struct {
	Name        string
	Description string
	Terminal    bool
	Initial     bool
}

// WorkflowTransitionConfig describes a workflow transition.
type WorkflowTransitionConfig struct {
	Name        string
	Description string
	From        string
	To          string
	Guard       string
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

// MarkdownConfig captures filesystem and parser behaviour for Markdown ingestion.
type MarkdownConfig struct {
	Enabled           bool
	ContentDir        string
	Pattern           string
	Recursive         bool
	LocalePatterns    map[string]string
	DefaultLocale     string
	Locales           []string
	Parser            MarkdownParserConfig
	ProcessShortcodes bool
}

// MarkdownParserConfig mirrors interfaces.ParseOptions for runtime configuration.
type MarkdownParserConfig struct {
	Extensions []string
	Sanitize   bool
	HardWraps  bool
	SafeMode   bool
}

// ShortcodeConfig captures runtime toggles for shortcode processing.
type ShortcodeConfig struct {
	Enabled               bool
	EnableWordPressSyntax bool
	BuiltIns              []string
	CustomDefinitions     []ShortcodeDefinitionConfig
	Security              ShortcodeSecurityConfig
	Cache                 ShortcodeCacheConfig
}

// ShortcodeDefinitionConfig allows hosts to register additional shortcode templates via configuration.
type ShortcodeDefinitionConfig struct {
	Name     string
	Template string
	Schema   map[string]any
}

// ShortcodeSecurityConfig wires sanitisation and execution-guard controls.
type ShortcodeSecurityConfig struct {
	AllowedDomains     []string
	MaxNestingDepth    int
	MaxExecutionTime   time.Duration
	SanitizeOutput     bool
	CSPEnabled         bool
	RateLimitPerMinute int
}

// ShortcodeCacheConfig configures caching hints for shortcode output.
type ShortcodeCacheConfig struct {
	Enabled      bool
	Provider     string
	DefaultTTL   time.Duration
	PerShortcode map[string]time.Duration
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
			Enabled:               true,
			Locales:               []string{"en"},
			RequireTranslations:   true,
			DefaultLocaleRequired: true,
		},
		Shortcodes: ShortcodeConfig{
			Enabled:               false,
			EnableWordPressSyntax: false,
			BuiltIns:              []string{"youtube", "alert", "gallery", "figure", "code"},
			Security: ShortcodeSecurityConfig{
				MaxNestingDepth:    5,
				MaxExecutionTime:   5 * time.Second,
				SanitizeOutput:     true,
				CSPEnabled:         false,
				RateLimitPerMinute: 0,
			},
			Cache: ShortcodeCacheConfig{
				Enabled:      false,
				Provider:     "",
				DefaultTTL:   time.Hour,
				PerShortcode: map[string]time.Duration{},
			},
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
		Workflow: WorkflowConfig{
			Enabled:  true,
			Provider: "simple",
		},
	}
}

// Validate performs high-level consistency checks.
func (cfg Config) Validate() error {
	if err := cfg.Storage.ValidateProfiles(); err != nil {
		return err
	}
	defaultLocale := strings.TrimSpace(cfg.DefaultLocale)
	if cfg.I18N.Enabled {
		if cfg.I18N.DefaultLocaleRequired && defaultLocale == "" {
			return ErrDefaultLocaleRequired
		}
		if cfg.I18N.RequireTranslations && defaultLocale == "" {
			return ErrDefaultLocaleRequired
		}
	}
	if !cfg.Features.Themes {
		if strings.TrimSpace(cfg.Themes.DefaultTheme) != "" ||
			strings.TrimSpace(cfg.Themes.DefaultVariant) != "" ||
			len(cfg.Themes.PartialFallbacks) > 0 ||
			strings.TrimSpace(cfg.Themes.CSSVariablePrefix) != "" {
			return ErrThemesFeatureRequired
		}
	}
	if cfg.Features.Scheduling && !cfg.Features.Versioning {
		return ErrSchedulingFeatureRequiresVersioning
	}
	if cfg.Features.AdvancedCache && !cfg.Cache.Enabled {
		return ErrAdvancedCacheRequiresEnabledCache
	}
	if cfg.Markdown.Enabled {
		if !cfg.Features.Markdown {
			return ErrMarkdownFeatureRequired
		}
		if strings.TrimSpace(cfg.Markdown.ContentDir) == "" {
			return ErrMarkdownContentDirRequired
		}
	}
	if cfg.Shortcodes.Enabled {
		if !cfg.Features.Shortcodes {
			return ErrShortcodesFeatureRequired
		}
	} else if cfg.Shortcodes.EnableWordPressSyntax && !cfg.Features.Shortcodes {
		return ErrShortcodesFeatureRequired
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
	provider := normalizeWorkflowProvider(cfg.Workflow.Provider)
	if !cfg.Workflow.Enabled {
		if provider != "" && provider != "simple" {
			return ErrWorkflowProviderConfiguredWhenDisabled
		}
	} else {
		if !isSupportedWorkflowProvider(provider) {
			return fmt.Errorf("%w: %s", ErrWorkflowProviderUnknown, provider)
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

func normalizeWorkflowProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func isSupportedWorkflowProvider(provider string) bool {
	switch provider {
	case "", "simple", "custom":
		return true
	default:
		return false
	}
}

var storageProfileNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// ValidateProfiles ensures storage profiles and their aliases are well-formed.
func (cfg StorageConfig) ValidateProfiles() error {
	if len(cfg.Profiles) == 0 && len(cfg.Aliases) == 0 {
		return nil
	}
	profilesByName := make(map[string]storage.Profile, len(cfg.Profiles))
	var defaultSeen bool
	for _, raw := range cfg.Profiles {
		name := strings.TrimSpace(raw.Name)
		if name == "" {
			return ErrStorageProfileNameRequired
		}
		if !storageProfileNamePattern.MatchString(name) {
			return fmt.Errorf("%w: %s", ErrStorageProfileNameInvalid, name)
		}
		if _, exists := profilesByName[name]; exists {
			return fmt.Errorf("%w: %s", ErrStorageProfileDuplicateName, name)
		}
		provider := strings.TrimSpace(raw.Provider)
		if provider == "" {
			return fmt.Errorf("%w: %s", ErrStorageProfileProviderRequired, name)
		}
		configName := strings.TrimSpace(raw.Config.Name)
		if configName == "" {
			return fmt.Errorf("%w: %s", ErrStorageProfileConfigNameRequired, name)
		}
		driver := strings.TrimSpace(raw.Config.Driver)
		if driver == "" {
			return fmt.Errorf("%w: %s", ErrStorageProfileConfigDriverRequired, name)
		}
		dsn := strings.TrimSpace(raw.Config.DSN)
		if dsn == "" {
			return fmt.Errorf("%w: %s", ErrStorageProfileConfigDSNRequired, name)
		}
		if raw.Default {
			if defaultSeen {
				return ErrStorageProfileMultipleDefaults
			}
			defaultSeen = true
		}
		for _, fallbackRaw := range raw.Fallbacks {
			fallback := strings.TrimSpace(fallbackRaw)
			if fallback == "" {
				return fmt.Errorf("%w: %s", ErrStorageProfileFallbackEmpty, name)
			}
			if fallback == name {
				return fmt.Errorf("%w: %s", ErrStorageProfileFallbackSelf, name)
			}
		}
		profilesByName[name] = raw
	}

	for _, profile := range cfg.Profiles {
		for _, fallbackRaw := range profile.Fallbacks {
			fallback := strings.TrimSpace(fallbackRaw)
			if fallback == "" {
				return fmt.Errorf("%w: %s", ErrStorageProfileFallbackEmpty, profile.Name)
			}
			if _, ok := profilesByName[fallback]; !ok {
				return fmt.Errorf("%w: %s -> %s", ErrStorageProfileFallbackUnknown, profile.Name, fallback)
			}
		}
	}

	if len(cfg.Aliases) == 0 {
		return nil
	}
	aliasNames := make(map[string]struct{}, len(cfg.Aliases))
	for aliasRaw, targetRaw := range cfg.Aliases {
		alias := strings.TrimSpace(aliasRaw)
		target := strings.TrimSpace(targetRaw)
		if alias == "" {
			return ErrStorageProfileAliasNameRequired
		}
		if !storageProfileNamePattern.MatchString(alias) {
			return fmt.Errorf("%w: %s", ErrStorageProfileAliasInvalid, alias)
		}
		if target == "" {
			return fmt.Errorf("%w: %s", ErrStorageProfileAliasTargetRequired, alias)
		}
		if !storageProfileNamePattern.MatchString(target) {
			return fmt.Errorf("%w: %s", ErrStorageProfileAliasInvalid, target)
		}
		if _, ok := profilesByName[target]; !ok {
			return fmt.Errorf("%w: %s -> %s", ErrStorageProfileAliasTargetUnknown, alias, target)
		}
		if _, ok := profilesByName[alias]; ok {
			return fmt.Errorf("%w: %s", ErrStorageProfileAliasCollides, alias)
		}
		if _, exists := aliasNames[alias]; exists {
			return fmt.Errorf("%w: %s", ErrStorageProfileAliasDuplicate, alias)
		}
		aliasNames[alias] = struct{}{}
	}
	return nil
}
