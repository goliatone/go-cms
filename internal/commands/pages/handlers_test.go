package pagescmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/pages"
	goerrors "github.com/goliatone/go-errors"
	"github.com/google/uuid"
)

type stubPageService struct {
	publishRequests  []pages.PublishPagePublishRequest
	scheduleRequests []pages.SchedulePageRequest
	restoreRequests  []pages.RestorePageVersionRequest
	deleteRequests   []pages.DeletePageRequest

	publishErr  error
	scheduleErr error
	restoreErr  error
	deleteErr   error
}

func (s *stubPageService) Create(context.Context, pages.CreatePageRequest) (*pages.Page, error) {
	return nil, errors.New("not implemented")
}

func (s *stubPageService) Get(context.Context, uuid.UUID) (*pages.Page, error) {
	return nil, errors.New("not implemented")
}

func (s *stubPageService) List(context.Context) ([]*pages.Page, error) {
	return nil, errors.New("not implemented")
}

func (s *stubPageService) Update(context.Context, pages.UpdatePageRequest) (*pages.Page, error) {
	return nil, errors.New("not implemented")
}

func (s *stubPageService) Delete(ctx context.Context, req pages.DeletePageRequest) error {
	s.deleteRequests = append(s.deleteRequests, req)
	if s.deleteErr != nil {
		return s.deleteErr
	}
	return nil
}

func (s *stubPageService) Schedule(ctx context.Context, req pages.SchedulePageRequest) (*pages.Page, error) {
	s.scheduleRequests = append(s.scheduleRequests, req)
	if s.scheduleErr != nil {
		return nil, s.scheduleErr
	}
	return &pages.Page{ID: req.PageID}, nil
}

func (s *stubPageService) CreateDraft(context.Context, pages.CreatePageDraftRequest) (*pages.PageVersion, error) {
	return nil, errors.New("not implemented")
}

func (s *stubPageService) PublishDraft(ctx context.Context, req pages.PublishPagePublishRequest) (*pages.PageVersion, error) {
	s.publishRequests = append(s.publishRequests, req)
	if s.publishErr != nil {
		return nil, s.publishErr
	}
	return &pages.PageVersion{Version: req.Version}, nil
}

func (s *stubPageService) ListVersions(context.Context, uuid.UUID) ([]*pages.PageVersion, error) {
	return nil, errors.New("not implemented")
}

func (s *stubPageService) RestoreVersion(ctx context.Context, req pages.RestorePageVersionRequest) (*pages.PageVersion, error) {
	s.restoreRequests = append(s.restoreRequests, req)
	if s.restoreErr != nil {
		return nil, s.restoreErr
	}
	return &pages.PageVersion{Version: req.Version}, nil
}

func TestPublishPageHandlerExecutesService(t *testing.T) {
	service := &stubPageService{}
	logger := commands.CommandLogger(nil, "pages")
	handler := NewPublishPageHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return true },
	})

	pageID := uuid.New()
	templateID := uuid.New()
	publishedBy := uuid.New()
	publishedAt := time.Now().UTC()
	msg := PublishPageCommand{
		PageID:      pageID,
		Version:     4,
		Locale:      "en",
		TemplateID:  &templateID,
		PublishedBy: &publishedBy,
		PublishedAt: &publishedAt,
	}

	if err := handler.Execute(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(service.publishRequests) != 1 {
		t.Fatalf("expected one publish request, got %d", len(service.publishRequests))
	}
	req := service.publishRequests[0]
	if req.PageID != pageID {
		t.Fatalf("expected page id %s, got %s", pageID, req.PageID)
	}
	if req.Version != 4 {
		t.Fatalf("expected version 4, got %d", req.Version)
	}
	if req.PublishedBy == uuid.Nil || req.PublishedBy != publishedBy {
		t.Fatalf("expected published_by %s, got %s", publishedBy, req.PublishedBy)
	}
	if req.PublishedAt == nil || !req.PublishedAt.Equal(publishedAt) {
		t.Fatalf("expected published_at %v, got %v", publishedAt, req.PublishedAt)
	}
}

func TestPublishPageHandlerValidationError(t *testing.T) {
	service := &stubPageService{}
	logger := logging.NoOp()
	handler := NewPublishPageHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return true },
	})

	err := handler.Execute(context.Background(), PublishPageCommand{})
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

func TestPublishPageHandlerFeatureDisabled(t *testing.T) {
	service := &stubPageService{}
	logger := logging.NoOp()
	handler := NewPublishPageHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return false },
	})

	msg := PublishPageCommand{
		PageID:  uuid.New(),
		Version: 1,
		Locale:  "en",
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

func TestPublishPageHandlerContextCancellation(t *testing.T) {
	service := &stubPageService{}
	logger := logging.NoOp()
	handler := NewPublishPageHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return true },
	})

	msg := PublishPageCommand{
		PageID:  uuid.New(),
		Version: 2,
		Locale:  "en",
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

func TestSchedulePageHandlerExecutesService(t *testing.T) {
	service := &stubPageService{}
	logger := commands.CommandLogger(nil, "pages")
	handler := NewSchedulePageHandler(service, logger, FeatureGates{
		SchedulingEnabled: func() bool { return true },
	})

	pageID := uuid.New()
	publishAt := time.Now().UTC().Add(2 * time.Hour)
	unpublishAt := publishAt.Add(6 * time.Hour)
	scheduledBy := uuid.New()
	msg := SchedulePageCommand{
		PageID:      pageID,
		Locale:      "en",
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
	if req.PageID != pageID {
		t.Fatalf("expected page id %s, got %s", pageID, req.PageID)
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

func TestSchedulePageHandlerValidationError(t *testing.T) {
	service := &stubPageService{}
	logger := logging.NoOp()
	handler := NewSchedulePageHandler(service, logger, FeatureGates{
		SchedulingEnabled: func() bool { return true },
	})

	err := handler.Execute(context.Background(), SchedulePageCommand{})
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

func TestSchedulePageHandlerFeatureDisabled(t *testing.T) {
	service := &stubPageService{}
	logger := logging.NoOp()
	handler := NewSchedulePageHandler(service, logger, FeatureGates{
		SchedulingEnabled: func() bool { return false },
	})

	msg := SchedulePageCommand{
		PageID: uuid.New(),
		Locale: "en",
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

func TestSchedulePageHandlerContextCancellation(t *testing.T) {
	service := &stubPageService{}
	logger := logging.NoOp()
	handler := NewSchedulePageHandler(service, logger, FeatureGates{
		SchedulingEnabled: func() bool { return true },
	})

	msg := SchedulePageCommand{
		PageID: uuid.New(),
		Locale: "en",
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

func TestRestorePageVersionHandlerExecutesService(t *testing.T) {
	service := &stubPageService{}
	logger := commands.CommandLogger(nil, "pages")
	handler := NewRestorePageVersionHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return true },
	})

	pageID := uuid.New()
	templateID := uuid.New()
	restoredBy := uuid.New()
	msg := RestorePageVersionCommand{
		PageID:     pageID,
		Version:    3,
		Locale:     "en",
		TemplateID: &templateID,
		RestoredBy: restoredBy,
	}

	if err := handler.Execute(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(service.restoreRequests) != 1 {
		t.Fatalf("expected one restore request, got %d", len(service.restoreRequests))
	}
	req := service.restoreRequests[0]
	if req.PageID != pageID {
		t.Fatalf("expected page id %s, got %s", pageID, req.PageID)
	}
	if req.Version != 3 {
		t.Fatalf("expected version 3, got %d", req.Version)
	}
	if req.RestoredBy != restoredBy {
		t.Fatalf("expected restored_by %s, got %s", restoredBy, req.RestoredBy)
	}
}

func TestRestorePageVersionHandlerValidationError(t *testing.T) {
	service := &stubPageService{}
	logger := logging.NoOp()
	handler := NewRestorePageVersionHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return true },
	})

	err := handler.Execute(context.Background(), RestorePageVersionCommand{})
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

func TestRestorePageVersionHandlerFeatureDisabled(t *testing.T) {
	service := &stubPageService{}
	logger := logging.NoOp()
	handler := NewRestorePageVersionHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return false },
	})

	msg := RestorePageVersionCommand{
		PageID:     uuid.New(),
		Version:    2,
		Locale:     "en",
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

func TestRestorePageVersionHandlerContextCancellation(t *testing.T) {
	service := &stubPageService{}
	logger := logging.NoOp()
	handler := NewRestorePageVersionHandler(service, logger, FeatureGates{
		VersioningEnabled: func() bool { return true },
	})

	msg := RestorePageVersionCommand{
		PageID:     uuid.New(),
		Version:    5,
		Locale:     "en",
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
