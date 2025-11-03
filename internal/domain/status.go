package domain

import "strings"

// WorkflowState represents high-level lifecycle states understood by workflow engines.
type WorkflowState string

const (
	WorkflowStateDraft     WorkflowState = WorkflowState(StatusDraft)
	WorkflowStatePublished WorkflowState = WorkflowState(StatusPublished)
	WorkflowStateArchived  WorkflowState = WorkflowState(StatusArchived)
	WorkflowStateScheduled WorkflowState = WorkflowState(StatusScheduled)

	WorkflowStateReview     WorkflowState = "review"
	WorkflowStateApproved   WorkflowState = "approved"
	WorkflowStateRejected   WorkflowState = "rejected"
	WorkflowStateTranslated WorkflowState = "translated"
)

// WorkflowStateFromStatus maps a legacy Status into a workflow state.
func WorkflowStateFromStatus(status Status) WorkflowState {
	switch status {
	case StatusDraft:
		return WorkflowStateDraft
	case StatusPublished:
		return WorkflowStatePublished
	case StatusArchived:
		return WorkflowStateArchived
	case StatusScheduled:
		return WorkflowStateScheduled
	default:
		return WorkflowState(strings.TrimSpace(string(status)))
	}
}

// StatusFromWorkflowState maps a workflow state back to the persisted Status value.
func StatusFromWorkflowState(state WorkflowState) Status {
	switch state {
	case WorkflowStateDraft:
		return StatusDraft
	case WorkflowStatePublished:
		return StatusPublished
	case WorkflowStateArchived:
		return StatusArchived
	case WorkflowStateScheduled:
		return StatusScheduled
	default:
		return Status(strings.TrimSpace(string(state)))
	}
}

// NormalizeWorkflowState coerces arbitrary state strings into a known representation.
func NormalizeWorkflowState(input string) WorkflowState {
	if strings.TrimSpace(input) == "" {
		return WorkflowStateDraft
	}
	state := WorkflowState(strings.ToLower(strings.TrimSpace(input)))
	switch state {
	case WorkflowStateDraft,
		WorkflowStatePublished,
		WorkflowStateArchived,
		WorkflowStateScheduled,
		WorkflowStateReview,
		WorkflowStateApproved,
		WorkflowStateRejected,
		WorkflowStateTranslated:
		return state
	default:
		return state
	}
}
