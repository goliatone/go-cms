package contentcmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/logging"
	goerrors "github.com/goliatone/go-errors"
	"github.com/google/uuid"
)

type stubContentService struct {
	publishRequests []content.PublishContentDraftRequest
	publishErr      error
}

func (s *stubContentService) Create(context.Context, content.CreateContentRequest) (*content.Content, error) {
	return nil, errors.New("not implemented")
}

func (s *stubContentService) Get(context.Context, uuid.UUID) (*content.Content, error) {
	return nil, errors.New("not implemented")
}

func (s *stubContentService) List(context.Context) ([]*content.Content, error) {
	return nil, errors.New("not implemented")
}

func (s *stubContentService) Schedule(context.Context, content.ScheduleContentRequest) (*content.Content, error) {
	return nil, errors.New("not implemented")
}

func (s *stubContentService) CreateDraft(context.Context, content.CreateContentDraftRequest) (*content.ContentVersion, error) {
	return nil, errors.New("not implemented")
}

func (s *stubContentService) PublishDraft(ctx context.Context, req content.PublishContentDraftRequest) (*content.ContentVersion, error) {
	s.publishRequests = append(s.publishRequests, req)
	if s.publishErr != nil {
		return nil, s.publishErr
	}
	return &content.ContentVersion{Version: req.Version}, nil
}

func (s *stubContentService) ListVersions(context.Context, uuid.UUID) ([]*content.ContentVersion, error) {
	return nil, errors.New("not implemented")
}

func (s *stubContentService) RestoreVersion(context.Context, content.RestoreContentVersionRequest) (*content.ContentVersion, error) {
	return nil, errors.New("not implemented")
}

func TestPublishContentHandlerExecutesService(t *testing.T) {
	service := &stubContentService{}
	logger := commands.CommandLogger(nil, "content")
	handler := NewPublishContentHandler(service, logger)

	contentID := uuid.New()
	publishedBy := uuid.New()
	timestamp := time.Now().UTC()
	msg := PublishContentCommand{
		ContentID:   contentID,
		Version:     3,
		PublishedBy: &publishedBy,
		PublishedAt: &timestamp,
	}

	if err := handler.Execute(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(service.publishRequests) != 1 {
		t.Fatalf("expected one publish request, got %d", len(service.publishRequests))
	}
	req := service.publishRequests[0]
	if req.ContentID != contentID {
		t.Fatalf("expected content id %s, got %s", contentID, req.ContentID)
	}
	if req.Version != 3 {
		t.Fatalf("expected version 3, got %d", req.Version)
	}
	if req.PublishedBy != publishedBy {
		t.Fatalf("expected published_by %s, got %s", publishedBy, req.PublishedBy)
	}
	if req.PublishedAt == nil || !req.PublishedAt.Equal(timestamp) {
		t.Fatalf("expected published_at %v, got %v", timestamp, req.PublishedAt)
	}
}

func TestPublishContentHandlerValidationError(t *testing.T) {
	service := &stubContentService{}
	logger := logging.NoOp()
	handler := NewPublishContentHandler(service, logger)

	err := handler.Execute(context.Background(), PublishContentCommand{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryValidation) {
		t.Fatalf("expected validation category, got %v", err)
	}
	if len(service.publishRequests) != 0 {
		t.Fatalf("expected no publish attempts, got %d", len(service.publishRequests))
	}
}
