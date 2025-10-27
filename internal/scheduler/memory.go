package scheduler

import (
	"context"
	"errors"
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

const defaultMaxAttempts = 3

// NewInMemory creates a deterministic scheduler implementation suitable for tests.
func NewInMemory(opts ...Option) interfaces.Scheduler {
	mem := &inMemoryScheduler{
		now:        time.Now,
		id:         func() string { return uuid.NewString() },
		jobs:       make(map[string]*interfaces.Job),
		jobKeys:    make(map[string]string),
		maxAttempt: defaultMaxAttempts,
	}
	for _, opt := range opts {
		opt(mem)
	}
	return mem
}

// Option allows customizing the behaviour of the in-memory scheduler.
type Option func(*inMemoryScheduler)

// WithClock overrides the internal clock, used mainly for tests.
func WithClock(clock func() time.Time) Option {
	return func(s *inMemoryScheduler) {
		if clock != nil {
			s.now = clock
		}
	}
}

// WithIDGenerator overrides the ID generator used when enqueuing jobs.
func WithIDGenerator(generator func() string) Option {
	return func(s *inMemoryScheduler) {
		if generator != nil {
			s.id = generator
		}
	}
}

// WithDefaultMaxAttempts overrides the default retry attempts applied when the job spec leaves it unset.
func WithDefaultMaxAttempts(limit int) Option {
	return func(s *inMemoryScheduler) {
		if limit > 0 {
			s.maxAttempt = limit
		}
	}
}

type inMemoryScheduler struct {
	mu         sync.Mutex
	now        func() time.Time
	id         func() string
	maxAttempt int
	jobs       map[string]*interfaces.Job
	jobKeys    map[string]string
}

func (s *inMemoryScheduler) Enqueue(_ context.Context, spec interfaces.JobSpec) (*interfaces.Job, error) {
	if spec.RunAt.IsZero() {
		return nil, errors.New("scheduler: run_at is required")
	}
	job := &interfaces.Job{
		JobSpec: interfaces.JobSpec{
			Key:         spec.Key,
			Type:        spec.Type,
			RunAt:       spec.RunAt,
			Payload:     clonePayload(spec.Payload),
			MaxAttempts: spec.MaxAttempts,
		},
	}
	if job.MaxAttempts == 0 {
		job.MaxAttempts = s.maxAttempt
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if job.Key != "" {
		if existingID, ok := s.jobKeys[job.Key]; ok {
			delete(s.jobs, existingID)
		}
	}

	job.ID = s.id()
	now := s.now()
	job.Status = interfaces.JobStatusPending
	job.CreatedAt = now
	job.UpdatedAt = now

	s.jobs[job.ID] = job
	if job.Key != "" {
		s.jobKeys[job.Key] = job.ID
	}

	return cloneJob(job), nil
}

func (s *inMemoryScheduler) Cancel(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return interfaces.ErrJobNotFound
	}
	job.Status = interfaces.JobStatusCanceled
	job.UpdatedAt = s.now()
	if job.Key != "" {
		delete(s.jobKeys, job.Key)
	}
	return nil
}

func (s *inMemoryScheduler) CancelByKey(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.jobKeys[key]
	if !ok {
		return interfaces.ErrJobNotFound
	}
	job := s.jobs[id]
	if job == nil {
		return interfaces.ErrJobNotFound
	}
	job.Status = interfaces.JobStatusCanceled
	job.UpdatedAt = s.now()
	delete(s.jobKeys, key)
	return nil
}

func (s *inMemoryScheduler) Get(_ context.Context, id string) (*interfaces.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return nil, interfaces.ErrJobNotFound
	}
	return cloneJob(job), nil
}

func (s *inMemoryScheduler) GetByKey(_ context.Context, key string) (*interfaces.Job, error) {
	if key == "" {
		return nil, interfaces.ErrJobNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.jobKeys[key]
	if !ok {
		return nil, interfaces.ErrJobNotFound
	}
	job, ok := s.jobs[id]
	if !ok {
		return nil, interfaces.ErrJobNotFound
	}
	return cloneJob(job), nil
}

func (s *inMemoryScheduler) ListDue(_ context.Context, until time.Time, limit int) ([]*interfaces.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = len(s.jobs)
	}
	candidates := make([]*interfaces.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		if job.Status != interfaces.JobStatusPending {
			continue
		}
		if job.RunAt.After(until) {
			continue
		}
		candidates = append(candidates, cloneJob(job))
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].RunAt.Equal(candidates[j].RunAt) {
			return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
		}
		return candidates[i].RunAt.Before(candidates[j].RunAt)
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func (s *inMemoryScheduler) MarkDone(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return interfaces.ErrJobNotFound
	}
	job.Status = interfaces.JobStatusCompleted
	job.UpdatedAt = s.now()
	if job.Key != "" {
		delete(s.jobKeys, job.Key)
	}
	return nil
}

func (s *inMemoryScheduler) MarkFailed(_ context.Context, id string, failure error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return interfaces.ErrJobNotFound
	}
	job.Attempt++
	job.UpdatedAt = s.now()
	job.LastError = ""
	if failure != nil {
		job.LastError = failure.Error()
	}
	if job.MaxAttempts > 0 && job.Attempt >= job.MaxAttempts {
		job.Status = interfaces.JobStatusFailed
	} else {
		job.Status = interfaces.JobStatusPending
	}
	return nil
}

func cloneJob(job *interfaces.Job) *interfaces.Job {
	if job == nil {
		return nil
	}
	clone := *job
	if job.Payload != nil {
		clone.Payload = maps.Clone(job.Payload)
	}
	return &clone
}

func clonePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	return maps.Clone(payload)
}
