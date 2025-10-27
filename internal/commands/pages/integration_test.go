package pagescmd

import (
	"context"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/pages"
	cmsscheduler "github.com/goliatone/go-cms/internal/scheduler"
	"github.com/google/uuid"
)

func TestSchedulePageCommandIntegrationEnqueuesJobs(t *testing.T) {
	ctx := context.Background()
	scheduler := cmsscheduler.NewInMemory()

	pageRepo := pages.NewMemoryPageRepository()
	contentRepo := content.NewMemoryContentRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	now := time.Now().UTC().Truncate(time.Second)

	service := pages.NewService(
		pageRepo,
		contentRepo,
		localeRepo,
		pages.WithScheduler(scheduler),
		pages.WithSchedulingEnabled(true),
		pages.WithPageVersioningEnabled(true),
		pages.WithPageClock(func() time.Time { return now }),
	)

	pageID := uuid.New()
	templateID := uuid.New()
	record := &pages.Page{
		ID:             pageID,
		ContentID:      uuid.New(),
		TemplateID:     templateID,
		Slug:           "integration-page",
		Status:         string(domain.StatusDraft),
		CurrentVersion: 1,
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
	}
	if _, err := pageRepo.Create(ctx, record); err != nil {
		t.Fatalf("seed page: %v", err)
	}

	handler := NewSchedulePageHandler(service, commands.CommandLogger(nil, "pages"), FeatureGates{
		SchedulingEnabled: func() bool { return true },
	})

	publishAt := now.Add(3 * time.Hour)
	unpublishAt := publishAt.Add(5 * time.Hour)
	scheduledBy := uuid.New()

	msg := SchedulePageCommand{
		PageID:      pageID,
		Locale:      "en",
		TemplateID:  &templateID,
		PublishAt:   &publishAt,
		UnpublishAt: &unpublishAt,
		ScheduledBy: scheduledBy,
	}

	if err := handler.Execute(ctx, msg); err != nil {
		t.Fatalf("execute schedule command: %v", err)
	}

	publishJob, err := scheduler.GetByKey(ctx, cmsscheduler.PagePublishJobKey(pageID))
	if err != nil {
		t.Fatalf("publish job lookup: %v", err)
	}
	if publishJob.Type != cmsscheduler.JobTypePagePublish {
		t.Fatalf("expected publish job type %s, got %s", cmsscheduler.JobTypePagePublish, publishJob.Type)
	}
	if !publishJob.RunAt.Equal(publishAt) {
		t.Fatalf("expected publish run_at %v, got %v", publishAt, publishJob.RunAt)
	}
	if id, ok := publishJob.Payload["page_id"].(string); !ok || id != pageID.String() {
		t.Fatalf("expected publish payload page_id %s, got %#v", pageID, publishJob.Payload["page_id"])
	}

	unpublishJob, err := scheduler.GetByKey(ctx, cmsscheduler.PageUnpublishJobKey(pageID))
	if err != nil {
		t.Fatalf("unpublish job lookup: %v", err)
	}
	if unpublishJob.Type != cmsscheduler.JobTypePageUnpublish {
		t.Fatalf("expected unpublish job type %s, got %s", cmsscheduler.JobTypePageUnpublish, unpublishJob.Type)
	}
	if !unpublishJob.RunAt.Equal(unpublishAt) {
		t.Fatalf("expected unpublish run_at %v, got %v", unpublishAt, unpublishJob.RunAt)
	}
	if id, ok := unpublishJob.Payload["page_id"].(string); !ok || id != pageID.String() {
		t.Fatalf("expected unpublish payload page_id %s, got %#v", pageID, unpublishJob.Payload["page_id"])
	}

	updated, err := pageRepo.GetByID(ctx, pageID)
	if err != nil {
		t.Fatalf("page lookup: %v", err)
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
