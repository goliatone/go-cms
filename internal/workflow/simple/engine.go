package simple

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

const (
	entityTypePage = "page"
)

var (
	// ErrUnknownEntityType indicates no workflow definition exists for the requested entity.
	ErrUnknownEntityType = errors.New("workflow: entity type not registered")
	// ErrInvalidTransition indicates the requested transition is not allowed.
	ErrInvalidTransition = errors.New("workflow: transition not allowed")
	// ErrMissingTransition indicates neither a transition name nor target state were supplied.
	ErrMissingTransition = errors.New("workflow: transition name or target state required")
	// ErrNilEntityID signals input validation failure.
	ErrNilEntityID = errors.New("workflow: entity id required")
	// ErrDefinitionStatesRequired indicates a definition is missing states.
	ErrDefinitionStatesRequired = errors.New("workflow: definition requires at least one state")
	// ErrStateNameRequired indicates a workflow state is missing its name.
	ErrStateNameRequired = errors.New("workflow: state name required")
	// ErrDuplicateState indicates duplicate workflow state names were declared.
	ErrDuplicateState = errors.New("workflow: duplicate state")
	// ErrTransitionStateUnknown indicates a transition references an unknown state.
	ErrTransitionStateUnknown = errors.New("workflow: transition references unknown state")
	// ErrDuplicateTransition indicates the same transition name is declared multiple times for a state.
	ErrDuplicateTransition = errors.New("workflow: duplicate transition for state")
	// ErrTerminalState indicates a transition was attempted from a terminal state.
	ErrTerminalState = errors.New("workflow: terminal state reached")
	// ErrGuardAuthorizerRequired indicates a guard was present but no authorizer was configured.
	ErrGuardAuthorizerRequired = errors.New("workflow: guard authorizer required")
)

// Engine is a simple in-memory workflow engine that executes deterministic state transitions.
type Engine struct {
	mu          sync.RWMutex
	definitions map[string]*workflowDefinition
	now         func() time.Time

	authorizer interfaces.WorkflowAuthorizer
}

// Option configures the engine.
type Option func(*Engine)

// WithClock overrides the clock used for transition timestamps (primarily for testing).
func WithClock(clock func() time.Time) Option {
	return func(e *Engine) {
		if clock != nil {
			e.now = clock
		}
	}
}

// WithAuthorizer wires a guard authorizer to enforce guarded transitions.
func WithAuthorizer(authorizer interfaces.WorkflowAuthorizer) Option {
	return func(e *Engine) {
		e.authorizer = authorizer
	}
}

// New constructs a workflow engine seeded with the default page workflow.
func New(opts ...Option) *Engine {
	engine := &Engine{
		definitions: make(map[string]*workflowDefinition),
		now:         time.Now,
	}
	for _, opt := range opts {
		opt(engine)
	}

	_ = engine.RegisterWorkflow(context.Background(), defaultPageWorkflowDefinition())

	return engine
}

// Transition applies a workflow transition for an entity.
func (e *Engine) Transition(ctx context.Context, input interfaces.TransitionInput) (*interfaces.TransitionResult, error) {
	if input.EntityID == uuid.Nil {
		return nil, ErrNilEntityID
	}

	definition, err := e.definitionFor(input.EntityType)
	if err != nil {
		return nil, err
	}

	current := toWorkflowState(input.CurrentState, definition.definition.InitialState)
	transitionName := strings.TrimSpace(strings.ToLower(input.Transition))
	var targetState interfaces.WorkflowState
	if strings.TrimSpace(string(input.TargetState)) != "" {
		targetState = toWorkflowState(input.TargetState, "")
	}

	if transitionName == "" && targetState == "" {
		targetState = current
	}

	if transitionName == "" && targetState == current {
		return &interfaces.TransitionResult{
			EntityID:    input.EntityID,
			EntityType:  input.EntityType,
			Transition:  "",
			FromState:   current,
			ToState:     current,
			CompletedAt: e.now(),
			ActorID:     input.ActorID,
			Metadata:    cloneMetadata(input.Metadata),
		}, nil
	}

	if definition.isTerminal(current) {
		return nil, fmt.Errorf("%w: %s", ErrTerminalState, current)
	}

	var transition interfaces.WorkflowTransition
	switch {
	case transitionName != "":
		transition, err = definition.lookupTransition(transitionName, current)
		if err != nil {
			return nil, err
		}
	case targetState != "":
		transition, err = definition.lookupByStates(current, targetState)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrMissingTransition
	}

	if guard := strings.TrimSpace(transition.Guard); guard != "" {
		if e.authorizer == nil {
			return nil, fmt.Errorf("%w: %s", ErrGuardAuthorizerRequired, guard)
		}
		if err := e.authorizer.AuthorizeTransition(ctx, input, guard); err != nil {
			return nil, err
		}
	}

	result := &interfaces.TransitionResult{
		EntityID:    input.EntityID,
		EntityType:  input.EntityType,
		Transition:  transition.Name,
		FromState:   current,
		ToState:     normalizeWorkflowState(transition.To),
		CompletedAt: e.now(),
		ActorID:     input.ActorID,
		Metadata:    cloneMetadata(input.Metadata),
	}

	return result, nil
}

// AvailableTransitions returns the transitions reachable from the supplied state.
func (e *Engine) AvailableTransitions(ctx context.Context, query interfaces.TransitionQuery) ([]WorkflowTransition, error) {
	definition, err := e.definitionFor(query.EntityType)
	if err != nil {
		return nil, err
	}
	state := toWorkflowState(query.State, definition.definition.InitialState)
	transitions := definition.transitionsByState[state]
	result := make([]WorkflowTransition, len(transitions))
	copy(result, transitions)
	return result, nil
}

// WorkflowTransition mirrors interfaces.WorkflowTransition while keeping the
// package self-contained for consumers of AvailableTransitions.
type WorkflowTransition = interfaces.WorkflowTransition

// WorkflowDefinition mirrors interfaces.WorkflowDefinition for return paths.
type WorkflowDefinition = interfaces.WorkflowDefinition

// RegisterWorkflow installs a workflow definition for the supplied entity type.
func (e *Engine) RegisterWorkflow(ctx context.Context, definition interfaces.WorkflowDefinition) error {
	if strings.TrimSpace(definition.EntityType) == "" {
		return fmt.Errorf("workflow: entity type required")
	}
	normalized, err := compileDefinition(definition)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.definitions[normalized.definition.EntityType] = normalized
	return nil
}

func (e *Engine) definitionFor(entityType string) (*workflowDefinition, error) {
	entityKey := strings.ToLower(strings.TrimSpace(entityType))
	e.mu.RLock()
	definition, ok := e.definitions[entityKey]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownEntityType, entityType)
	}
	return definition, nil
}

type workflowDefinition struct {
	definition         interfaces.WorkflowDefinition
	transitions        map[string]interfaces.WorkflowTransition
	transitionsByState map[interfaces.WorkflowState][]interfaces.WorkflowTransition
	terminalStates     map[interfaces.WorkflowState]struct{}
}

func compileDefinition(definition interfaces.WorkflowDefinition) (*workflowDefinition, error) {
	if len(definition.States) == 0 {
		return nil, ErrDefinitionStatesRequired
	}

	normalizedStates := make([]interfaces.WorkflowStateDefinition, 0, len(definition.States))
	stateMap := make(map[interfaces.WorkflowState]struct{}, len(definition.States))
	terminalStates := make(map[interfaces.WorkflowState]struct{}, len(definition.States))

	for idx, state := range definition.States {
		name := normalizeWorkflowState(state.Name)
		if strings.TrimSpace(string(name)) == "" {
			return nil, fmt.Errorf("%w at index %d", ErrStateNameRequired, idx)
		}
		if _, exists := stateMap[name]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateState, name)
		}
		stateMap[name] = struct{}{}
		if state.Terminal {
			terminalStates[name] = struct{}{}
		}
		normalizedStates = append(normalizedStates, interfaces.WorkflowStateDefinition{
			Name:        name,
			Description: strings.TrimSpace(state.Description),
			Terminal:    state.Terminal,
		})
	}

	initial := normalizeWorkflowState(definition.InitialState)
	if strings.TrimSpace(string(initial)) == "" {
		initial = normalizedStates[0].Name
	} else if _, ok := stateMap[initial]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrTransitionStateUnknown, initial)
	}

	compiled := &workflowDefinition{
		definition: interfaces.WorkflowDefinition{
			EntityType:   strings.ToLower(strings.TrimSpace(definition.EntityType)),
			InitialState: initial,
			States:       normalizedStates,
		},
		transitions:        make(map[string]interfaces.WorkflowTransition),
		transitionsByState: make(map[interfaces.WorkflowState][]interfaces.WorkflowTransition),
		terminalStates:     terminalStates,
	}

	for _, transition := range definition.Transitions {
		from := normalizeWorkflowState(transition.From)
		to := normalizeWorkflowState(transition.To)
		if strings.TrimSpace(string(from)) == "" || strings.TrimSpace(string(to)) == "" {
			return nil, ErrTransitionStateUnknown
		}
		if _, ok := stateMap[from]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrTransitionStateUnknown, from)
		}
		if _, ok := stateMap[to]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrTransitionStateUnknown, to)
		}
		if _, isTerminal := terminalStates[from]; isTerminal && from != to {
			return nil, fmt.Errorf("%w: %s", ErrTerminalState, from)
		}
		transition.From = from
		transition.To = to
		key := transitionKey(transition.Name, from)
		if _, exists := compiled.transitions[key]; exists {
			return nil, fmt.Errorf("%w: %s from %s", ErrDuplicateTransition, transition.Name, from)
		}
		compiled.transitions[key] = transition
		compiled.transitionsByState[from] = append(compiled.transitionsByState[from], transition)
		compiled.definition.Transitions = append(compiled.definition.Transitions, transition)
	}

	return compiled, nil
}

func (d *workflowDefinition) lookupTransition(name string, state interfaces.WorkflowState) (interfaces.WorkflowTransition, error) {
	key := transitionKey(name, normalizeWorkflowState(state))
	transition, ok := d.transitions[key]
	if !ok {
		return interfaces.WorkflowTransition{}, fmt.Errorf("%w: %s from %s", ErrInvalidTransition, name, state)
	}
	return transition, nil
}

func (d *workflowDefinition) lookupByStates(from, to interfaces.WorkflowState) (interfaces.WorkflowTransition, error) {
	transitions := d.transitionsByState[normalizeWorkflowState(from)]
	target := normalizeWorkflowState(to)
	for _, candidate := range transitions {
		if normalizeWorkflowState(candidate.To) == target {
			return candidate, nil
		}
	}
	return interfaces.WorkflowTransition{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
}

func transitionKey(name string, from interfaces.WorkflowState) string {
	return strings.TrimSpace(strings.ToLower(name)) + "::" + string(normalizeWorkflowState(from))
}

func (d *workflowDefinition) isTerminal(state interfaces.WorkflowState) bool {
	_, ok := d.terminalStates[normalizeWorkflowState(state)]
	return ok
}

func toWorkflowState(state interfaces.WorkflowState, fallback interfaces.WorkflowState) interfaces.WorkflowState {
	if strings.TrimSpace(string(state)) == "" {
		return normalizeWorkflowState(fallback)
	}
	return normalizeWorkflowState(state)
}

func normalizeWorkflowState(state interfaces.WorkflowState) interfaces.WorkflowState {
	if strings.TrimSpace(string(state)) == "" {
		return interfaces.WorkflowState(domain.WorkflowStateDraft)
	}
	return interfaces.WorkflowState(domain.NormalizeWorkflowState(string(state)))
}

func cloneMetadata(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	clone := make(map[string]any, len(input))
	for k, v := range input {
		clone[k] = v
	}
	return clone
}

func defaultPageWorkflowDefinition() interfaces.WorkflowDefinition {
	return interfaces.WorkflowDefinition{
		EntityType:   entityTypePage,
		InitialState: interfaces.WorkflowState(domain.WorkflowStateDraft),
		States: []interfaces.WorkflowStateDefinition{
			{Name: interfaces.WorkflowState(domain.WorkflowStateDraft), Description: "Draft content awaiting validation"},
			{Name: interfaces.WorkflowState(domain.WorkflowStateReview), Description: "Under editorial review"},
			{Name: interfaces.WorkflowState(domain.WorkflowStateApproved), Description: "Approved and ready to publish"},
			{Name: interfaces.WorkflowState(domain.WorkflowStateScheduled), Description: "Scheduled to publish at a future time"},
			{Name: interfaces.WorkflowState(domain.WorkflowStatePublished), Description: "Published and visible"},
			{Name: interfaces.WorkflowState(domain.WorkflowStateArchived), Description: "Archived and hidden"},
		},
		Transitions: []interfaces.WorkflowTransition{
			{Name: "submit_review", From: interfaces.WorkflowState(domain.WorkflowStateDraft), To: interfaces.WorkflowState(domain.WorkflowStateReview)},
			{Name: "approve", From: interfaces.WorkflowState(domain.WorkflowStateReview), To: interfaces.WorkflowState(domain.WorkflowStateApproved)},
			{Name: "reject", From: interfaces.WorkflowState(domain.WorkflowStateReview), To: interfaces.WorkflowState(domain.WorkflowStateDraft)},
			{Name: "request_changes", From: interfaces.WorkflowState(domain.WorkflowStateApproved), To: interfaces.WorkflowState(domain.WorkflowStateReview)},
			{Name: "publish", From: interfaces.WorkflowState(domain.WorkflowStateApproved), To: interfaces.WorkflowState(domain.WorkflowStatePublished)},
			{Name: "publish", From: interfaces.WorkflowState(domain.WorkflowStateDraft), To: interfaces.WorkflowState(domain.WorkflowStatePublished)},
			{Name: "publish", From: interfaces.WorkflowState(domain.WorkflowStateScheduled), To: interfaces.WorkflowState(domain.WorkflowStatePublished)},
			{Name: "unpublish", From: interfaces.WorkflowState(domain.WorkflowStatePublished), To: interfaces.WorkflowState(domain.WorkflowStateDraft)},
			{Name: "archive", From: interfaces.WorkflowState(domain.WorkflowStateDraft), To: interfaces.WorkflowState(domain.WorkflowStateArchived)},
			{Name: "archive", From: interfaces.WorkflowState(domain.WorkflowStatePublished), To: interfaces.WorkflowState(domain.WorkflowStateArchived)},
			{Name: "restore", From: interfaces.WorkflowState(domain.WorkflowStateArchived), To: interfaces.WorkflowState(domain.WorkflowStateDraft)},
			{Name: "schedule", From: interfaces.WorkflowState(domain.WorkflowStateDraft), To: interfaces.WorkflowState(domain.WorkflowStateScheduled)},
			{Name: "schedule", From: interfaces.WorkflowState(domain.WorkflowStateApproved), To: interfaces.WorkflowState(domain.WorkflowStateScheduled)},
			{Name: "cancel_schedule", From: interfaces.WorkflowState(domain.WorkflowStateScheduled), To: interfaces.WorkflowState(domain.WorkflowStateDraft)},
		},
	}
}
