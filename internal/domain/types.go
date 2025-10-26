package domain

// Status represents lifecycle states for CMS entities
type Status string

const (
	// StatusDraft indicates content still under preparation
	StatusDraft Status = "draft"
	// StatusPublished identifies content available to consumers
	StatusPublished Status = "published"
	// StatusArchived marks content that is retained for history but not publicly visible
	StatusArchived Status = "archived"
	// StatusScheduled marks content that has a future publish time configured
	StatusScheduled Status = "scheduled"
)
