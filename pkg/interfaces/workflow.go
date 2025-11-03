package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// WorkflowState represents a lifecycle stage understood by workflow engines.
type WorkflowState string

// WorkflowEngine coordinates lifecycle transitions for domain entities.
type WorkflowEngine interface {
	// Transition applies the named transition (or explicit state change) to the entity.
	Transition(ctx context.Context, input TransitionInput) (*TransitionResult, error)
	// AvailableTransitions lists the possible transitions from the supplied state.
	AvailableTransitions(ctx context.Context, query TransitionQuery) ([]WorkflowTransition, error)
	// RegisterWorkflow installs or replaces a workflow definition for the given entity type.
	RegisterWorkflow(ctx context.Context, definition WorkflowDefinition) error
}

// WorkflowDefinitionStore exposes workflow definitions sourced from external storage.
type WorkflowDefinitionStore interface {
	ListWorkflowDefinitions(ctx context.Context) ([]WorkflowDefinition, error)
}

// WorkflowAuthorizer evaluates guard expressions attached to workflow transitions.
// Returning nil authorises the transition; returning an error prevents it.
type WorkflowAuthorizer interface {
	AuthorizeTransition(ctx context.Context, input TransitionInput, guard string) error
}

// TransitionInput captures the data required to run a workflow transition.
type TransitionInput struct {
	EntityID     uuid.UUID
	EntityType   string
	CurrentState WorkflowState
	Transition   string
	TargetState  WorkflowState
	ActorID      uuid.UUID
	Metadata     map[string]any
}

// TransitionResult describes the outcome of a workflow transition.
type TransitionResult struct {
	EntityID      uuid.UUID
	EntityType    string
	Transition    string
	FromState     WorkflowState
	ToState       WorkflowState
	CompletedAt   time.Time
	ActorID       uuid.UUID
	Metadata      map[string]any
	Events        []WorkflowEvent
	Notifications []WorkflowNotification
}

// WorkflowEvent represents an emitted domain event during a transition.
type WorkflowEvent struct {
	Name      string
	Timestamp time.Time
	Payload   map[string]any
}

// WorkflowNotification represents a downstream notification request driven by a transition.
type WorkflowNotification struct {
	Channel string
	Message string
	Data    map[string]any
}

// TransitionQuery describes the state for which transitions should be listed.
type TransitionQuery struct {
	EntityType string
	State      WorkflowState
	Context    map[string]any
}

// WorkflowDefinition describes a state machine for a specific entity type.
type WorkflowDefinition struct {
	EntityType   string
	InitialState WorkflowState
	States       []WorkflowStateDefinition
	Transitions  []WorkflowTransition
}

// WorkflowStateDefinition documents a workflow state.
type WorkflowStateDefinition struct {
	Name        WorkflowState
	Description string
	Terminal    bool
}

// WorkflowTransition declares an allowed transition between two states.
type WorkflowTransition struct {
	Name        string
	Description string
	From        WorkflowState
	To          WorkflowState
	Guard       string
}
