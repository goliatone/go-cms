package console_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/logging/console"
)

func TestConsoleLogger_WritesStructuredEntry(t *testing.T) {
	var buf bytes.Buffer
	now := time.Date(2024, 3, 14, 15, 9, 26, 535897000, time.UTC)

	minLevel := console.LevelDebug
	provider := console.NewProvider(console.Options{
		Writer:   &buf,
		TimeFunc: func() time.Time { return now },
		MinLevel: &minLevel,
	})

	logger := provider.GetLogger("cms.content")
	logger = logger.WithFields(map[string]any{"module": "cms.content"})
	ctx := logging.ContextWithFields(context.Background(), map[string]any{
		"correlation_id": "req-1234",
	})
	logger = logger.WithContext(ctx)

	contentID := uuid.MustParse("8a51a9b1-2d30-4b2c-8ecd-2c0b87dfa999")
	logger.Info("content.created",
		"content_id", contentID,
		"publish_at", time.Date(2024, 3, 15, 8, 0, 0, 0, time.UTC),
	)

	got := strings.TrimSpace(buf.String())
	want := "2024-03-14T15:09:26.535897Z INFO content.created content_id=8a51a9b1-2d30-4b2c-8ecd-2c0b87dfa999 correlation_id=req-1234 logger=cms.content module=cms.content publish_at=2024-03-15T08:00:00Z"
	if got != want {
		t.Fatalf("unexpected log entry\nwant: %s\ngot:  %s", want, got)
	}
}

func TestConsoleLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	minLevel := console.LevelInfo
	provider := console.NewProvider(console.Options{
		Writer:   &buf,
		TimeFunc: time.Now,
		MinLevel: &minLevel,
	})

	logger := provider.GetLogger("cms.test")
	logger.Debug("ignored.debug", "foo", "bar")
	logger.Info("included.info", "foo", "bar")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected single log line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "included.info") {
		t.Fatalf("expected info log to be written, got %s", lines[0])
	}
	if strings.Contains(lines[0], "ignored.debug") {
		t.Fatalf("unexpected debug log present: %s", lines[0])
	}
}
