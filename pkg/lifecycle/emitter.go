package lifecycle

import "context"

// Config controls lifecycle emission defaults supplied by DI/config.
type Config struct {
	Enabled bool
}

// Emitter fans out lifecycle events to hooks.
type Emitter struct {
	hooks   Hooks
	enabled bool
}

// NewEmitter constructs an emitter from hooks and configuration.
func NewEmitter(hooks Hooks, cfg Config) *Emitter {
	return &Emitter{
		hooks:   cloneHooks(hooks),
		enabled: cfg.Enabled && len(hooks) > 0,
	}
}

// Enabled reports whether emissions should be attempted.
func (e *Emitter) Enabled() bool {
	return e != nil && e.enabled && len(e.hooks) > 0
}

// Emit forwards the event to all hooks.
func (e *Emitter) Emit(ctx context.Context, event Event) error {
	if !e.Enabled() {
		return nil
	}
	return e.hooks.Notify(ctx, event)
}

func cloneHooks(hooks Hooks) Hooks {
	if len(hooks) == 0 {
		return nil
	}
	normalized := make([]Hook, 0, len(hooks))
	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		normalized = append(normalized, hook)
	}
	return Hooks(normalized)
}
