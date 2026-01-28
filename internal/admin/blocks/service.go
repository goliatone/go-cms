package blocksadmin

import (
	"context"
	"errors"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
)

// ErrReporterRequired indicates the service was constructed without a reporter.
var ErrReporterRequired = errors.New("adminblocks: reporter is required")

// Service exposes embedded block conflict reporting and backfill helpers.
type Service struct {
	reporter *blocks.EmbeddedBlockBridge
}

// NewService constructs a blocks admin service.
func NewService(reporter *blocks.EmbeddedBlockBridge) *Service {
	return &Service{reporter: reporter}
}

// ListConflicts returns embedded-vs-legacy conflict reports.
func (s *Service) ListConflicts(ctx context.Context, opts blocks.ConflictReportOptions) ([]content.EmbeddedBlockConflict, error) {
	if s == nil || s.reporter == nil {
		return nil, ErrReporterRequired
	}
	return s.reporter.ListConflicts(ctx, opts)
}

// BackfillEmbeddedBlocks migrates legacy block instances into embedded payloads.
func (s *Service) BackfillEmbeddedBlocks(ctx context.Context, opts blocks.BackfillOptions) (blocks.BackfillReport, error) {
	if s == nil || s.reporter == nil {
		return blocks.BackfillReport{}, ErrReporterRequired
	}
	return s.reporter.BackfillFromLegacy(ctx, opts)
}
