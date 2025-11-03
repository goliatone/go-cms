package workflow

import (
	"time"

	"github.com/goliatone/go-cms/internal/domain"
	"github.com/google/uuid"
)

const (
	// EntityTypePage identifies page entities for workflow transitions.
	EntityTypePage = "page"
)

// PageContext captures metadata about a page used during workflow transitions.
type PageContext struct {
	ID               uuid.UUID
	ContentID        uuid.UUID
	TemplateID       uuid.UUID
	ParentID         *uuid.UUID
	Slug             string
	Status           domain.Status
	WorkflowState    domain.WorkflowState
	CurrentVersion   int
	PublishedVersion *int
	PublishAt        *time.Time
	UnpublishAt      *time.Time
	CreatedBy        uuid.UUID
	UpdatedBy        uuid.UUID
}

// Metadata renders the context as a map suitable for workflow metadata payloads.
func (c PageContext) Metadata() map[string]any {
	payload := map[string]any{
		"page_id":         c.ID.String(),
		"content_id":      c.ContentID.String(),
		"template_id":     c.TemplateID.String(),
		"slug":            c.Slug,
		"status":          string(c.Status),
		"workflow_state":  string(c.WorkflowState),
		"current_version": c.CurrentVersion,
		"published_version": func() any {
			if c.PublishedVersion == nil {
				return nil
			}
			return *c.PublishedVersion
		}(),
	}

	if c.ParentID != nil {
		payload["parent_id"] = c.ParentID.String()
	}
	if c.PublishAt != nil {
		payload["publish_at"] = c.PublishAt.UTC().Format(time.RFC3339)
	}
	if c.UnpublishAt != nil {
		payload["unpublish_at"] = c.UnpublishAt.UTC().Format(time.RFC3339)
	}
	if c.CreatedBy != uuid.Nil {
		payload["created_by"] = c.CreatedBy.String()
	}
	if c.UpdatedBy != uuid.Nil {
		payload["updated_by"] = c.UpdatedBy.String()
	}

	return payload
}
