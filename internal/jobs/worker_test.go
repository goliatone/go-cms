package jobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/pages"
	cmsscheduler "github.com/goliatone/go-cms/internal/scheduler"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

func TestWorkerProcessContentPublish(t *testing.T) {
	ctx := context.Background()
	scheduler := cmsscheduler.NewInMemory()
	contentRepo := content.NewMemoryContentRepository()
	pageRepo := pages.NewMemoryPageRepository()
	audit := jobs.NewInMemoryAuditRecorder()
	now := time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC)
	worker := jobs.NewWorker(scheduler, contentRepo, pageRepo, jobs.WithAuditRecorder(audit), jobs.WithClock(func() time.Time { return now }))

	contentID := uuid.New()
	userID := uuid.New()
	record := &content.Content{
		ID:            contentID,
		ContentTypeID: uuid.New(),
		Status:        string(domain.StatusScheduled),
		Slug:          "about",
		PublishAt:     ptrTime(now.Add(-time.Minute)),
		UpdatedAt:     now.Add(-time.Hour),
		UpdatedBy:     userID,
	}
	if _, err := contentRepo.Create(ctx, record); err != nil {
		t.Fatalf("create content: %v", err)
	}

	jobPayload := map[string]any{
		"content_id":   contentID.String(),
		"scheduled_by": userID.String(),
	}
	enqueued, err := scheduler.Enqueue(ctx, interfaces.JobSpec{
		Key:     cmsscheduler.ContentPublishJobKey(contentID),
		Type:    cmsscheduler.JobTypeContentPublish,
		RunAt:   now.Add(-time.Minute),
		Payload: jobPayload,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := worker.Process(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}

	updated, err := contentRepo.GetByID(ctx, contentID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if updated.Status != string(domain.StatusPublished) {
		t.Fatalf("expected published status, got %s", updated.Status)
	}
	if updated.PublishAt != nil {
		t.Fatalf("expected publish_at cleared")
	}
	if updated.PublishedAt == nil || !updated.PublishedAt.Equal(now.Add(-time.Minute)) {
		t.Fatalf("unexpected published_at %v", updated.PublishedAt)
	}

	auditEvents := audit.Events()
	if len(auditEvents) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(auditEvents))
	}
	if auditEvents[0].Action != "publish" {
		t.Fatalf("expected publish action, got %s", auditEvents[0].Action)
	}

	stored, err := scheduler.Get(ctx, enqueued.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if stored.Status != interfaces.JobStatusCompleted {
		t.Fatalf("expected job completed, got %s", stored.Status)
	}
}

func TestWorkerProcessContentUnpublish(t *testing.T) {
	ctx := context.Background()
	scheduler := cmsscheduler.NewInMemory()
	contentRepo := content.NewMemoryContentRepository()
	pageRepo := pages.NewMemoryPageRepository()
	audit := jobs.NewInMemoryAuditRecorder()
	now := time.Date(2024, 6, 1, 8, 0, 0, 0, time.UTC)
	worker := jobs.NewWorker(scheduler, contentRepo, pageRepo, jobs.WithAuditRecorder(audit), jobs.WithClock(func() time.Time { return now }))

	contentID := uuid.New()
	userID := uuid.New()
	publishedAt := now.Add(-2 * time.Hour)
	record := &content.Content{
		ID:            contentID,
		ContentTypeID: uuid.New(),
		Status:        string(domain.StatusPublished),
		Slug:          "news",
		PublishedAt:   &publishedAt,
		UnpublishAt:   ptrTime(now.Add(-time.Minute)),
		UpdatedAt:     now,
		UpdatedBy:     userID,
	}
	if _, err := contentRepo.Create(ctx, record); err != nil {
		t.Fatalf("create content: %v", err)
	}

	if _, err := scheduler.Enqueue(ctx, interfaces.JobSpec{
		Key:     cmsscheduler.ContentUnpublishJobKey(contentID),
		Type:    cmsscheduler.JobTypeContentUnpublish,
		RunAt:   now.Add(-time.Minute),
		Payload: map[string]any{"content_id": contentID.String(), "scheduled_by": userID.String()},
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := worker.Process(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}

	updated, err := contentRepo.GetByID(ctx, contentID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if updated.Status != string(domain.StatusArchived) {
		t.Fatalf("expected archived status, got %s", updated.Status)
	}
	if updated.UnpublishAt != nil {
		t.Fatalf("expected unpublish_at cleared")
	}

	if len(audit.Events()) != 1 || audit.Events()[0].Action != "unpublish" {
		t.Fatalf("expected unpublish audit event")
	}
}

func TestWorkerProcessPagePublishAndUnpublish(t *testing.T) {
	ctx := context.Background()
	scheduler := cmsscheduler.NewInMemory()
	contentRepo := content.NewMemoryContentRepository()
	pageRepo := pages.NewMemoryPageRepository()
	audit := jobs.NewInMemoryAuditRecorder()
	now := time.Date(2024, 7, 10, 9, 0, 0, 0, time.UTC)
	worker := jobs.NewWorker(scheduler, contentRepo, pageRepo, jobs.WithAuditRecorder(audit), jobs.WithClock(func() time.Time { return now }))

	pageID := uuid.New()
	contentID := uuid.New()
	userID := uuid.New()
	pageRecord := &pages.Page{
		ID:         pageID,
		ContentID:  contentID,
		TemplateID: uuid.New(),
		Slug:       "home",
		Status:     string(domain.StatusScheduled),
		PublishAt:  ptrTime(now.Add(-time.Minute)),
		UpdatedAt:  now,
		UpdatedBy:  userID,
	}
	if _, err := pageRepo.Create(ctx, pageRecord); err != nil {
		t.Fatalf("create page: %v", err)
	}

	if _, err := scheduler.Enqueue(ctx, interfaces.JobSpec{
		Key:     cmsscheduler.PagePublishJobKey(pageID),
		Type:    cmsscheduler.JobTypePagePublish,
		RunAt:   now.Add(-time.Minute),
		Payload: map[string]any{"page_id": pageID.String(), "scheduled_by": userID.String()},
	}); err != nil {
		t.Fatalf("enqueue publish: %v", err)
	}

	if err := worker.Process(ctx); err != nil {
		t.Fatalf("process publish: %v", err)
	}

	pageUpdated, err := pageRepo.GetByID(ctx, pageID)
	if err != nil {
		t.Fatalf("get page: %v", err)
	}
	if pageUpdated.Status != string(domain.StatusPublished) {
		t.Fatalf("expected published status, got %s", pageUpdated.Status)
	}

	pageUpdated.UnpublishAt = ptrTime(now.Add(-time.Minute))
	if _, err := pageRepo.Update(ctx, pageUpdated); err != nil {
		t.Fatalf("update page: %v", err)
	}

	if _, err := scheduler.Enqueue(ctx, interfaces.JobSpec{
		Key:     cmsscheduler.PageUnpublishJobKey(pageID),
		Type:    cmsscheduler.JobTypePageUnpublish,
		RunAt:   now.Add(-time.Minute),
		Payload: map[string]any{"page_id": pageID.String(), "scheduled_by": userID.String()},
	}); err != nil {
		t.Fatalf("enqueue unpublish: %v", err)
	}

	if err := worker.Process(ctx); err != nil {
		t.Fatalf("process unpublish: %v", err)
	}

	pageUpdated, err = pageRepo.GetByID(ctx, pageID)
	if err != nil {
		t.Fatalf("get page after unpublish: %v", err)
	}
	if pageUpdated.Status != string(domain.StatusArchived) {
		t.Fatalf("expected archived status, got %s", pageUpdated.Status)
	}
	if len(audit.Events()) < 2 {
		t.Fatalf("expected at least 2 audit events, got %d", len(audit.Events()))
	}
}

func TestSchedulingCancellation(t *testing.T) {
	ctx := context.Background()
	scheduler := cmsscheduler.NewInMemory()
	contentRepo := content.NewMemoryContentRepository()
	contentTypeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	typeID := uuid.New()
	if err := contentTypeRepo.Put(&content.ContentType{ID: typeID, Name: "article"}); err != nil {
		t.Fatalf("seed content type: %v", err)
	}
	localeRepo.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	svc := content.NewService(
		contentRepo,
		contentTypeRepo,
		localeRepo,
		content.WithScheduler(scheduler),
		content.WithSchedulingEnabled(true),
	)

	record, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "cancel",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Cancel"},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}
	publishAt := time.Now().Add(time.Hour)
	if _, err := svc.Schedule(ctx, content.ScheduleContentRequest{ContentID: record.ID, PublishAt: &publishAt}); err != nil {
		t.Fatalf("schedule publish: %v", err)
	}
	if _, err := svc.Schedule(ctx, content.ScheduleContentRequest{ContentID: record.ID}); err != nil {
		t.Fatalf("cancel schedule: %v", err)
	}

	if _, err := scheduler.GetByKey(ctx, cmsscheduler.ContentPublishJobKey(record.ID)); !errors.Is(err, interfaces.ErrJobNotFound) {
		t.Fatalf("expected publish job removal, got %v", err)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
