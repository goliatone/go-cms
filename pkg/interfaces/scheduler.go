package interfaces

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrJobNotFound reports missing jobs when looking them up by ID or key.
	ErrJobNotFound = errors.New("scheduler: job not found")
)

// Scheduler coordinates delayed execution of jobs such as publish/unpublish actions.
type Scheduler interface {
	// Enqueue registers a job for future execution. If a job with the same key already exists,
	// it is replaced with the new definition to keep scheduling idempotent.
	Enqueue(ctx context.Context, spec JobSpec) (*Job, error)
	// Cancel marks the job as cancelled so it will not be executed.
	Cancel(ctx context.Context, id string) error
	// CancelByKey cancels the job associated to the supplied unique key.
	CancelByKey(ctx context.Context, key string) error
	// Get returns the stored job by identifier.
	Get(ctx context.Context, id string) (*Job, error)
	// GetByKey returns the stored job that matches the supplied key.
	GetByKey(ctx context.Context, key string) (*Job, error)
	// ListDue returns pending jobs scheduled to run at or before the supplied instant.
	ListDue(ctx context.Context, until time.Time, limit int) ([]*Job, error)
	// MarkDone marks the job as successfully processed.
	MarkDone(ctx context.Context, id string) error
	// MarkFailed updates the job after a failed attempt.
	MarkFailed(ctx context.Context, id string, err error) error
}

// JobStatus describes the lifecycle of a scheduled job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusCompleted JobStatus = "completed"
	JobStatusCanceled  JobStatus = "canceled"
	JobStatusFailed    JobStatus = "failed"
)

// JobSpec captures the required information to enqueue a job.
type JobSpec struct {
	// Key uniquely identifies the job so that new requests can safely replace existing entries.
	Key string
	// Type describes the action to perform (e.g. cms.content.publish).
	Type string
	// RunAt specifies when the job should execute.
	RunAt time.Time
	// Payload carries contextual data required by the worker.
	Payload map[string]any
	// MaxAttempts limits retries when a worker reports failure. Zero means unlimited.
	MaxAttempts int
}

// Job represents a stored job entry with metadata managed by the scheduler implementation.
type Job struct {
	JobSpec
	ID        string
	Attempt   int
	LastError string
	Status    JobStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}
