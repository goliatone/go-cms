package commands

import (
	"context"
	"time"

	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
)

// TelemetryStatus captures the result category for command execution.
type TelemetryStatus string

const (
	// TelemetryStatusSuccess indicates the command completed without errors.
	TelemetryStatusSuccess TelemetryStatus = "success"
	// TelemetryStatusFailed indicates the command execution returned an error.
	TelemetryStatusFailed TelemetryStatus = "failed"
	// TelemetryStatusContextError indicates execution failed due to context cancellation or deadline.
	TelemetryStatusContextError TelemetryStatus = "context_error"
)

// TelemetryInfo describes a command execution outcome provided to telemetry callbacks.
type TelemetryInfo struct {
	Command   string
	Operation string
	Fields    map[string]any
	Duration  time.Duration
	Error     error
	Status    TelemetryStatus
	Logger    interfaces.Logger
}

// Telemetry represents an optional callback invoked after command execution.
type Telemetry[T command.Message] func(ctx context.Context, msg T, info TelemetryInfo)

// DefaultTelemetry returns a telemetry callback that logs command outcomes with the supplied logger.
func DefaultTelemetry[T command.Message](logger interfaces.Logger) Telemetry[T] {
	if logger == nil {
		logger = logging.NoOp()
	}
	return func(ctx context.Context, _ T, info TelemetryInfo) {
		entry := logger
		if info.Fields != nil {
			entry = entry.WithFields(info.Fields)
		}
		args := []any{"duration_ms", info.Duration.Milliseconds()}
		switch info.Status {
		case TelemetryStatusSuccess:
			entry.Info("command.execute.success", args...)
		case TelemetryStatusContextError:
			entry.Error("command.execute.context_error", append(args, "error", info.Error)...)
		default:
			entry.Error("command.execute.failed", append(args, "error", info.Error)...)
		}
	}
}
