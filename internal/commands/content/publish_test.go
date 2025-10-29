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
	publishRequests  []content.PublishContentDraftRequest
	scheduleRequests []content.ScheduleContentRequest
	restoreRequests  []content.RestoreContentVersionRequest

	publishErr  error
	scheduleErr error
	restoreErr  error
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

func (s *stubContentService) Update(context.Context, content.UpdateContentRequest) (*content.Content, error) {
	return nil, errors.New("not implemented")
}

func (s *stubContentService) Delete(context.Context, content.DeleteContentRequest) error {
	return errors.New("not implemented")
}

func (s *stubContentService) Schedule(ctx context.Context, req content.ScheduleContentRequest) (*content.Content, error) {
	s.scheduleRequests = append(s.scheduleRequests, req)
	if s.scheduleErr != nil {
		return nil, s.scheduleErr
	}
	return &content.Content{ID: req.ContentID}, nil
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

func (s *stubContentService) RestoreVersion(ctx context.Context, req content.RestoreContentVersionRequest) (*content.ContentVersion, error) {
	s.restoreRequests = append(s.restoreRequests, req)
	if s.restoreErr != nil {
		return nil, s.restoreErr
	}
	return &content.ContentVersion{Version: req.Version}, nil
}

func TestPublishContentHandlerExecutesService(t *testing.T) {
	service := &stubContentService{}
	logger := commands.CommandLogger(nil, "content")
	handler := NewPublishContentHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return true },
	})

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
	handler := NewPublishContentHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return true },
	})

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

func TestPublishContentHandlerFeatureDisabled(t *testing.T) {
	service := &stubContentService{}
	logger := logging.NoOp()
	handler := NewPublishContentHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return false },
	})

	msg := PublishContentCommand{
		ContentID: uuid.New(),
		Version:   1,
	}

	err := handler.Execute(context.Background(), msg)
	if err == nil {
		t.Fatal("expected feature gate error")
	}
	if len(service.publishRequests) != 0 {
		t.Fatalf("expected no publish attempts, got %d", len(service.publishRequests))
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category when feature disabled, got %v", err)
	}
}

func TestPublishContentHandlerContextCancellation(t *testing.T) {
	service := &stubContentService{}
	logger := logging.NoOp()
	handler := NewPublishContentHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return true },
	})

	msg := PublishContentCommand{
		ContentID: uuid.New(),
		Version:   2,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := handler.Execute(ctx, msg)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category for cancellation, got %v", err)
	}
	if len(service.publishRequests) != 0 {
		t.Fatalf("expected no publish attempts, got %d", len(service.publishRequests))
	}
}

func TestScheduleContentHandlerExecutesService(t *testing.T) {
	service := &stubContentService{}
	logger := commands.CommandLogger(nil, "content")
	handler := NewScheduleContentHandler(service, logger, FeatureGates{
		SchedulingEnabled: func() bool { return true },
	})

	contentID := uuid.New()
	publishAt := time.Now().UTC().Add(time.Hour)
	unpublishAt := publishAt.Add(2 * time.Hour)
	scheduledBy := uuid.New()

	msg := ScheduleContentCommand{
		ContentID:   contentID,
		PublishAt:   &publishAt,
		UnpublishAt: &unpublishAt,
		ScheduledBy: scheduledBy,
	}

	if err := handler.Execute(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(service.scheduleRequests) != 1 {
		t.Fatalf("expected one schedule request, got %d", len(service.scheduleRequests))
	}
	req := service.scheduleRequests[0]
	if req.ContentID != contentID {
		t.Fatalf("expected content id %s, got %s", contentID, req.ContentID)
	}
	if req.PublishAt == nil || !req.PublishAt.Equal(publishAt) {
		t.Fatalf("expected publish_at %v, got %v", publishAt, req.PublishAt)
	}
	if req.UnpublishAt == nil || !req.UnpublishAt.Equal(unpublishAt) {
		t.Fatalf("expected unpublish_at %v, got %v", unpublishAt, req.UnpublishAt)
	}
	if req.ScheduledBy != scheduledBy {
		t.Fatalf("expected scheduled_by %s, got %s", scheduledBy, req.ScheduledBy)
	}
}

func TestScheduleContentHandlerValidationError(t *testing.T) {
	service := &stubContentService{}
	logger := logging.NoOp()
	handler := NewScheduleContentHandler(service, logger, FeatureGates{
		SchedulingEnabled: func() bool { return true },
	})

	err := handler.Execute(context.Background(), ScheduleContentCommand{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryValidation) {
		t.Fatalf("expected validation category, got %v", err)
	}
	if len(service.scheduleRequests) != 0 {
		t.Fatalf("expected no schedule attempts, got %d", len(service.scheduleRequests))
	}
}

func TestScheduleContentHandlerFeatureDisabled(t *testing.T) {
	service := &stubContentService{}
	logger := logging.NoOp()
	handler := NewScheduleContentHandler(service, logger, FeatureGates{
		SchedulingEnabled: func() bool { return false },
	})

	msg := ScheduleContentCommand{
		ContentID: uuid.New(),
	}

	err := handler.Execute(context.Background(), msg)
	if err == nil {
		t.Fatal("expected feature gate error")
	}
	if len(service.scheduleRequests) != 0 {
		t.Fatalf("expected no schedule attempts, got %d", len(service.scheduleRequests))
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category when feature disabled, got %v", err)
	}
}

func TestScheduleContentHandlerContextCancellation(t *testing.T) {
	service := &stubContentService{}
	logger := logging.NoOp()
	handler := NewScheduleContentHandler(service, logger, FeatureGates{
		SchedulingEnabled: func() bool { return true },
	})

	msg := ScheduleContentCommand{
		ContentID: uuid.New(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := handler.Execute(ctx, msg)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category for cancellation, got %v", err)
	}
	if len(service.scheduleRequests) != 0 {
		t.Fatalf("expected no schedule attempts, got %d", len(service.scheduleRequests))
	}
}

func TestRestoreContentVersionHandlerExecutesService(t *testing.T) {
	service := &stubContentService{}
	logger := commands.CommandLogger(nil, "content")
	handler := NewRestoreContentVersionHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return true },
	})

	contentID := uuid.New()
	restoredBy := uuid.New()

	msg := RestoreContentVersionCommand{
		ContentID:  contentID,
		Version:    5,
		RestoredBy: restoredBy,
	}

	if err := handler.Execute(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(service.restoreRequests) != 1 {
		t.Fatalf("expected one restore request, got %d", len(service.restoreRequests))
	}
	req := service.restoreRequests[0]
	if req.ContentID != contentID {
		t.Fatalf("expected content id %s, got %s", contentID, req.ContentID)
	}
	if req.Version != 5 {
		t.Fatalf("expected version 5, got %d", req.Version)
	}
	if req.RestoredBy != restoredBy {
		t.Fatalf("expected restored_by %s, got %s", restoredBy, req.RestoredBy)
	}
}

func TestRestoreContentVersionHandlerValidationError(t *testing.T) {
	service := &stubContentService{}
	logger := logging.NoOp()
	handler := NewRestoreContentVersionHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return true },
	})

	err := handler.Execute(context.Background(), RestoreContentVersionCommand{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryValidation) {
		t.Fatalf("expected validation category, got %v", err)
	}
	if len(service.restoreRequests) != 0 {
		t.Fatalf("expected no restore attempts, got %d", len(service.restoreRequests))
	}
}

func TestRestoreContentVersionHandlerFeatureDisabled(t *testing.T) {
	service := &stubContentService{}
	logger := logging.NoOp()
	handler := NewRestoreContentVersionHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return false },
	})

	msg := RestoreContentVersionCommand{
		ContentID:  uuid.New(),
		Version:    2,
		RestoredBy: uuid.New(),
	}

	err := handler.Execute(context.Background(), msg)
	if err == nil {
		t.Fatal("expected feature gate error")
	}
	if len(service.restoreRequests) != 0 {
		t.Fatalf("expected no restore attempts, got %d", len(service.restoreRequests))
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category when feature disabled, got %v", err)
	}
}

func TestRestoreContentVersionHandlerContextCancellation(t *testing.T) {
	service := &stubContentService{}
	logger := logging.NoOp()
	handler := NewRestoreContentVersionHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return true },
	})

	msg := RestoreContentVersionCommand{
		ContentID:  uuid.New(),
		Version:    3,
		RestoredBy: uuid.New(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := handler.Execute(ctx, msg)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category for cancellation, got %v", err)
	}
	if len(service.restoreRequests) != 0 {
		t.Fatalf("expected no restore attempts, got %d", len(service.restoreRequests))
	}
}
