package promotions

import (
	"context"

	"github.com/google/uuid"
)

// Service defines promotion orchestration across environments.
type Service interface {
	PromoteEnvironment(ctx context.Context, req PromoteEnvironmentRequest) (*PromoteEnvironmentResult, error)
	PromoteContentType(ctx context.Context, req PromoteContentTypeRequest) (*PromoteItem, error)
	PromoteContentEntry(ctx context.Context, req PromoteContentEntryRequest) (*PromoteItem, error)
}

// PromoteScope describes the scope for bulk promotions.
type PromoteScope string

const (
	ScopeContentTypes   PromoteScope = "content_types"
	ScopeContentEntries PromoteScope = "content_entries"
	ScopeAll            PromoteScope = "all"
)

// PromoteMode controls how conflicts are handled.
type PromoteMode string

const (
	ModeStrict PromoteMode = "strict"
	ModeUpsert PromoteMode = "upsert"
)

// PromoteOptions captures common promotion flags.
type PromoteOptions struct {
	AllowBreakingChanges bool        `json:"allow_breaking_changes,omitempty"`
	AllowDraft           bool        `json:"allow_draft,omitempty"`
	PromoteAsActive      bool        `json:"promote_as_active,omitempty"`
	PreferPublished      *bool       `json:"prefer_published,omitempty"`
	PromoteAsPublished   bool        `json:"promote_as_published,omitempty"`
	Mode                 PromoteMode `json:"mode,omitempty"`
	IncludeVersions      bool        `json:"include_versions,omitempty"`
	MigrateOnPromote     *bool       `json:"migrate_on_promote,omitempty"`
	AutoPromoteType      bool        `json:"auto_promote_type,omitempty"`
	Force                bool        `json:"force,omitempty"`
	DryRun               bool        `json:"dry_run,omitempty"`
}

// PromoteEnvironmentRequest describes a bulk environment promotion request.
type PromoteEnvironmentRequest struct {
	SourceEnvironment string `json:"-"`
	TargetEnvironment string `json:"-"`

	Scope            PromoteScope   `json:"scope,omitempty"`
	ContentTypeIDs   []uuid.UUID    `json:"content_type_ids,omitempty"`
	ContentTypeSlugs []string       `json:"content_type_slugs,omitempty"`
	ContentIDs       []uuid.UUID    `json:"content_ids,omitempty"`
	ContentSlugs     []string       `json:"content_slugs,omitempty"`
	Options          PromoteOptions `json:"options,omitempty"`
}

// PromoteContentTypeRequest describes a single content type promotion.
type PromoteContentTypeRequest struct {
	ContentTypeID       uuid.UUID      `json:"-"`
	TargetEnvironment   string         `json:"target_environment,omitempty"`
	TargetEnvironmentID *uuid.UUID     `json:"target_environment_id,omitempty"`
	Options             PromoteOptions `json:"options,omitempty"`
}

// PromoteContentEntryRequest describes a single content entry promotion.
type PromoteContentEntryRequest struct {
	ContentID           uuid.UUID      `json:"-"`
	TargetEnvironment   string         `json:"target_environment,omitempty"`
	TargetEnvironmentID *uuid.UUID     `json:"target_environment_id,omitempty"`
	Options             PromoteOptions `json:"options,omitempty"`
}

// EnvironmentRef references an environment in promotion responses.
type EnvironmentRef struct {
	ID  uuid.UUID `json:"id"`
	Key string    `json:"key"`
}

// PromoteSummaryCounts reports bulk promotion counts per entity type.
type PromoteSummaryCounts struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

// PromoteSummary aggregates counts for bulk promotions.
type PromoteSummary struct {
	ContentTypes   PromoteSummaryCounts `json:"content_types"`
	ContentEntries PromoteSummaryCounts `json:"content_entries"`
}

// PromoteItem reports a single promoted entity.
type PromoteItem struct {
	Kind     string         `json:"kind"`
	SourceID uuid.UUID      `json:"source_id"`
	TargetID uuid.UUID      `json:"target_id"`
	Status   string         `json:"status"`
	Message  string         `json:"message,omitempty"`
	Details  map[string]any `json:"details,omitempty"`
}

// PromoteError describes a promotion failure.
type PromoteError struct {
	Kind     string         `json:"kind"`
	SourceID uuid.UUID      `json:"source_id"`
	Error    string         `json:"error"`
	Details  map[string]any `json:"details,omitempty"`
}

// PromoteEnvironmentResult captures the bulk promotion response.
type PromoteEnvironmentResult struct {
	SourceEnv EnvironmentRef `json:"source_env"`
	TargetEnv EnvironmentRef `json:"target_env"`
	Summary   PromoteSummary `json:"summary"`
	Items     []PromoteItem  `json:"items,omitempty"`
	Errors    []PromoteError `json:"errors,omitempty"`
}
