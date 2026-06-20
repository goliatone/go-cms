package adapter

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/goliatone/go-cms/internal/domain"
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

// WorkflowState is the normalized lifecycle state used by this legacy adapter.
type WorkflowState string

// TransitionInput describes a requested workflow transition.
type TransitionInput struct {
	EntityID     uuid.UUID
	EntityType   string
	CurrentState WorkflowState
	TargetState  WorkflowState
	Transition   string
	ActorID      uuid.UUID
	Metadata     map[string]any
}

// TransitionQuery describes a request for transitions available from a state.
type TransitionQuery struct {
	EntityType string
	State      WorkflowState
	Context    map[string]any
}

// TransitionResult describes the completed transition.
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

// WorkflowEvent is emitted by workflow actions.
type WorkflowEvent struct {
	Name      string
	Timestamp time.Time
	Payload   map[string]any
}

// WorkflowNotification is produced by workflow actions.
type WorkflowNotification struct {
	Channel string
	Message string
	Data    map[string]any
}

// WorkflowAuthorizer gates guarded transitions.
type WorkflowAuthorizer interface {
	AuthorizeTransition(ctx context.Context, input TransitionInput, guard string) error
}

// WorkflowDefinition describes the states and transitions for an entity type.
type WorkflowDefinition struct {
	EntityType   string
	InitialState WorkflowState
	States       []WorkflowStateDefinition
	Transitions  []WorkflowTransition
}

// WorkflowStateDefinition describes a state in a workflow definition.
type WorkflowStateDefinition struct {
	Name        WorkflowState
	Description string
	Terminal    bool
}

// WorkflowTransition describes a transition in a workflow definition.
type WorkflowTransition struct {
	Name        string
	Description string
	From        WorkflowState
	To          WorkflowState
	Guard       string
}

type workflowEngine interface {
	Transition(ctx context.Context, input TransitionInput) (*TransitionResult, error)
	AvailableTransitions(ctx context.Context, query TransitionQuery) ([]WorkflowTransition, error)
	RegisterWorkflow(ctx context.Context, definition WorkflowDefinition) error
}

// Engine adapts a legacy workflow state machine behind a normalized API.
type Engine struct {
	machine workflowEngine

	authorizer     WorkflowAuthorizer
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
func WithAuthorizer(authorizer WorkflowAuthorizer) Option {
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
func (e *Engine) Transition(ctx context.Context, input TransitionInput) (*TransitionResult, error) {
	if input.EntityID == uuid.Nil {
		return nil, ErrNilEntityID
	}

	definition, err := e.definitionFor(input.EntityType)
	if err != nil {
		return nil, err
	}

	normalizedInput := normalizeInput(definition, input)

	transition, noOp, err := definition.transitionForInput(normalizedInput)
	if err != nil {
		return nil, err
	}
	if noOp {
		return e.noOpTransitionResult(definition, normalizedInput), nil
	}

	normalizedInput.Transition = transition.Name
	normalizedInput.TargetState = transition.To

	if authErr := e.authorizeTransition(ctx, normalizedInput, transition); authErr != nil {
		return nil, authErr
	}

	result, err := e.machine.Transition(ctx, normalizedInput)
	if err != nil {
		return nil, err
	}

	normalizedResult := normalizeResult(result, transition, normalizedInput, e.now)

	return e.applyTransitionAction(ctx, definition, transition, normalizedInput, normalizedResult)
}

func (e *Engine) noOpTransitionResult(definition *compiledDefinition, input TransitionInput) *TransitionResult {
	return &TransitionResult{
		EntityID:    input.EntityID,
		EntityType:  definition.definition.EntityType,
		Transition:  "",
		FromState:   input.CurrentState,
		ToState:     input.CurrentState,
		CompletedAt: e.now(),
		ActorID:     input.ActorID,
		Metadata:    cloneMetadata(input.Metadata),
	}
}

func (e *Engine) authorizeTransition(ctx context.Context, input TransitionInput, transition WorkflowTransition) error {
	guard := strings.TrimSpace(transition.Guard)
	if guard == "" {
		return nil
	}
	if e.authorizer == nil {
		return fmt.Errorf("%w: %s", ErrGuardAuthorizerRequired, guard)
	}
	guardInput := input
	guardInput.Metadata = cloneMetadata(guardInput.Metadata)
	if guardErr := e.authorizer.AuthorizeTransition(ctx, guardInput, guard); guardErr != nil {
		return fmt.Errorf("%w: %w", ErrGuardRejected, guardErr)
	}
	return nil
}

func (e *Engine) applyTransitionAction(
	ctx context.Context,
	definition *compiledDefinition,
	transition WorkflowTransition,
	input TransitionInput,
	result *TransitionResult,
) (*TransitionResult, error) {
	if action := e.resolveAction(definition, transition); action != nil {
		output, err := action(ctx, ActionInput{
			Transition: input,
			Result:     result,
		})
		if err != nil {
			return nil, err
		}
		if len(output.Events) > 0 {
			result.Events = append(result.Events, cloneEvents(output.Events)...)
		}
		if len(output.Notifications) > 0 {
			result.Notifications = append(result.Notifications, cloneNotifications(output.Notifications)...)
		}
		if len(output.Metadata) > 0 {
			result.Metadata = mergeMetadata(result.Metadata, output.Metadata)
		}
	}

	return result, nil
}

// AvailableTransitions returns the transitions reachable from the supplied state.
func (e *Engine) AvailableTransitions(ctx context.Context, query TransitionQuery) ([]WorkflowTransition, error) {
	definition, err := e.definitionFor(query.EntityType)
	if err != nil {
		return nil, err
	}

	normalizedQuery := TransitionQuery{
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
func (e *Engine) RegisterWorkflow(ctx context.Context, definition WorkflowDefinition) error {
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

func (e *Engine) resolveAction(definition *compiledDefinition, transition WorkflowTransition) Action {
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

func compileDefinition(definition WorkflowDefinition) *compiledDefinition {
	normalized := WorkflowDefinition{
		EntityType: normalizeEntityType(definition.EntityType),
		States:     make([]WorkflowStateDefinition, 0, len(definition.States)),
	}

	for _, state := range definition.States {
		normalizedState := WorkflowStateDefinition{
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
		normalized.InitialState = WorkflowState(domain.WorkflowStateDraft)
	}

	compiled := &compiledDefinition{
		definition:         normalized,
		transitions:        make(map[string]WorkflowTransition),
		transitionsByState: make(map[WorkflowState][]WorkflowTransition),
	}

	for _, transition := range definition.Transitions {
		normalizedTransition := WorkflowTransition{
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
	definition         WorkflowDefinition
	transitions        map[string]WorkflowTransition
	transitionsByState map[WorkflowState][]WorkflowTransition
}

func (d *compiledDefinition) transitionForInput(input TransitionInput) (WorkflowTransition, bool, error) {
	transitionName := normalizeTransitionName(input.Transition)
	targetState := normalizeWorkflowState(input.TargetState)
	current := input.CurrentState

	if transitionName == "" && targetState == "" {
		targetState = current
	}
	if transitionName == "" && targetState == current {
		return WorkflowTransition{}, true, nil
	}
	if transitionName != "" {
		transition, err := d.lookupTransition(transitionName, current)
		return transition, false, err
	}
	if targetState != "" {
		transition, err := d.lookupByStates(current, targetState)
		return transition, false, err
	}
	return WorkflowTransition{}, false, ErrMissingTransition
}

func (d *compiledDefinition) lookupTransition(name string, state WorkflowState) (WorkflowTransition, error) {
	key := transitionKey(name, normalizeWorkflowState(state))
	transition, ok := d.transitions[key]
	if !ok {
		return WorkflowTransition{}, fmt.Errorf("%w: %s from %s", ErrInvalidTransition, name, state)
	}
	return transition, nil
}

func (d *compiledDefinition) lookupByStates(from, to WorkflowState) (WorkflowTransition, error) {
	transitions := d.transitionsByState[normalizeWorkflowState(from)]
	target := normalizeWorkflowState(to)
	for _, candidate := range transitions {
		if normalizeWorkflowState(candidate.To) == target {
			return candidate, nil
		}
	}
	return WorkflowTransition{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
}

func normalizeInput(definition *compiledDefinition, input TransitionInput) TransitionInput {
	normalized := input
	normalized.EntityType = definition.definition.EntityType
	normalized.CurrentState = normalizeWorkflowStateWithDefault(input.CurrentState, definition.definition.InitialState)
	normalized.TargetState = normalizeWorkflowState(input.TargetState)
	normalized.Transition = normalizeTransitionName(input.Transition)
	normalized.Metadata = cloneMetadata(input.Metadata)
	return normalized
}

func normalizeWorkflowStateWithDefault(state WorkflowState, fallback WorkflowState) WorkflowState {
	if strings.TrimSpace(string(state)) == "" {
		return normalizeWorkflowState(fallback)
	}
	return normalizeWorkflowState(state)
}

func normalizeWorkflowState(state WorkflowState) WorkflowState {
	return WorkflowState(domain.NormalizeWorkflowState(string(state)))
}

func normalizeEntityType(entity string) string {
	return strings.ToLower(strings.TrimSpace(entity))
}

func normalizeTransitionName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func transitionKey(name string, from WorkflowState) string {
	return normalizeTransitionName(name) + "::" + string(normalizeWorkflowState(from))
}

func normalizeResult(result *TransitionResult, transition WorkflowTransition, input TransitionInput, clock func() time.Time) *TransitionResult {
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

func normalizeTransitions(transitions []WorkflowTransition) []WorkflowTransition {
	result := make([]WorkflowTransition, len(transitions))
	for i, transition := range transitions {
		result[i] = WorkflowTransition{
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
	maps.Copy(clone, input)
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
	maps.Copy(result, extra)
	return result
}

func cloneEvents(events []WorkflowEvent) []WorkflowEvent {
	if len(events) == 0 {
		return nil
	}
	result := make([]WorkflowEvent, len(events))
	for i, event := range events {
		cloned := WorkflowEvent{
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

func cloneNotifications(notifications []WorkflowNotification) []WorkflowNotification {
	if len(notifications) == 0 {
		return nil
	}
	result := make([]WorkflowNotification, len(notifications))
	for i, notification := range notifications {
		cloned := WorkflowNotification{
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
	Transition TransitionInput
	Result     *TransitionResult
}

// ActionOutput conveys side effects produced by an action.
type ActionOutput struct {
	Events        []WorkflowEvent
	Notifications []WorkflowNotification
	Metadata      map[string]any
}

// ActionResolver resolves the action bound to a transition (if any).
type ActionResolver func(entityType string, transition WorkflowTransition) (Action, bool)

// ActionRegistry provides a simple map-backed ActionResolver.
type ActionRegistry map[string]Action

// Resolve returns the action registered for the supplied entity/transition combination.
func (r ActionRegistry) Resolve(entityType string, transition WorkflowTransition) (Action, bool) {
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
