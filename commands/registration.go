package commands

import (
	"errors"
	"strings"

	markdownadapter "github.com/goliatone/go-cms/commands/markdown"
	auditcmd "github.com/goliatone/go-cms/internal/commands/audit"
	blockscmd "github.com/goliatone/go-cms/internal/commands/blocks"
	contentcmd "github.com/goliatone/go-cms/internal/commands/content"
	markdowncmd "github.com/goliatone/go-cms/internal/commands/markdown"
	mediacmd "github.com/goliatone/go-cms/internal/commands/media"
	menuscmd "github.com/goliatone/go-cms/internal/commands/menus"
	pagescmd "github.com/goliatone/go-cms/internal/commands/pages"
	staticcmd "github.com/goliatone/go-cms/internal/commands/static"
	widgetscmd "github.com/goliatone/go-cms/internal/commands/widgets"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
)

// CommandRegistry records command handlers so hosts can expose them via CLI or cron.
type CommandRegistry interface {
	RegisterCommand(handler any) error
}

// CommandDispatcher subscribes command handlers to a dispatcher implementation.
type CommandDispatcher interface {
	RegisterCommand(handler any) (CommandSubscription, error)
}

// CommandSubscription allows hosts to tear down dispatcher subscriptions.
type CommandSubscription interface {
	Unsubscribe()
}

// CronRegistrar registers command handlers with a cron scheduler.
type CronRegistrar func(command.HandlerConfig, any) error

// RegistrationOptions configures how handlers are registered during construction.
type RegistrationOptions struct {
	Registry       CommandRegistry
	Dispatcher     CommandDispatcher
	CronRegistrar  CronRegistrar
	LoggerProvider interfaces.LoggerProvider
	// CleanupAuditCron overrides the default cron expression applied to the audit cleanup handler.
	CleanupAuditCron string
}

// RegistrationResult captures the constructed command handlers and any dispatcher subscriptions.
type RegistrationResult struct {
	Handlers      []any
	Subscriptions []CommandSubscription
}

// RegisterContainerCommands builds the command handlers exposed by the provided container and
// optionally registers them with registry/dispatcher/cron integrations.
func RegisterContainerCommands(container *di.Container, opts RegistrationOptions) (*RegistrationResult, error) {
	if container == nil {
		return &RegistrationResult{}, nil
	}

	cfg := container.Config

	provider := opts.LoggerProvider
	if provider == nil {
		provider = container.LoggerProvider()
	}

	if opts.Registry != nil && opts.CronRegistrar != nil {
		if reg, ok := opts.Registry.(interface {
			SetCronRegister(func(command.HandlerConfig, any) error) *command.Registry
		}); ok && reg != nil {
			reg.SetCronRegister(opts.CronRegistrar)
		}
	}

	result := &RegistrationResult{
		Handlers:      make([]any, 0),
		Subscriptions: make([]CommandSubscription, 0),
	}

	var errs error

	register := func(handler any) {
		if handler == nil {
			return
		}
		result.Handlers = append(result.Handlers, handler)

		if opts.Registry != nil {
			if err := opts.Registry.RegisterCommand(handler); err != nil {
				errs = errors.Join(errs, err)
			}
		}

		if opts.Dispatcher != nil {
			subscription, err := opts.Dispatcher.RegisterCommand(handler)
			if err != nil {
				errs = errors.Join(errs, err)
			} else if subscription != nil {
				result.Subscriptions = append(result.Subscriptions, subscription)
			}
		}

		if opts.CronRegistrar != nil {
			if cronCmd, ok := handler.(command.CronCommand); ok {
				if err := opts.CronRegistrar(cronCmd.CronOptions(), cronCmd.CronHandler()); err != nil {
					errs = errors.Join(errs, err)
				}
			}
		}
	}

	loggerFor := func(module string) interfaces.Logger {
		return CommandLogger(provider, module)
	}

	// Content commands.
	if service := container.ContentService(); service != nil {
		gates := contentcmd.FeatureGates{
			VersioningEnabled: func() bool { return cfg.Features.Versioning },
			SchedulingEnabled: func() bool { return cfg.Features.Scheduling },
		}
		if cfg.Features.Versioning {
			contentLogger := loggerFor("content")
			register(contentcmd.NewPublishContentHandler(service, contentLogger, gates))
			register(contentcmd.NewRestoreContentVersionHandler(service, contentLogger, gates))
		}
		if cfg.Features.Scheduling {
			register(contentcmd.NewScheduleContentHandler(service, loggerFor("content"), gates))
		}
	}

	// Page commands.
	if service := container.PageService(); service != nil {
		gates := pagescmd.FeatureGates{
			VersioningEnabled: func() bool { return cfg.Features.Versioning },
			SchedulingEnabled: func() bool { return cfg.Features.Scheduling },
		}
		if cfg.Features.Versioning {
			pagesLogger := loggerFor("pages")
			register(pagescmd.NewPublishPageHandler(service, pagesLogger, gates))
			register(pagescmd.NewRestorePageVersionHandler(service, pagesLogger, gates))
		}
		if cfg.Features.Scheduling {
			register(pagescmd.NewSchedulePageHandler(service, loggerFor("pages"), gates))
		}
	}

	// Media commands.
	if service := container.MediaService(); service != nil && cfg.Features.MediaLibrary {
		gates := mediacmd.FeatureGates{
			MediaLibraryEnabled: func() bool { return cfg.Features.MediaLibrary },
		}
		mediaLogger := loggerFor("media")
		register(mediacmd.NewImportAssetsHandler(service, mediaLogger, gates))
		register(mediacmd.NewCleanupAssetsHandler(service, mediaLogger, gates))
	}

	// Markdown commands.
	if service := container.MarkdownService(); service != nil && cfg.Features.Markdown {
		gates := markdowncmd.FeatureGates{
			MarkdownEnabled: func() bool { return cfg.Features.Markdown },
		}
		handlerSet, err := markdownadapter.RegisterMarkdownCommands(nil, service, provider, gates)
		if err != nil {
			errs = errors.Join(errs, err)
		} else if handlerSet != nil {
			register(handlerSet.Import)
			register(handlerSet.Sync)
		}
	}

	// Static generator commands.
	if service := container.GeneratorService(); service != nil && cfg.Generator.Enabled {
		gates := staticcmd.FeatureGates{
			GeneratorEnabled: func() bool { return cfg.Generator.Enabled },
			SitemapEnabled:   func() bool { return cfg.Generator.GenerateSitemap },
		}
		staticLogger := loggerFor("static")
		register(staticcmd.NewBuildSiteHandler(service, staticLogger, gates))
		register(staticcmd.NewDiffSiteHandler(service, staticLogger, gates))
		register(staticcmd.NewCleanSiteHandler(service, staticLogger, gates))
		register(staticcmd.NewBuildSitemapHandler(service, staticLogger, gates))
	}

	// Menu commands.
	if service := container.MenuService(); service != nil {
		gates := menuscmd.FeatureGates{
			MenusEnabled: func() bool { return service != nil },
		}
		register(menuscmd.NewInvalidateMenuCacheHandler(service, loggerFor("menus"), gates))
	}

	// Blocks commands.
	if service := container.BlockService(); service != nil {
		gates := blockscmd.FeatureGates{
			BlocksEnabled: func() bool { return service != nil },
		}
		register(blockscmd.NewSyncBlockRegistryHandler(service, loggerFor("blocks"), gates))
	}

	// Widget commands.
	if service := container.WidgetService(); service != nil && cfg.Features.Widgets {
		gates := widgetscmd.FeatureGates{
			WidgetsEnabled: func() bool { return cfg.Features.Widgets },
		}
		register(widgetscmd.NewSyncWidgetRegistryHandler(service, loggerFor("widgets"), gates))
	}

	// Audit commands.
	if cfg.Features.Scheduling && container.AuditRecorder() != nil {
		auditLogger := loggerFor("audit")
		if container.JobWorker() != nil {
			register(auditcmd.NewReplayAuditHandler(container.JobWorker(), auditLogger))
		}
		register(auditcmd.NewExportAuditHandler(container.AuditRecorder(), auditLogger))
		cleanupOpts := []auditcmd.CleanupHandlerOption{}
		if expr := strings.TrimSpace(opts.CleanupAuditCron); expr != "" {
			cleanupOpts = append(cleanupOpts, auditcmd.CleanupWithCronExpression(expr))
		}
		register(auditcmd.NewCleanupAuditHandler(container.AuditRecorder(), auditLogger, cleanupOpts...))
	}

	if errs != nil && len(result.Handlers) == 0 {
		return result, errs
	}

	if len(result.Handlers) == 0 {
		return result, errors.New("no command handlers registered; ensure services are configured and required features enabled")
	}

	return result, errs
}
