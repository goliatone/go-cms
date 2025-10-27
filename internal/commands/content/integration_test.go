package contentcmd

import (
	"context"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	cmsscheduler "github.com/goliatone/go-cms/internal/scheduler"
	"github.com/google/uuid"
)

func TestScheduleContentCommandIntegrationEnqueuesJobs(t *testing.T) {
	ctx := context.Background()
	scheduler := cmsscheduler.NewInMemory()

	contentRepo := content.NewMemoryContentRepository()
	typeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	now := time.Now().UTC().Truncate(time.Second)

	service := content.NewService(
		contentRepo,
		typeRepo,
		localeRepo,
		content.WithScheduler(scheduler),
		content.WithSchedulingEnabled(true),
		content.WithVersioningEnabled(true),
		content.WithClock(func() time.Time { return now }),
	)

	contentID := uuid.New()
	record := &content.Content{
		ID:             contentID,
		ContentTypeID:  uuid.New(),
		CurrentVersion: 1,
		Status:         string(domain.StatusDraft),
		Slug:           "integration-content",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
	}
	if _, err := contentRepo.Create(ctx, record); err != nil {
		t.Fatalf("seed content: %v", err)
	}

	handler := NewScheduleContentHandler(service, commands.CommandLogger(nil, "content"), FeatureGates{
		VersioningEnabled: func() bool { return true },
		SchedulingEnabled: func() bool { return true },
	})

	publishAt := now.Add(2 * time.Hour)
	unpublishAt := publishAt.Add(4 * time.Hour)
	scheduledBy := uuid.New()

	msg := ScheduleContentCommand{
		ContentID:   contentID,
		PublishAt:   &publishAt,
		UnpublishAt: &unpublishAt,
		ScheduledBy: scheduledBy,
	}

	if err := handler.Execute(ctx, msg); err != nil {
		t.Fatalf("execute schedule command: %v", err)
	}

	publishJob, err := scheduler.GetByKey(ctx, cmsscheduler.ContentPublishJobKey(contentID))
	if err != nil {
		t.Fatalf("publish job lookup: %v", err)
	}
	if publishJob.Type != cmsscheduler.JobTypeContentPublish {
		t.Fatalf("expected publish job type %s, got %s", cmsscheduler.JobTypeContentPublish, publishJob.Type)
	}
	if !publishJob.RunAt.Equal(publishAt) {
		t.Fatalf("expected publish run_at %v, got %v", publishAt, publishJob.RunAt)
	}
	if id, ok := publishJob.Payload["content_id"].(string); !ok || id != contentID.String() {
		t.Fatalf("expected publish payload content_id %s, got %#v", contentID, publishJob.Payload["content_id"])
	}

	unpublishJob, err := scheduler.GetByKey(ctx, cmsscheduler.ContentUnpublishJobKey(contentID))
	if err != nil {
		t.Fatalf("unpublish job lookup: %v", err)
	}
	if unpublishJob.Type != cmsscheduler.JobTypeContentUnpublish {
		t.Fatalf("expected unpublish job type %s, got %s", cmsscheduler.JobTypeContentUnpublish, unpublishJob.Type)
	}
	if !unpublishJob.RunAt.Equal(unpublishAt) {
		t.Fatalf("expected unpublish run_at %v, got %v", unpublishAt, unpublishJob.RunAt)
	}
	if id, ok := unpublishJob.Payload["content_id"].(string); !ok || id != contentID.String() {
		t.Fatalf("expected unpublish payload content_id %s, got %#v", contentID, unpublishJob.Payload["content_id"])
	}

	updated, err := contentRepo.GetByID(ctx, contentID)
	if err != nil {
		t.Fatalf("content lookup: %v", err)
	}
	if updated.PublishAt == nil || !updated.PublishAt.Equal(publishAt) {
		t.Fatalf("expected updated publish_at %v, got %v", publishAt, updated.PublishAt)
	}
	if updated.UnpublishAt == nil || !updated.UnpublishAt.Equal(unpublishAt) {
		t.Fatalf("expected updated unpublish_at %v, got %v", unpublishAt, updated.UnpublishAt)
	}
	if updated.Status != string(domain.StatusScheduled) {
		t.Fatalf("expected status %q, got %q", domain.StatusScheduled, updated.Status)
	}
}
