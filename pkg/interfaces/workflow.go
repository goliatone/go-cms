package interfaces

import (
	"context"
	"strings"

	"github.com/goliatone/go-command/flow"
)

const DefaultWorkflowMessageType = "go-cms.workflow"

// WorkflowMessage is the canonical workflow payload consumed by FSM envelopes.
type WorkflowMessage struct {
	TypeName string         `json:"type,omitempty"`
	Payload  map[string]any `json:"payload,omitempty"`
}

// Type satisfies go-command's command.Message contract.
func (m WorkflowMessage) Type() string {
	if value := strings.TrimSpace(m.TypeName); value != "" {
		return value
	}
	return DefaultWorkflowMessageType
}

type (
	WorkflowState = string

	ExecutionContext   = flow.ExecutionContext
	ApplyEventRequest  = flow.ApplyEventRequest[WorkflowMessage]
	ApplyEventResponse = flow.ApplyEventResponse[WorkflowMessage]
	SnapshotRequest    = flow.SnapshotRequest[WorkflowMessage]
	Snapshot           = flow.Snapshot
	TransitionInfo     = flow.TransitionInfo
	GuardRejection     = flow.GuardRejection
	TargetInfo         = flow.TargetInfo
	TransitionResult   = flow.TransitionResult[WorkflowMessage]

	Effect        = flow.Effect
	CommandEffect = flow.CommandEffect
	EmitEvent     = flow.EmitEvent

	MachineDefinition            = flow.MachineDefinition
	StateDefinition              = flow.StateDefinition
	TransitionDefinition         = flow.TransitionDefinition
	WorkflowDefinition           = flow.MachineDefinition
	WorkflowStateDefinition      = flow.StateDefinition
	WorkflowTransition           = flow.TransitionDefinition
	DynamicTargetDefinition      = flow.DynamicTargetDefinition
	GuardDefinition              = flow.GuardDefinition
	TransitionWorkflowDefinition = flow.TransitionWorkflowDefinition
	WorkflowNodeDefinition       = flow.WorkflowNodeDefinition
	StepDefinition               = flow.StepDefinition

	Guard                 = flow.Guard[WorkflowMessage]
	DynamicTargetResolver = flow.DynamicTargetResolver[WorkflowMessage]
	Action                = func(context.Context, WorkflowMessage) error
)

// WorkflowEngine coordinates lifecycle transitions using canonical FSM envelopes.
type WorkflowEngine interface {
	ApplyEvent(ctx context.Context, req ApplyEventRequest) (*ApplyEventResponse, error)
	Snapshot(ctx context.Context, req SnapshotRequest) (*Snapshot, error)
	RegisterMachine(ctx context.Context, definition MachineDefinition) error
	RegisterGuard(name string, guard Guard) error
	RegisterDynamicTarget(name string, resolver DynamicTargetResolver) error
	RegisterAction(name string, action Action) error
}

// WorkflowDefinitionStore exposes workflow definitions sourced from external storage.
type WorkflowDefinitionStore interface {
	ListWorkflowDefinitions(ctx context.Context) ([]MachineDefinition, error)
}
