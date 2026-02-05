package markdowncmd

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	goerrors "github.com/goliatone/go-errors"
	"github.com/google/uuid"
)

type importCall struct {
	directory string
	options   interfaces.ImportOptions
}

type syncCall struct {
	directory string
	options   interfaces.SyncOptions
}

type stubMarkdownService struct {
	importCalls []importCall
	syncCalls   []syncCall

	importResult *interfaces.ImportResult
	syncResult   *interfaces.SyncResult

	importErr error
	syncErr   error
}

func (s *stubMarkdownService) Load(context.Context, string, interfaces.LoadOptions) (*interfaces.Document, error) {
	return nil, nil
}

func (s *stubMarkdownService) LoadDirectory(context.Context, string, interfaces.LoadOptions) ([]*interfaces.Document, error) {
	return nil, nil
}

func (s *stubMarkdownService) Render(context.Context, []byte, interfaces.ParseOptions) ([]byte, error) {
	return nil, nil
}

func (s *stubMarkdownService) RenderDocument(context.Context, *interfaces.Document, interfaces.ParseOptions) ([]byte, error) {
	return nil, nil
}

func (s *stubMarkdownService) Import(context.Context, *interfaces.Document, interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	return nil, nil
}

func (s *stubMarkdownService) ImportDirectory(ctx context.Context, directory string, opts interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	s.importCalls = append(s.importCalls, importCall{
		directory: directory,
		options:   opts,
	})
	if s.importErr != nil {
		return nil, s.importErr
	}
	return s.importResult, nil
}

func (s *stubMarkdownService) Sync(ctx context.Context, directory string, opts interfaces.SyncOptions) (*interfaces.SyncResult, error) {
	s.syncCalls = append(s.syncCalls, syncCall{
		directory: directory,
		options:   opts,
	})
	if s.syncErr != nil {
		return nil, s.syncErr
	}
	return s.syncResult, nil
}

type captureLogger struct {
	fields       []map[string]any
	infoMessages []string
}

var _ interfaces.Logger = (*captureLogger)(nil)

func (c *captureLogger) Trace(string, ...any) {}
func (c *captureLogger) Debug(string, ...any) {}
func (c *captureLogger) Info(msg string, _ ...any) {
	c.infoMessages = append(c.infoMessages, msg)
}
func (c *captureLogger) Warn(string, ...any)  {}
func (c *captureLogger) Error(string, ...any) {}
func (c *captureLogger) Fatal(string, ...any) {}

func (c *captureLogger) WithFields(fields map[string]any) interfaces.Logger {
	if fields == nil {
		c.fields = append(c.fields, map[string]any{})
		return c
	}
	copied := make(map[string]any, len(fields))
	for k, v := range fields {
		copied[k] = v
	}
	c.fields = append(c.fields, copied)
	return c
}

func (c *captureLogger) WithContext(context.Context) interfaces.Logger {
	return c
}

func TestImportDirectoryHandlerInvokesService(t *testing.T) {
	service := &stubMarkdownService{
		importResult: &interfaces.ImportResult{
			CreatedContentIDs: []uuid.UUID{uuid.New()},
			UpdatedContentIDs: []uuid.UUID{uuid.New()},
			SkippedContentIDs: []uuid.UUID{},
			Errors:            []error{},
		},
	}
	logger := &captureLogger{}
	handler := NewImportDirectoryHandler(service, logger, FeatureGates{
		MarkdownEnabled: func() bool { return true },
	})

	contentTypeID := uuid.New()
	authorID := uuid.New()

	cmd := ImportDirectoryCommand{
		Directory:                       "content/en",
		ContentTypeID:                   contentTypeID,
		AuthorID:                        authorID,
		ContentAllowMissingTranslations: true,
		DryRun:                          true,
	}

	if err := handler.Execute(context.Background(), cmd); err != nil {
		t.Fatalf("execute import directory: %v", err)
	}

	if len(service.importCalls) != 1 {
		t.Fatalf("expected import call, got %d", len(service.importCalls))
	}
	call := service.importCalls[0]
	if call.directory != cmd.Directory {
		t.Fatalf("expected directory %q, got %q", cmd.Directory, call.directory)
	}
	if call.options.ContentTypeID != contentTypeID {
		t.Fatalf("expected content type %s, got %s", contentTypeID, call.options.ContentTypeID)
	}
	if call.options.AuthorID != authorID {
		t.Fatalf("expected author %s, got %s", authorID, call.options.AuthorID)
	}
	if !call.options.ContentAllowMissingTranslations {
		t.Fatalf("expected content allow missing translations option set")
	}
	if !call.options.DryRun {
		t.Fatalf("expected dry run option set")
	}

	if len(logger.infoMessages) == 0 {
		t.Fatalf("expected summary log emitted")
	}
	found := false
	for _, fields := range logger.fields {
		if _, ok := fields["created_count"]; ok {
			found = true
			if fields["created_count"] != len(service.importResult.CreatedContentIDs) {
				t.Fatalf("expected created count %d, got %v", len(service.importResult.CreatedContentIDs), fields["created_count"])
			}
			if fields["dry_run"] != cmd.DryRun {
				t.Fatalf("expected dry_run %v, got %v", cmd.DryRun, fields["dry_run"])
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected summary fields recorded, got %#v", logger.fields)
	}
}

func TestImportDirectoryHandlerFeatureDisabled(t *testing.T) {
	service := &stubMarkdownService{}
	handler := NewImportDirectoryHandler(service, logging.NoOp(), FeatureGates{
		MarkdownEnabled: func() bool { return false },
	})

	err := handler.Execute(context.Background(), ImportDirectoryCommand{
		Directory: "content",
	})
	if !errors.Is(err, ErrMarkdownFeatureDisabled) {
		t.Fatalf("expected feature disabled error, got %v", err)
	}
	if len(service.importCalls) != 0 {
		t.Fatalf("expected no import calls, got %d", len(service.importCalls))
	}
}

func TestImportDirectoryHandlerContextCancellation(t *testing.T) {
	service := &stubMarkdownService{}
	handler := NewImportDirectoryHandler(service, logging.NoOp(), FeatureGates{
		MarkdownEnabled: func() bool { return true },
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := handler.Execute(ctx, ImportDirectoryCommand{
		Directory: "content",
	})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command error category, got %v", err)
	}
	if len(service.importCalls) != 0 {
		t.Fatalf("expected no import calls, got %d", len(service.importCalls))
	}
}

func TestSyncDirectoryHandlerInvokesService(t *testing.T) {
	service := &stubMarkdownService{
		syncResult: &interfaces.SyncResult{
			Created: 2,
			Updated: 1,
			Deleted: 1,
			Skipped: 3,
			Errors:  []error{},
		},
	}
	logger := &captureLogger{}
	handler := NewSyncDirectoryHandler(service, logger, FeatureGates{
		MarkdownEnabled: func() bool { return true },
	})

	contentTypeID := uuid.New()
	authorID := uuid.New()

	cmd := SyncDirectoryCommand{
		Directory:                       "content",
		ContentTypeID:                   contentTypeID,
		AuthorID:                        authorID,
		ContentAllowMissingTranslations: true,
		DryRun:                          true,
		DeleteOrphaned:                  true,
		UpdateExisting:                  true,
	}

	if err := handler.Execute(context.Background(), cmd); err != nil {
		t.Fatalf("execute sync directory: %v", err)
	}

	if len(service.syncCalls) != 1 {
		t.Fatalf("expected sync call, got %d", len(service.syncCalls))
	}
	call := service.syncCalls[0]
	if call.directory != cmd.Directory {
		t.Fatalf("expected directory %q, got %q", cmd.Directory, call.directory)
	}
	if call.options.ContentTypeID != contentTypeID {
		t.Fatalf("expected content type %s, got %s", contentTypeID, call.options.ContentTypeID)
	}
	if call.options.AuthorID != authorID {
		t.Fatalf("expected author %s, got %s", authorID, call.options.AuthorID)
	}
	if !call.options.ContentAllowMissingTranslations {
		t.Fatalf("expected content allow missing translations option set")
	}
	if !call.options.DryRun {
		t.Fatalf("expected dry run option set")
	}
	if !call.options.DeleteOrphaned {
		t.Fatalf("expected delete orphans option set")
	}
	if !call.options.UpdateExisting {
		t.Fatalf("expected update existing option set")
	}

	found := false
	for _, fields := range logger.fields {
		if _, ok := fields["deleted_count"]; ok {
			found = true
			if fields["deleted_count"] != service.syncResult.Deleted {
				t.Fatalf("expected deleted count %d, got %v", service.syncResult.Deleted, fields["deleted_count"])
			}
			if fields["delete_orphans"] != cmd.DeleteOrphaned {
				t.Fatalf("expected delete_orphans %v, got %v", cmd.DeleteOrphaned, fields["delete_orphans"])
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected sync summary fields recorded, got %#v", logger.fields)
	}
}

func TestSyncDirectoryHandlerFeatureDisabled(t *testing.T) {
	service := &stubMarkdownService{}
	handler := NewSyncDirectoryHandler(service, logging.NoOp(), FeatureGates{
		MarkdownEnabled: func() bool { return false },
	})

	err := handler.Execute(context.Background(), SyncDirectoryCommand{
		Directory: "content",
	})
	if !errors.Is(err, ErrMarkdownFeatureDisabled) {
		t.Fatalf("expected feature disabled error, got %v", err)
	}
	if len(service.syncCalls) != 0 {
		t.Fatalf("expected no sync calls, got %d", len(service.syncCalls))
	}
}
