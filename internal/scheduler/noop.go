package scheduler

import (
	"context"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// NewNoOp returns a scheduler implementation that drops every request.
func NewNoOp() interfaces.Scheduler {
	return noOpScheduler{}
}

type noOpScheduler struct{}

func (noOpScheduler) Enqueue(_ context.Context, spec interfaces.JobSpec) (*interfaces.Job, error) {
	job := &interfaces.Job{
		JobSpec: spec,
		Status:  interfaces.JobStatusCompleted,
	}
	return job, nil
}

func (noOpScheduler) Cancel(context.Context, string) error {
	return nil
}

func (noOpScheduler) CancelByKey(context.Context, string) error {
	return nil
}

func (noOpScheduler) Get(context.Context, string) (*interfaces.Job, error) {
	return nil, interfaces.ErrJobNotFound
}

func (noOpScheduler) GetByKey(context.Context, string) (*interfaces.Job, error) {
	return nil, interfaces.ErrJobNotFound
}

func (noOpScheduler) ListDue(context.Context, time.Time, int) ([]*interfaces.Job, error) {
	return nil, nil
}

func (noOpScheduler) MarkDone(context.Context, string) error {
	return nil
}

func (noOpScheduler) MarkFailed(context.Context, string, error) error {
	return nil
}
