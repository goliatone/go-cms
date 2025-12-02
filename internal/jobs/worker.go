package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/pages"
	cmsscheduler "github.com/goliatone/go-cms/internal/scheduler"
	"github.com/goliatone/go-cms/pkg/activity"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

type ContentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*content.Content, error)
	Update(ctx context.Context, record *content.Content) (*content.Content, error)
}

type PageRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*pages.Page, error)
	Update(ctx context.Context, record *pages.Page) (*pages.Page, error)
}

type Worker struct {
	scheduler interfaces.Scheduler
	contents  ContentRepository
	pages     PageRepository
	audit     AuditRecorder
	activity  *activity.Emitter
	now       func() time.Time
	batchSize int
}

type Option func(*Worker)

func WithAuditRecorder(recorder AuditRecorder) Option {
	return func(w *Worker) {
		w.audit = recorder
	}
}

func WithActivityEmitter(emitter *activity.Emitter) Option {
	return func(w *Worker) {
		if emitter != nil {
			w.activity = emitter
		}
	}
}

func WithClock(clock func() time.Time) Option {
	return func(w *Worker) {
		if clock != nil {
			w.now = clock
		}
	}
}

func WithBatchSize(size int) Option {
	return func(w *Worker) {
		if size > 0 {
			w.batchSize = size
		}
	}
}

func NewWorker(scheduler interfaces.Scheduler, contents ContentRepository, pagesRepo PageRepository, opts ...Option) *Worker {
	w := &Worker{
		scheduler: scheduler,
		contents:  contents,
		pages:     pagesRepo,
		now:       time.Now,
		batchSize: 50,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *Worker) emitActivity(ctx context.Context, actor *uuid.UUID, verb, objectType string, objectID uuid.UUID, meta map[string]any) {
	if w.activity == nil || !w.activity.Enabled() || objectID == uuid.Nil {
		return
	}
	actorID := uuid.Nil
	if actor != nil {
		actorID = *actor
	}
	event := activity.Event{
		Verb:       verb,
		ActorID:    actorID.String(),
		ObjectType: objectType,
		ObjectID:   objectID.String(),
		Metadata:   meta,
	}
	_ = w.activity.Emit(ctx, event)
}

func (w *Worker) Process(ctx context.Context) error {
	if w.scheduler == nil {
		return errors.New("jobs: scheduler is nil")
	}
	deadline := w.now()
	jobs, err := w.scheduler.ListDue(ctx, deadline, w.batchSize)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job == nil {
			continue
		}
		if err := w.handleJob(ctx, job, deadline); err != nil {
			_ = w.scheduler.MarkFailed(ctx, job.ID, err)
			continue
		}
		_ = w.scheduler.MarkDone(ctx, job.ID)
	}
	return nil
}

func (w *Worker) handleJob(ctx context.Context, job *interfaces.Job, now time.Time) error {
	switch job.Type {
	case cmsscheduler.JobTypeContentPublish:
		return w.processContentPublish(ctx, job, now)
	case cmsscheduler.JobTypeContentUnpublish:
		return w.processContentUnpublish(ctx, job, now)
	case cmsscheduler.JobTypePagePublish:
		return w.processPagePublish(ctx, job, now)
	case cmsscheduler.JobTypePageUnpublish:
		return w.processPageUnpublish(ctx, job, now)
	default:
		return nil
	}
}

func (w *Worker) processContentPublish(ctx context.Context, job *interfaces.Job, now time.Time) error {
	if w.contents == nil {
		return errors.New("jobs: content repository is nil")
	}
	id, triggeredBy, err := parseJobIdentifiers(job.Payload, "content_id")
	if err != nil {
		return err
	}
	record, err := w.contents.GetByID(ctx, id)
	if err != nil {
		return err
	}
	originalStatus := determineContentStatus(record, now)
	statusChanged := originalStatus != domain.StatusPublished
	if record.PublishAt != nil {
		record.PublishAt = nil
		statusChanged = true
	}
	if statusChanged {
		record.Status = string(domain.StatusPublished)
		publishedAt := job.RunAt
		if publishedAt.IsZero() {
			publishedAt = now
		}
		record.PublishedAt = &publishedAt
		record.UpdatedAt = now
		if triggeredBy != nil {
			record.PublishedBy = triggeredBy
			record.UpdatedBy = *triggeredBy
		}
		if _, err := w.contents.Update(ctx, record); err != nil {
			return err
		}
		w.recordAudit(ctx, AuditEvent{
			EntityType: "content",
			EntityID:   id.String(),
			Action:     "publish",
			OccurredAt: now,
			Metadata:   buildAuditMetadata(job, triggeredBy),
		})
	}
	w.emitActivity(ctx, triggeredBy, "publish", "content", id, map[string]any{
		"job_id":       job.ID,
		"job_type":     job.Type,
		"status":       record.Status,
		"published_at": record.PublishedAt,
	})
	return nil
}

func (w *Worker) processContentUnpublish(ctx context.Context, job *interfaces.Job, now time.Time) error {
	if w.contents == nil {
		return errors.New("jobs: content repository is nil")
	}
	id, triggeredBy, err := parseJobIdentifiers(job.Payload, "content_id")
	if err != nil {
		return err
	}
	record, err := w.contents.GetByID(ctx, id)
	if err != nil {
		return err
	}
	originalStatus := determineContentStatus(record, now)
	statusChanged := originalStatus == domain.StatusPublished
	if record.UnpublishAt != nil {
		record.UnpublishAt = nil
		statusChanged = true
	}
	if statusChanged {
		record.Status = string(domain.StatusArchived)
		record.UpdatedAt = now
		if triggeredBy != nil {
			record.UpdatedBy = *triggeredBy
		}
		if _, err := w.contents.Update(ctx, record); err != nil {
			return err
		}
		w.recordAudit(ctx, AuditEvent{
			EntityType: "content",
			EntityID:   id.String(),
			Action:     "unpublish",
			OccurredAt: now,
			Metadata:   buildAuditMetadata(job, triggeredBy),
		})
	}
	w.emitActivity(ctx, triggeredBy, "unpublish", "content", id, map[string]any{
		"job_id":   job.ID,
		"job_type": job.Type,
		"status":   record.Status,
	})
	return nil
}

func (w *Worker) processPagePublish(ctx context.Context, job *interfaces.Job, now time.Time) error {
	if w.pages == nil {
		return errors.New("jobs: page repository is nil")
	}
	id, triggeredBy, err := parseJobIdentifiers(job.Payload, "page_id")
	if err != nil {
		return err
	}
	record, err := w.pages.GetByID(ctx, id)
	if err != nil {
		return err
	}
	originalStatus := determinePageStatus(record, now)
	statusChanged := originalStatus != domain.StatusPublished
	if record.PublishAt != nil {
		record.PublishAt = nil
		statusChanged = true
	}
	if statusChanged {
		record.Status = string(domain.StatusPublished)
		publishedAt := job.RunAt
		if publishedAt.IsZero() {
			publishedAt = now
		}
		record.PublishedAt = &publishedAt
		record.UpdatedAt = now
		if triggeredBy != nil {
			record.PublishedBy = triggeredBy
			record.UpdatedBy = *triggeredBy
		}
		if _, err := w.pages.Update(ctx, record); err != nil {
			return err
		}
		w.recordAudit(ctx, AuditEvent{
			EntityType: "page",
			EntityID:   id.String(),
			Action:     "publish",
			OccurredAt: now,
			Metadata:   buildAuditMetadata(job, triggeredBy),
		})
	}
	w.emitActivity(ctx, triggeredBy, "publish", "page", id, map[string]any{
		"job_id":       job.ID,
		"job_type":     job.Type,
		"status":       record.Status,
		"published_at": record.PublishedAt,
	})
	return nil
}

func (w *Worker) processPageUnpublish(ctx context.Context, job *interfaces.Job, now time.Time) error {
	if w.pages == nil {
		return errors.New("jobs: page repository is nil")
	}
	id, triggeredBy, err := parseJobIdentifiers(job.Payload, "page_id")
	if err != nil {
		return err
	}
	record, err := w.pages.GetByID(ctx, id)
	if err != nil {
		return err
	}
	originalStatus := determinePageStatus(record, now)
	statusChanged := originalStatus == domain.StatusPublished
	if record.UnpublishAt != nil {
		record.UnpublishAt = nil
		statusChanged = true
	}
	if statusChanged {
		record.Status = string(domain.StatusArchived)
		record.UpdatedAt = now
		if triggeredBy != nil {
			record.UpdatedBy = *triggeredBy
		}
		if _, err := w.pages.Update(ctx, record); err != nil {
			return err
		}
		w.recordAudit(ctx, AuditEvent{
			EntityType: "page",
			EntityID:   id.String(),
			Action:     "unpublish",
			OccurredAt: now,
			Metadata:   buildAuditMetadata(job, triggeredBy),
		})
	}
	w.emitActivity(ctx, triggeredBy, "unpublish", "page", id, map[string]any{
		"job_id":   job.ID,
		"job_type": job.Type,
		"status":   record.Status,
	})
	return nil
}

func (w *Worker) recordAudit(ctx context.Context, event AuditEvent) {
	if w.audit == nil {
		return
	}
	_ = w.audit.Record(ctx, event)
}

func parseJobIdentifiers(payload map[string]any, key string) (uuid.UUID, *uuid.UUID, error) {
	if payload == nil {
		return uuid.Nil, nil, fmt.Errorf("jobs: missing payload")
	}
	rawID, ok := payload[key]
	if !ok {
		return uuid.Nil, nil, fmt.Errorf("jobs: payload missing %s", key)
	}
	idStr, ok := rawID.(string)
	if !ok {
		return uuid.Nil, nil, fmt.Errorf("jobs: invalid %s payload", key)
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, nil, err
	}
	var triggeredBy *uuid.UUID
	if rawScheduledBy, ok := payload["scheduled_by"]; ok {
		if str, ok := rawScheduledBy.(string); ok {
			if parsed, err := uuid.Parse(str); err == nil {
				triggeredBy = &parsed
			}
		}
	}
	return id, triggeredBy, nil
}

func buildAuditMetadata(job *interfaces.Job, triggeredBy *uuid.UUID) map[string]any {
	meta := map[string]any{
		"job_id":   job.ID,
		"job_type": job.Type,
		"run_at":   job.RunAt,
		"attempt":  job.Attempt,
	}
	if triggeredBy != nil {
		meta["scheduled_by"] = triggeredBy.String()
	}
	return meta
}

func determineContentStatus(record *content.Content, now time.Time) domain.Status {
	if record == nil {
		return domain.StatusDraft
	}
	status := domain.Status(record.Status)
	if record.UnpublishAt != nil && !record.UnpublishAt.After(now) {
		return domain.StatusArchived
	}
	if record.PublishAt != nil {
		if record.PublishAt.After(now) {
			return domain.StatusScheduled
		}
		return domain.StatusPublished
	}
	if record.PublishedAt != nil && !record.PublishedAt.After(now) {
		return domain.StatusPublished
	}
	if status == "" {
		return domain.StatusDraft
	}
	return status
}

func determinePageStatus(record *pages.Page, now time.Time) domain.Status {
	if record == nil {
		return domain.StatusDraft
	}
	status := domain.Status(record.Status)
	if record.UnpublishAt != nil && !record.UnpublishAt.After(now) {
		return domain.StatusArchived
	}
	if record.PublishAt != nil {
		if record.PublishAt.After(now) {
			return domain.StatusScheduled
		}
		return domain.StatusPublished
	}
	if record.PublishedAt != nil && !record.PublishedAt.After(now) {
		return domain.StatusPublished
	}
	if status == "" {
		return domain.StatusDraft
	}
	return status
}
