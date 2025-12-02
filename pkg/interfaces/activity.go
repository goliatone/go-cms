package interfaces

import (
	"context"

	usertypes "github.com/goliatone/go-users/pkg/types"
)

// ActivityRecord mirrors the go-users activity record contract so downstream
// packages can depend on a single type.
type ActivityRecord = usertypes.ActivityRecord

// ActivitySink captures activity events; implementations are expected to satisfy
// the go-users ActivitySink contract.
type ActivitySink interface {
	Log(ctx context.Context, record ActivityRecord) error
}
