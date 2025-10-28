package auditcmd

import (
	"context"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
)

const cleanupAuditMessageType = "cms.audit.cleanup"

// AuditCleaner extends AuditLog with cleanup capabilities.
type AuditCleaner interface {
	AuditLog
	Clear(ctx context.Context) error
}

// CleanupAuditCommand removes recorded audit events. When DryRun is true only the event count is reported.
type CleanupAuditCommand struct {
	DryRun bool `json:"dry_run,omitempty"`
}

// Type implements command.Message.
func (CleanupAuditCommand) Type() string { return cleanupAuditMessageType }

// Validate satisfies command.Message.
func (CleanupAuditCommand) Validate() error {
	return validation.ValidateStruct(&CleanupAuditCommand{})
}

type cleanupHandlerConfig struct {
	handlerOptions []commands.HandlerOption[CleanupAuditCommand]
	cronConfig     command.HandlerConfig
}

// CleanupHandlerOption customises the cleanup handler.
type CleanupHandlerOption func(*cleanupHandlerConfig)

// CleanupWithHandlerOptions forwards CMS handler options to the wrapped command handler.
func CleanupWithHandlerOptions(opts ...commands.HandlerOption[CleanupAuditCommand]) CleanupHandlerOption {
	return func(cfg *cleanupHandlerConfig) {
		cfg.handlerOptions = append(cfg.handlerOptions, opts...)
	}
}

// CleanupWithCronConfig overrides the cron registration options for the cleanup handler.
func CleanupWithCronConfig(config command.HandlerConfig) CleanupHandlerOption {
	return func(cfg *cleanupHandlerConfig) {
		cfg.cronConfig = config
	}
}

// CleanupWithCronExpression overrides the cron expression for the cleanup handler.
func CleanupWithCronExpression(expression string) CleanupHandlerOption {
	return func(cfg *cleanupHandlerConfig) {
		if trimmed := strings.TrimSpace(expression); trimmed != "" {
			cfg.cronConfig.Expression = trimmed
		}
	}
}

// CleanupAuditHandler clears audit logs via the supplied cleaner implementation.
type CleanupAuditHandler struct {
	inner      *commands.Handler[CleanupAuditCommand]
	cronConfig command.HandlerConfig
}

// NewCleanupAuditHandler constructs a handler that delegates to the provided cleaner instance.
func NewCleanupAuditHandler(cleaner AuditCleaner, logger interfaces.Logger, opts ...CleanupHandlerOption) *CleanupAuditHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	cfg := cleanupHandlerConfig{
		cronConfig: command.HandlerConfig{
			Expression: "@daily",
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	exec := func(ctx context.Context, msg CleanupAuditCommand) error {
		events, err := cleaner.List(ctx)
		if err != nil {
			return err
		}
		if msg.DryRun {
			logging.WithFields(baseLogger, map[string]any{
				"dry_run":        true,
				"existing_count": len(events),
			}).Debug("audit.command.cleanup.dry_run")
			return nil
		}
		if err := cleaner.Clear(ctx); err != nil {
			return err
		}
		logging.WithFields(baseLogger, map[string]any{
			"removed": len(events),
		}).Debug("audit.command.cleanup.removed")
		return nil
	}

	handlerOpts := []commands.HandlerOption[CleanupAuditCommand]{
		commands.WithLogger[CleanupAuditCommand](baseLogger),
		commands.WithOperation[CleanupAuditCommand]("audit.cleanup"),
		commands.WithMessageFields(func(msg CleanupAuditCommand) map[string]any {
			if !msg.DryRun {
				return nil
			}
			return map[string]any{"dry_run": true}
		}),
		commands.WithTelemetry(commands.DefaultTelemetry[CleanupAuditCommand](baseLogger)),
	}
	handlerOpts = append(handlerOpts, cfg.handlerOptions...)

	return &CleanupAuditHandler{
		inner:      commands.NewHandler(exec, handlerOpts...),
		cronConfig: cfg.cronConfig,
	}
}

// Execute satisfies command.Commander[CleanupAuditCommand].
func (h *CleanupAuditHandler) Execute(ctx context.Context, msg CleanupAuditCommand) error {
	return h.inner.Execute(ctx, msg)
}

// CronHandler satisfies command.CronCommand by binding cleanup execution to a cron runner.
func (h *CleanupAuditHandler) CronHandler() func() error {
	return func() error {
		return h.inner.Execute(context.Background(), CleanupAuditCommand{})
	}
}

// CronOptions satisfies command.CronCommand by returning the configured cron metadata.
func (h *CleanupAuditHandler) CronOptions() command.HandlerConfig {
	return h.cronConfig
}
