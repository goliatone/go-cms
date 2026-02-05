package markdowncmd

import (
	"context"
	"errors"
	"time"

	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
)

const (
	importOperation = "markdown.import_directory"
	syncOperation   = "markdown.sync_directory"
)

var (
	// ErrMarkdownFeatureDisabled is returned when the markdown feature flag is disabled at runtime.
	ErrMarkdownFeatureDisabled = errors.New("markdown command: feature disabled")
)

var (
	_ command.Commander[ImportDirectoryCommand] = (*ImportDirectoryHandler)(nil)
	_ command.Commander[SyncDirectoryCommand]   = (*SyncDirectoryHandler)(nil)
)

// ImportDirectoryHandler orchestrates Markdown directory imports.
type ImportDirectoryHandler struct {
	service interfaces.MarkdownService
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// ImportDirectoryOption customises the import handler.
type ImportDirectoryOption func(*ImportDirectoryHandler)

// ImportDirectoryWithTimeout overrides the default execution timeout.
func ImportDirectoryWithTimeout(timeout time.Duration) ImportDirectoryOption {
	return func(h *ImportDirectoryHandler) {
		h.timeout = timeout
	}
}

// NewImportDirectoryHandler creates a handler bound to the supplied Markdown service.
func NewImportDirectoryHandler(service interfaces.MarkdownService, logger interfaces.Logger, gates FeatureGates, opts ...ImportDirectoryOption) *ImportDirectoryHandler {
	handler := &ImportDirectoryHandler{
		service: service,
		logger:  commands.EnsureLogger(logger),
		gates:   gates,
		timeout: commands.DefaultCommandTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(handler)
		}
	}
	return handler
}

// Execute satisfies command.Commander[ImportDirectoryCommand].
func (h *ImportDirectoryHandler) Execute(ctx context.Context, msg ImportDirectoryCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}
	if !h.gates.markdownEnabled() {
		return commands.WrapExecuteError(ErrMarkdownFeatureDisabled)
	}

	importOpts := interfaces.ImportOptions{
		ContentTypeID:                   msg.ContentTypeID,
		AuthorID:                        msg.AuthorID,
		DryRun:                          msg.DryRun,
		ContentAllowMissingTranslations: msg.ContentAllowMissingTranslations,
	}

	result, err := h.service.ImportDirectory(ctx, msg.Directory, importOpts)
	if err != nil {
		return commands.WrapExecuteError(err)
	}
	if result != nil {
		logging.WithFields(h.logger, map[string]any{
			"created_count": len(result.CreatedContentIDs),
			"updated_count": len(result.UpdatedContentIDs),
			"skipped_count": len(result.SkippedContentIDs),
			"error_count":   len(result.Errors),
			"dry_run":       msg.DryRun,
		}).Info("markdown.command.import_directory.completed")
	}
	return nil
}

// CLIHandler exposes the import handler for CLI registration.
func (h *ImportDirectoryHandler) CLIHandler() any {
	return h
}

// CLIOptions describes the CLI metadata for markdown import.
func (h *ImportDirectoryHandler) CLIOptions() command.CLIConfig {
	return command.CLIConfig{
		Path:        []string{"markdown", "import"},
		Group:       "markdown",
		Description: "Import markdown content from a directory",
	}
}

// SyncDirectoryHandler orchestrates Markdown sync workflows.
type SyncDirectoryHandler struct {
	service interfaces.MarkdownService
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// SyncDirectoryOption customises the sync handler.
type SyncDirectoryOption func(*SyncDirectoryHandler)

// SyncDirectoryWithTimeout overrides the default execution timeout.
func SyncDirectoryWithTimeout(timeout time.Duration) SyncDirectoryOption {
	return func(h *SyncDirectoryHandler) {
		h.timeout = timeout
	}
}

// NewSyncDirectoryHandler creates a handler bound to the supplied Markdown service.
func NewSyncDirectoryHandler(service interfaces.MarkdownService, logger interfaces.Logger, gates FeatureGates, opts ...SyncDirectoryOption) *SyncDirectoryHandler {
	handler := &SyncDirectoryHandler{
		service: service,
		logger:  commands.EnsureLogger(logger),
		gates:   gates,
		timeout: commands.DefaultCommandTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(handler)
		}
	}
	return handler
}

// Execute satisfies command.Commander[SyncDirectoryCommand].
func (h *SyncDirectoryHandler) Execute(ctx context.Context, msg SyncDirectoryCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}
	if !h.gates.markdownEnabled() {
		return commands.WrapExecuteError(ErrMarkdownFeatureDisabled)
	}

	importOpts := interfaces.ImportOptions{
		ContentTypeID:                   msg.ContentTypeID,
		AuthorID:                        msg.AuthorID,
		DryRun:                          msg.DryRun,
		ContentAllowMissingTranslations: msg.ContentAllowMissingTranslations,
	}

	syncOpts := interfaces.SyncOptions{
		ImportOptions:  importOpts,
		DeleteOrphaned: msg.DeleteOrphaned,
		UpdateExisting: msg.UpdateExisting,
	}

	result, err := h.service.Sync(ctx, msg.Directory, syncOpts)
	if err != nil {
		return commands.WrapExecuteError(err)
	}
	if result != nil {
		logging.WithFields(h.logger, map[string]any{
			"created_count":   result.Created,
			"updated_count":   result.Updated,
			"deleted_count":   result.Deleted,
			"skipped_count":   result.Skipped,
			"error_count":     len(result.Errors),
			"dry_run":         msg.DryRun,
			"delete_orphans":  msg.DeleteOrphaned,
			"update_existing": msg.UpdateExisting,
		}).Info("markdown.command.sync_directory.completed")
	}
	return nil
}

// CLIHandler exposes the sync handler for CLI registration.
func (h *SyncDirectoryHandler) CLIHandler() any {
	return h
}

// CLIOptions describes the CLI metadata for markdown sync.
func (h *SyncDirectoryHandler) CLIOptions() command.CLIConfig {
	return command.CLIConfig{
		Path:        []string{"markdown", "sync"},
		Group:       "markdown",
		Description: "Synchronise markdown content from a directory",
	}
}
