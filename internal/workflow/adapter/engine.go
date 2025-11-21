package adapter

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

var (
	// ErrWorkflowEngineRequired indicates the adapter was constructed without a backing engine.
	ErrWorkflowEngineRequired = errors.New("workflow adapter: engine required")
	// ErrUnknownEntityType indicates no workflow definition exists for the requested entity.
	ErrUnknownEntityType = errors.New("workflow: entity type not registered")
	// ErrInvalidTransition indicates the requested transition is not allowed.
	ErrInvalidTransition = errors.New("workflow: transition not allowed")
	// ErrMissingTransition indicates neither a transition name nor target state were supplied.
	ErrMissingTransition = errors.New("workflow: transition name or target state required")
	// ErrNilEntityID signals input validation failure.
	ErrNilEntityID = errors.New("workflow: entity id required")
	// ErrGuardAuthorizerRequired indicates a guard was present but no authorizer was configured.
	ErrGuardAuthorizerRequired = errors.New("workflow adapter: guard authorizer required")
	// ErrGuardRejected indicates the configured authorizer blocked the transition.
	ErrGuardRejected = errors.New("workflow adapter: transition blocked by guard")
)

type workflowEngine interface {
	Transition(ctx context.Context, input interfaces.TransitionInput) (*interfaces.TransitionResult, error)
	AvailableTransitions(ctx context.Context, query interfaces.TransitionQuery) ([]interfaces.WorkflowTransition, error)
	RegisterWorkflow(ctx context.Context, definition interfaces.WorkflowDefinition) error
}

// Engine adapts a workflow state machine to the go-cms interfaces.WorkflowEngine contract.
type Engine struct {
	machine workflowEngine

	authorizer     interfaces.WorkflowAuthorizer
	actionResolver ActionResolver
	now            func() time.Time

	mu          sync.RWMutex
	definitions map[string]*compiledDefinition
}

// Option configures the adapter engine.
type Option func(*Engine)

// NewEngine constructs a workflow adapter backed by the supplied engine.
func NewEngine(machine workflowEngine, opts ...Option) (*Engine, error) {
	if machine == nil {
		return nil, ErrWorkflowEngineRequired
	}

	engine := &Engine{
		machine:     machine,
		now:         time.Now,
		definitions: make(map[string]*compiledDefinition),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(engine)
		}
	}

	return engine, nil
}

// WithAuthorizer injects a guard authorizer to gate guarded transitions.
func WithAuthorizer(authorizer interfaces.WorkflowAuthorizer) Option {
	return func(e *Engine) {
		e.authorizer = authorizer
	}
}

// WithActionResolver wires a resolver used to execute transition actions.
func WithActionResolver(resolver ActionResolver) Option {
	return func(e *Engine) {
		e.actionResolver = resolver
	}
}

// WithActionRegistry configures a simple registry-based action resolver.
func WithActionRegistry(registry ActionRegistry) Option {
	return func(e *Engine) {
		if registry != nil {
			e.actionResolver = registry.Resolve
		}
	}
}

// WithClock overrides the clock used for transition timestamps (primarily for testing).
func WithClock(clock func() time.Time) Option {
	return func(e *Engine) {
		if clock != nil {
			e.now = clock
		}
	}
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

	normalizedInput := normalizeInput(definition, input)

	transitionName := normalizeTransitionName(normalizedInput.Transition)
	targetState := normalizeWorkflowState(normalizedInput.TargetState)
	current := normalizedInput.CurrentState

	if transitionName == "" && targetState == "" {
		targetState = current
	}

	if transitionName == "" && targetState == current {
		return &interfaces.TransitionResult{
			EntityID:    normalizedInput.EntityID,
			EntityType:  definition.definition.EntityType,
			Transition:  "",
			FromState:   current,
			ToState:     current,
			CompletedAt: e.now(),
			ActorID:     normalizedInput.ActorID,
			Metadata:    cloneMetadata(normalizedInput.Metadata),
		}, nil
	}

	var transition interfaces.WorkflowTransition

	switch {
	case transitionName != "":
		transition, err = definition.lookupTransition(transitionName, current)
	case targetState != "":
		transition, err = definition.lookupByStates(current, targetState)
	default:
		err = ErrMissingTransition
	}

	if err != nil {
		return nil, err
	}

	normalizedInput.Transition = transition.Name
	normalizedInput.TargetState = transition.To

	if guard := strings.TrimSpace(transition.Guard); guard != "" {
		if e.authorizer == nil {
			return nil, fmt.Errorf("%w: %s", ErrGuardAuthorizerRequired, guard)
		}
		guardInput := normalizedInput
		guardInput.Metadata = cloneMetadata(guardInput.Metadata)
		if err := e.authorizer.AuthorizeTransition(ctx, guardInput, guard); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrGuardRejected, err)
		}
	}

	result, err := e.machine.Transition(ctx, normalizedInput)
	if err != nil {
		return nil, err
	}

	normalizedResult := normalizeResult(result, transition, normalizedInput, e.now)

	if action := e.resolveAction(definition, transition); action != nil {
		output, err := action(ctx, ActionInput{
			Transition: normalizedInput,
			Result:     normalizedResult,
		})
		if err != nil {
			return nil, err
		}
		if len(output.Events) > 0 {
			normalizedResult.Events = append(normalizedResult.Events, cloneEvents(output.Events)...)
		}
		if len(output.Notifications) > 0 {
			normalizedResult.Notifications = append(normalizedResult.Notifications, cloneNotifications(output.Notifications)...)
		}
		if len(output.Metadata) > 0 {
			normalizedResult.Metadata = mergeMetadata(normalizedResult.Metadata, output.Metadata)
		}
	}

	return normalizedResult, nil
}

// AvailableTransitions returns the transitions reachable from the supplied state.
func (e *Engine) AvailableTransitions(ctx context.Context, query interfaces.TransitionQuery) ([]interfaces.WorkflowTransition, error) {
	definition, err := e.definitionFor(query.EntityType)
	if err != nil {
		return nil, err
	}

	normalizedQuery := interfaces.TransitionQuery{
		EntityType: definition.definition.EntityType,
		State:      normalizeWorkflowStateWithDefault(query.State, definition.definition.InitialState),
		Context:    cloneMetadata(query.Context),
	}

	transitions, err := e.machine.AvailableTransitions(ctx, normalizedQuery)
	if err != nil {
		return nil, err
	}

	return normalizeTransitions(transitions), nil
}

// RegisterWorkflow installs a workflow definition for the supplied entity type.
func (e *Engine) RegisterWorkflow(ctx context.Context, definition interfaces.WorkflowDefinition) error {
	compiled := compileDefinition(definition)
	if compiled.definition.EntityType == "" {
		return fmt.Errorf("%w: %s", ErrUnknownEntityType, definition.EntityType)
	}

	if err := e.machine.RegisterWorkflow(ctx, compiled.definition); err != nil {
		return err
	}

	e.mu.Lock()
	e.definitions[compiled.definition.EntityType] = compiled
	e.mu.Unlock()

	return nil
}

func (e *Engine) resolveAction(definition *compiledDefinition, transition interfaces.WorkflowTransition) Action {
	if e.actionResolver == nil {
		return nil
	}
	action, ok := e.actionResolver(definition.definition.EntityType, transition)
	if !ok {
		return nil
	}
	return action
}

func (e *Engine) definitionFor(entityType string) (*compiledDefinition, error) {
	key := normalizeEntityType(entityType)
	e.mu.RLock()
	definition, ok := e.definitions[key]
	e.mu.RUnlock()
	if !ok || definition == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownEntityType, key)
	}
	return definition, nil
}

func compileDefinition(definition interfaces.WorkflowDefinition) *compiledDefinition {
	normalized := interfaces.WorkflowDefinition{
		EntityType: normalizeEntityType(definition.EntityType),
		States:     make([]interfaces.WorkflowStateDefinition, 0, len(definition.States)),
	}

	for _, state := range definition.States {
		normalizedState := interfaces.WorkflowStateDefinition{
			Name:        normalizeWorkflowState(state.Name),
			Description: strings.TrimSpace(state.Description),
			Terminal:    state.Terminal,
		}
		normalized.States = append(normalized.States, normalizedState)
	}

	switch {
	case strings.TrimSpace(string(definition.InitialState)) != "":
		normalized.InitialState = normalizeWorkflowState(definition.InitialState)
	case len(normalized.States) > 0:
		normalized.InitialState = normalized.States[0].Name
	default:
		normalized.InitialState = interfaces.WorkflowState(domain.WorkflowStateDraft)
	}

	compiled := &compiledDefinition{
		definition:         normalized,
		transitions:        make(map[string]interfaces.WorkflowTransition),
		transitionsByState: make(map[interfaces.WorkflowState][]interfaces.WorkflowTransition),
	}

	for _, transition := range definition.Transitions {
		normalizedTransition := interfaces.WorkflowTransition{
			Name:        normalizeTransitionName(transition.Name),
			Description: strings.TrimSpace(transition.Description),
			From:        normalizeWorkflowState(transition.From),
			To:          normalizeWorkflowState(transition.To),
			Guard:       strings.TrimSpace(transition.Guard),
		}
		key := transitionKey(normalizedTransition.Name, normalizedTransition.From)
		compiled.transitions[key] = normalizedTransition
		compiled.transitionsByState[normalizedTransition.From] = append(compiled.transitionsByState[normalizedTransition.From], normalizedTransition)
		compiled.definition.Transitions = append(compiled.definition.Transitions, normalizedTransition)
	}

	return compiled
}

type compiledDefinition struct {
	definition         interfaces.WorkflowDefinition
	transitions        map[string]interfaces.WorkflowTransition
	transitionsByState map[interfaces.WorkflowState][]interfaces.WorkflowTransition
}

func (d *compiledDefinition) lookupTransition(name string, state interfaces.WorkflowState) (interfaces.WorkflowTransition, error) {
	key := transitionKey(name, normalizeWorkflowState(state))
	transition, ok := d.transitions[key]
	if !ok {
		return interfaces.WorkflowTransition{}, fmt.Errorf("%w: %s from %s", ErrInvalidTransition, name, state)
	}
	return transition, nil
}

func (d *compiledDefinition) lookupByStates(from, to interfaces.WorkflowState) (interfaces.WorkflowTransition, error) {
	transitions := d.transitionsByState[normalizeWorkflowState(from)]
	target := normalizeWorkflowState(to)
	for _, candidate := range transitions {
		if normalizeWorkflowState(candidate.To) == target {
			return candidate, nil
		}
	}
	return interfaces.WorkflowTransition{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
}

func normalizeInput(definition *compiledDefinition, input interfaces.TransitionInput) interfaces.TransitionInput {
	normalized := input
	normalized.EntityType = definition.definition.EntityType
	normalized.CurrentState = normalizeWorkflowStateWithDefault(input.CurrentState, definition.definition.InitialState)
	normalized.TargetState = normalizeWorkflowState(input.TargetState)
	normalized.Transition = normalizeTransitionName(input.Transition)
	normalized.Metadata = cloneMetadata(input.Metadata)
	return normalized
}

func normalizeWorkflowStateWithDefault(state interfaces.WorkflowState, fallback interfaces.WorkflowState) interfaces.WorkflowState {
	if strings.TrimSpace(string(state)) == "" {
		return normalizeWorkflowState(fallback)
	}
	return normalizeWorkflowState(state)
}

func normalizeWorkflowState(state interfaces.WorkflowState) interfaces.WorkflowState {
	return interfaces.WorkflowState(domain.NormalizeWorkflowState(string(state)))
}

func normalizeEntityType(entity string) string {
	return strings.ToLower(strings.TrimSpace(entity))
}

func normalizeTransitionName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func transitionKey(name string, from interfaces.WorkflowState) string {
	return normalizeTransitionName(name) + "::" + string(normalizeWorkflowState(from))
}

func normalizeResult(result *interfaces.TransitionResult, transition interfaces.WorkflowTransition, input interfaces.TransitionInput, clock func() time.Time) *interfaces.TransitionResult {
	if result == nil {
		return nil
	}

	normalized := *result
	normalized.EntityType = input.EntityType

	if normalized.Transition == "" {
		normalized.Transition = transition.Name
	}

	if normalized.FromState == "" {
		normalized.FromState = input.CurrentState
	}
	normalized.FromState = normalizeWorkflowState(normalized.FromState)

	if normalized.ToState == "" {
		normalized.ToState = transition.To
	}
	normalized.ToState = normalizeWorkflowState(normalized.ToState)

	if normalized.CompletedAt.IsZero() && clock != nil {
		normalized.CompletedAt = clock()
	}

	if normalized.Metadata == nil && len(input.Metadata) > 0 {
		normalized.Metadata = cloneMetadata(input.Metadata)
	} else {
		normalized.Metadata = cloneMetadata(normalized.Metadata)
	}

	normalized.Events = cloneEvents(normalized.Events)
	normalized.Notifications = cloneNotifications(normalized.Notifications)

	return &normalized
}

func normalizeTransitions(transitions []interfaces.WorkflowTransition) []interfaces.WorkflowTransition {
	result := make([]interfaces.WorkflowTransition, len(transitions))
	for i, transition := range transitions {
		result[i] = interfaces.WorkflowTransition{
			Name:        normalizeTransitionName(transition.Name),
			Description: strings.TrimSpace(transition.Description),
			From:        normalizeWorkflowState(transition.From),
			To:          normalizeWorkflowState(transition.To),
			Guard:       strings.TrimSpace(transition.Guard),
		}
	}
	return result
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

func mergeMetadata(base map[string]any, extra map[string]any) map[string]any {
	if len(extra) == 0 {
		return cloneMetadata(base)
	}
	result := cloneMetadata(base)
	if result == nil {
		result = make(map[string]any, len(extra))
	}
	for k, v := range extra {
		result[k] = v
	}
	return result
}

func cloneEvents(events []interfaces.WorkflowEvent) []interfaces.WorkflowEvent {
	if len(events) == 0 {
		return nil
	}
	result := make([]interfaces.WorkflowEvent, len(events))
	for i, event := range events {
		cloned := interfaces.WorkflowEvent{
			Name:      event.Name,
			Timestamp: event.Timestamp,
		}
		if len(event.Payload) > 0 {
			cloned.Payload = cloneMetadata(event.Payload)
		}
		result[i] = cloned
	}
	return result
}

func cloneNotifications(notifications []interfaces.WorkflowNotification) []interfaces.WorkflowNotification {
	if len(notifications) == 0 {
		return nil
	}
	result := make([]interfaces.WorkflowNotification, len(notifications))
	for i, notification := range notifications {
		cloned := interfaces.WorkflowNotification{
			Channel: notification.Channel,
			Message: notification.Message,
		}
		if len(notification.Data) > 0 {
			cloned.Data = cloneMetadata(notification.Data)
		}
		result[i] = cloned
	}
	return result
}

// Action describes an executable side effect bound to a workflow transition.
type Action func(ctx context.Context, input ActionInput) (ActionOutput, error)

// ActionInput captures the transition context passed to an action.
type ActionInput struct {
	Transition interfaces.TransitionInput
	Result     *interfaces.TransitionResult
}

// ActionOutput conveys side effects produced by an action.
type ActionOutput struct {
	Events        []interfaces.WorkflowEvent
	Notifications []interfaces.WorkflowNotification
	Metadata      map[string]any
}

// ActionResolver resolves the action bound to a transition (if any).
type ActionResolver func(entityType string, transition interfaces.WorkflowTransition) (Action, bool)

// ActionRegistry provides a simple map-backed ActionResolver.
type ActionRegistry map[string]Action

// Resolve returns the action registered for the supplied entity/transition combination.
func (r ActionRegistry) Resolve(entityType string, transition interfaces.WorkflowTransition) (Action, bool) {
	if len(r) == 0 {
		return nil, false
	}
	key := actionRegistryKey(entityType, transition.Name)
	if action, ok := r[key]; ok {
		return action, true
	}
	key = actionRegistryKey("", transition.Name)
	action, ok := r[key]
	return action, ok
}

func actionRegistryKey(entityType, transition string) string {
	return normalizeEntityType(entityType) + "::" + normalizeTransitionName(transition)
}
