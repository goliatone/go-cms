package markdowncmd

import (
	"context"
	"errors"

	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
	"github.com/google/uuid"
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

// ImportDirectoryHandler orchestrates Markdown directory imports via the shared command handler foundation.
type ImportDirectoryHandler struct {
	inner *commands.Handler[ImportDirectoryCommand]
}

// NewImportDirectoryHandler creates a handler bound to the supplied Markdown service.
func NewImportDirectoryHandler(service interfaces.MarkdownService, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[ImportDirectoryCommand]) *ImportDirectoryHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg ImportDirectoryCommand) error {
		if !gates.markdownEnabled() {
			return ErrMarkdownFeatureDisabled
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		importOpts := interfaces.ImportOptions{
			ContentTypeID:                   msg.ContentTypeID,
			AuthorID:                        msg.AuthorID,
			CreatePages:                     msg.CreatePages,
			DryRun:                          msg.DryRun,
			ContentAllowMissingTranslations: msg.ContentAllowMissingTranslations,
			PageAllowMissingTranslations:    msg.PageAllowMissingTranslations,
		}
		if tpl := normalizeUUIDPointer(msg.TemplateID); tpl != nil {
			importOpts.TemplateID = tpl
		}

		result, err := service.ImportDirectory(ctx, msg.Directory, importOpts)
		if err != nil {
			return err
		}
		if result != nil {
			logging.WithFields(baseLogger, map[string]any{
				"created_count": len(result.CreatedContentIDs),
				"updated_count": len(result.UpdatedContentIDs),
				"skipped_count": len(result.SkippedContentIDs),
				"error_count":   len(result.Errors),
				"dry_run":       msg.DryRun,
			}).Info("markdown.command.import_directory.completed")
		}
		return nil
	}

	handlerOpts := []commands.HandlerOption[ImportDirectoryCommand]{
		commands.WithLogger[ImportDirectoryCommand](baseLogger),
		commands.WithOperation[ImportDirectoryCommand](importOperation),
		commands.WithMessageFields(func(msg ImportDirectoryCommand) map[string]any {
			fields := map[string]any{
				"directory": msg.Directory,
			}
			if msg.ContentTypeID != uuid.Nil {
				fields["content_type_id"] = msg.ContentTypeID
			}
			if msg.AuthorID != uuid.Nil {
				fields["author_id"] = msg.AuthorID
			}
			if tpl := normalizeUUIDPointer(msg.TemplateID); tpl != nil {
				fields["template_id"] = *tpl
			}
			if msg.CreatePages {
				fields["create_pages"] = true
			}
			if msg.ContentAllowMissingTranslations {
				fields["content_allow_missing_translations"] = true
			}
			if msg.PageAllowMissingTranslations {
				fields["page_allow_missing_translations"] = true
			}
			if msg.DryRun {
				fields["dry_run"] = true
			}
			return fields
		}),
		commands.WithTelemetry(commands.DefaultTelemetry[ImportDirectoryCommand](baseLogger)),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &ImportDirectoryHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[ImportDirectoryCommand].
func (h *ImportDirectoryHandler) Execute(ctx context.Context, msg ImportDirectoryCommand) error {
	return h.inner.Execute(ctx, msg)
}

// SyncDirectoryHandler orchestrates Markdown sync workflows via the shared command handler foundation.
type SyncDirectoryHandler struct {
	inner *commands.Handler[SyncDirectoryCommand]
}

// NewSyncDirectoryHandler creates a handler bound to the supplied Markdown service.
func NewSyncDirectoryHandler(service interfaces.MarkdownService, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[SyncDirectoryCommand]) *SyncDirectoryHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg SyncDirectoryCommand) error {
		if !gates.markdownEnabled() {
			return ErrMarkdownFeatureDisabled
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		importOpts := interfaces.ImportOptions{
			ContentTypeID:                   msg.ContentTypeID,
			AuthorID:                        msg.AuthorID,
			CreatePages:                     msg.CreatePages,
			DryRun:                          msg.DryRun,
			ContentAllowMissingTranslations: msg.ContentAllowMissingTranslations,
			PageAllowMissingTranslations:    msg.PageAllowMissingTranslations,
		}
		if tpl := normalizeUUIDPointer(msg.TemplateID); tpl != nil {
			importOpts.TemplateID = tpl
		}

		syncOpts := interfaces.SyncOptions{
			ImportOptions:  importOpts,
			DeleteOrphaned: msg.DeleteOrphaned,
			UpdateExisting: msg.UpdateExisting,
		}

		result, err := service.Sync(ctx, msg.Directory, syncOpts)
		if err != nil {
			return err
		}
		if result != nil {
			logging.WithFields(baseLogger, map[string]any{
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

	handlerOpts := []commands.HandlerOption[SyncDirectoryCommand]{
		commands.WithLogger[SyncDirectoryCommand](baseLogger),
		commands.WithOperation[SyncDirectoryCommand](syncOperation),
		commands.WithMessageFields(func(msg SyncDirectoryCommand) map[string]any {
			fields := map[string]any{
				"directory": msg.Directory,
			}
			if msg.ContentTypeID != uuid.Nil {
				fields["content_type_id"] = msg.ContentTypeID
			}
			if msg.AuthorID != uuid.Nil {
				fields["author_id"] = msg.AuthorID
			}
			if tpl := normalizeUUIDPointer(msg.TemplateID); tpl != nil {
				fields["template_id"] = *tpl
			}
			if msg.CreatePages {
				fields["create_pages"] = true
			}
			if msg.ContentAllowMissingTranslations {
				fields["content_allow_missing_translations"] = true
			}
			if msg.PageAllowMissingTranslations {
				fields["page_allow_missing_translations"] = true
			}
			if msg.DryRun {
				fields["dry_run"] = true
			}
			if msg.DeleteOrphaned {
				fields["delete_orphaned"] = true
			}
			if msg.UpdateExisting {
				fields["update_existing"] = true
			}
			return fields
		}),
		commands.WithTelemetry(commands.DefaultTelemetry[SyncDirectoryCommand](baseLogger)),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &SyncDirectoryHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[SyncDirectoryCommand].
func (h *SyncDirectoryHandler) Execute(ctx context.Context, msg SyncDirectoryCommand) error {
	return h.inner.Execute(ctx, msg)
}

func normalizeUUIDPointer(id *uuid.UUID) *uuid.UUID {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	value := *id
	return &value
}
