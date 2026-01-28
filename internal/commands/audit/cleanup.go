package auditcmd

import (
	"context"
	"strings"
	"time"

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
	cronConfig command.HandlerConfig
	timeout    time.Duration
}

// CleanupHandlerOption customises the cleanup handler.
type CleanupHandlerOption func(*cleanupHandlerConfig)

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

// CleanupWithTimeout overrides the default execution timeout.
func CleanupWithTimeout(timeout time.Duration) CleanupHandlerOption {
	return func(cfg *cleanupHandlerConfig) {
		cfg.timeout = timeout
	}
}

// CleanupAuditHandler clears audit logs via the supplied cleaner implementation.
type CleanupAuditHandler struct {
	cleaner    AuditCleaner
	logger     interfaces.Logger
	cronConfig command.HandlerConfig
	timeout    time.Duration
}

// NewCleanupAuditHandler constructs a handler that delegates to the provided cleaner instance.
func NewCleanupAuditHandler(cleaner AuditCleaner, logger interfaces.Logger, opts ...CleanupHandlerOption) *CleanupAuditHandler {
	cfg := cleanupHandlerConfig{
		cronConfig: command.HandlerConfig{
			Expression: "@daily",
		},
		timeout: commands.DefaultCommandTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return &CleanupAuditHandler{
		cleaner:    cleaner,
		logger:     commands.EnsureLogger(logger),
		cronConfig: cfg.cronConfig,
		timeout:    cfg.timeout,
	}
}

// Execute satisfies command.Commander[CleanupAuditCommand].
func (h *CleanupAuditHandler) Execute(ctx context.Context, msg CleanupAuditCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}

	events, err := h.cleaner.List(ctx)
	if err != nil {
		return commands.WrapExecuteError(err)
	}

	logger := logging.WithFields(h.logger, map[string]any{
		"operation": "audit.cleanup",
	})

	if msg.DryRun {
		logging.WithFields(logger, map[string]any{
			"dry_run":        true,
			"existing_count": len(events),
		}).Debug("audit.command.cleanup.dry_run")
		return nil
	}

	if err := h.cleaner.Clear(ctx); err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(logger, map[string]any{
		"removed": len(events),
	}).Debug("audit.command.cleanup.removed")
	return nil
}

// CronHandler satisfies command.CronCommand by binding cleanup execution to a cron runner.
func (h *CleanupAuditHandler) CronHandler() func() error {
	return func() error {
		return h.Execute(context.Background(), CleanupAuditCommand{})
	}
}

// CronOptions satisfies command.CronCommand by returning the configured cron metadata.
func (h *CleanupAuditHandler) CronOptions() command.HandlerConfig {
	return h.cronConfig
}

// CLIHandler exposes the cleanup handler to CLI integrations.
func (h *CleanupAuditHandler) CLIHandler() any {
	return h
}

// CLIOptions describes the CLI metadata for audit cleanup.
func (h *CleanupAuditHandler) CLIOptions() command.CLIConfig {
	return command.CLIConfig{
		Path:        []string{"audit", "cleanup"},
		Group:       "audit",
		Description: "Remove recorded audit events; supports dry-run",
	}
}
