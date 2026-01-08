package domain

import internaldomain "github.com/goliatone/go-cms/internal/domain"

// Status represents lifecycle states for CMS entities.
type Status = internaldomain.Status

const (
	// StatusDraft indicates content still under preparation.
	StatusDraft = internaldomain.StatusDraft
	// StatusPublished identifies content available to consumers.
	StatusPublished = internaldomain.StatusPublished
	// StatusArchived marks content that is retained for history but not publicly visible.
	StatusArchived = internaldomain.StatusArchived
	// StatusScheduled marks content that has a future publish time configured.
	StatusScheduled = internaldomain.StatusScheduled
)
