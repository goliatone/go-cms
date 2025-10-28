package fixtures

import (
	"github.com/goliatone/go-cms/internal/di"
	command "github.com/goliatone/go-command"
)

// RecordingRegistry captures command handlers registered through DI.
type RecordingRegistry struct {
	Handlers []any
}

// NewRecordingRegistry constructs an empty registry recorder.
func NewRecordingRegistry() *RecordingRegistry {
	return &RecordingRegistry{
		Handlers: make([]any, 0),
	}
}

// RegisterCommand satisfies di.CommandRegistry while recording the handler.
func (r *RecordingRegistry) RegisterCommand(handler any) error {
	r.Handlers = append(r.Handlers, handler)
	return nil
}

// CronRegistration captures a single cron wiring invocation.
type CronRegistration struct {
	Config  command.HandlerConfig
	Handler any
}

// CronRecorder records calls to a CronRegistrar function.
type CronRecorder struct {
	Registrations []CronRegistration
	err           error
}

// NewCronRecorder constructs a cron recorder with an optional failure error.
func NewCronRecorder() *CronRecorder {
	return &CronRecorder{
		Registrations: make([]CronRegistration, 0),
	}
}

// Fail configures the recorder to return the supplied error on registration.
func (c *CronRecorder) Fail(err error) {
	c.err = err
}

// Registrar returns a di.CronRegistrar that records invocations.
func (c *CronRecorder) Registrar() di.CronRegistrar {
	return func(cfg command.HandlerConfig, handler any) error {
		if c.err != nil {
			return c.err
		}
		c.Registrations = append(c.Registrations, CronRegistration{
			Config:  cfg,
			Handler: handler,
		})
		return nil
	}
}

// RecordingDispatcher captures handlers registered with a dispatcher.
type RecordingDispatcher struct {
	Handlers      []any
	Subscriptions []*RecordingSubscription
	Err           error
}

// NewRecordingDispatcher constructs a dispatcher recorder.
func NewRecordingDispatcher() *RecordingDispatcher {
	return &RecordingDispatcher{
		Handlers:      make([]any, 0),
		Subscriptions: make([]*RecordingSubscription, 0),
	}
}

// RegisterCommand satisfies di.CommandDispatcher while recording the handler.
func (d *RecordingDispatcher) RegisterCommand(handler any) (di.CommandSubscription, error) {
	if d.Err != nil {
		return nil, d.Err
	}
	d.Handlers = append(d.Handlers, handler)
	sub := &RecordingSubscription{Handler: handler}
	d.Subscriptions = append(d.Subscriptions, sub)
	return sub, nil
}

// RecordingSubscription tracks unsubscribe calls.
type RecordingSubscription struct {
	Handler      any
	Unsubscribed bool
}

// Unsubscribe marks the subscription as released.
func (s *RecordingSubscription) Unsubscribe() {
	s.Unsubscribed = true
}
