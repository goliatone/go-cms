package simple

import (
	"context"
	stdErrors "errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-command/flow"
	apperrors "github.com/goliatone/go-errors"
)

const (
	entityTypePage = "page"
)

var (
	// ErrMachineIDRequired indicates a workflow machine identifier is missing.
	ErrMachineIDRequired = stdErrors.New("workflow: machine id required")
	// ErrMachineNotRegistered indicates no workflow machine exists for the requested id.
	ErrMachineNotRegistered = stdErrors.New("workflow: machine not registered")
)

// Option configures the engine.
type Option func(*Engine)

// Engine is a deterministic in-memory workflow engine aligned to flow envelopes.
type Engine struct {
	mu sync.RWMutex

	machines map[string]*workflowMachine
	states   map[string]map[string]stateRecord

	now       func() time.Time
	resolvers *flow.ResolverMap[interfaces.WorkflowMessage]
	actions   *flow.ActionRegistry[interfaces.WorkflowMessage]
}

type workflowMachine struct {
	definition      interfaces.MachineDefinition
	compiled        *flow.CompiledMachine[interfaces.WorkflowMessage]
	transitions     map[string]flow.CompiledTransition[interfaces.WorkflowMessage]
	byState         map[string][]flow.CompiledTransition[interfaces.WorkflowMessage]
	stateCandidates []string
}

type stateRecord struct {
	state   string
	version int
}

// WithClock overrides the clock used for deterministic test assertions.
func WithClock(clock func() time.Time) Option {
	return func(e *Engine) {
		if clock != nil {
			e.now = clock
		}
	}
}

// New constructs a workflow engine seeded with the default page machine.
func New(opts ...Option) *Engine {
	engine := &Engine{
		machines:  make(map[string]*workflowMachine),
		states:    make(map[string]map[string]stateRecord),
		now:       time.Now,
		resolvers: flow.NewResolverMap[interfaces.WorkflowMessage](),
		actions:   flow.NewActionRegistry[interfaces.WorkflowMessage](),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(engine)
		}
	}
	_ = engine.RegisterMachine(context.Background(), defaultPageWorkflowDefinition())
	return engine
}

// RegisterMachine installs or replaces a machine definition.
func (e *Engine) RegisterMachine(_ context.Context, definition interfaces.MachineDefinition) error {
	normalized, err := normalizeMachineDefinition(definition)
	if err != nil {
		return err
	}

	compiled, err := flow.CompileMachine[interfaces.WorkflowMessage](&normalized, e.resolvers)
	if err != nil {
		return err
	}

	machine := &workflowMachine{
		definition:      normalized,
		compiled:        compiled,
		transitions:     make(map[string]flow.CompiledTransition[interfaces.WorkflowMessage], len(compiled.Transitions)),
		byState:         make(map[string][]flow.CompiledTransition[interfaces.WorkflowMessage]),
		stateCandidates: make([]string, 0, len(normalized.States)),
	}

	stateSet := make(map[string]struct{}, len(normalized.States))
	for _, state := range normalized.States {
		ns := normalizeState(state.Name)
		if ns == "" {
			continue
		}
		if _, ok := stateSet[ns]; ok {
			continue
		}
		stateSet[ns] = struct{}{}
		machine.stateCandidates = append(machine.stateCandidates, ns)
	}
	sort.Strings(machine.stateCandidates)

	for _, tr := range compiled.Transitions {
		key := transitionKey(tr.From, tr.Event)
		machine.transitions[key] = tr
		from := normalizeState(tr.From)
		machine.byState[from] = append(machine.byState[from], tr)
	}
	for state := range machine.byState {
		sort.Slice(machine.byState[state], func(i, j int) bool {
			left := machine.byState[state][i]
			right := machine.byState[state][j]
			if left.Event == right.Event {
				return left.ID < right.ID
			}
			return left.Event < right.Event
		})
	}

	machineID := normalizeMachineID(normalized.ID)
	e.mu.Lock()
	e.machines[machineID] = machine
	if _, ok := e.states[machineID]; !ok {
		e.states[machineID] = make(map[string]stateRecord)
	}
	e.mu.Unlock()

	return nil
}

// RegisterGuard stores a named guard in the resolver registry.
func (e *Engine) RegisterGuard(name string, guard interfaces.Guard) error {
	if e == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" || guard == nil {
		return nil
	}
	e.resolvers.RegisterGuard(name, guard)
	return nil
}

// RegisterDynamicTarget stores a named dynamic target resolver.
func (e *Engine) RegisterDynamicTarget(name string, resolver interfaces.DynamicTargetResolver) error {
	if e == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" || resolver == nil {
		return nil
	}
	e.resolvers.RegisterDynamicTarget(name, resolver)
	return nil
}

// RegisterAction stores a named action in the registry.
func (e *Engine) RegisterAction(name string, action interfaces.Action) error {
	if e == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" || action == nil {
		return nil
	}
	return e.actions.Register(name, action)
}

// ApplyEvent applies one canonical event envelope against the target machine.
func (e *Engine) ApplyEvent(ctx context.Context, req interfaces.ApplyEventRequest) (*interfaces.ApplyEventResponse, error) {
	machineID, machine, err := e.machineForRequest(req.MachineID)
	if err != nil {
		return nil, err
	}

	entityID := strings.TrimSpace(req.EntityID)
	if entityID == "" {
		return nil, runtimeError(flow.ErrPreconditionFailed, "entity id is required", nil, map[string]any{"machine_id": machineID})
	}
	event := normalizeEvent(req.Event)
	if event == "" {
		return nil, runtimeError(flow.ErrPreconditionFailed, "event is required", nil, map[string]any{"machine_id": machineID, "entity_id": entityID})
	}

	current, version := e.currentState(machineID, machine, entityID, req)
	if expected := normalizeState(req.ExpectedState); expected != "" && expected != current {
		return nil, runtimeError(
			flow.ErrPreconditionFailed,
			fmt.Sprintf("expected state %q, got %q", expected, current),
			nil,
			map[string]any{"machine_id": machineID, "entity_id": entityID},
		)
	}
	if req.ExpectedVersion > 0 && req.ExpectedVersion != version {
		return nil, runtimeError(
			flow.ErrPreconditionFailed,
			fmt.Sprintf("expected version %d, got %d", req.ExpectedVersion, version),
			nil,
			map[string]any{"machine_id": machineID, "entity_id": entityID},
		)
	}

	transition, ok := machine.transitions[transitionKey(current, event)]
	if !ok {
		return nil, runtimeError(
			flow.ErrInvalidTransition,
			fmt.Sprintf("no transition for state=%s event=%s", current, event),
			nil,
			map[string]any{"machine_id": machineID, "entity_id": entityID, "event": event},
		)
	}

	if rejection, blocked := e.evaluateTransitionGuards(ctx, req, transition); blocked {
		return nil, runtimeError(
			flow.ErrGuardRejected,
			rejection.Message,
			nil,
			map[string]any{
				"machine_id":        machineID,
				"entity_id":         entityID,
				"event":             event,
				"transition_id":     transition.ID,
				"guard_rejection":   rejection,
				"guard_rejections":  []flow.GuardRejection{rejection},
				"guard_reject_code": rejection.Code,
			},
		)
	}

	next, err := e.resolveTarget(ctx, req, machine, transition)
	if err != nil {
		return nil, err
	}

	effects := compileEffects(transition)
	result := &interfaces.TransitionResult{
		PreviousState: current,
		CurrentState:  next,
		Effects:       effects,
	}

	outVersion := version
	if !req.DryRun {
		outVersion = e.persistState(machineID, entityID, next, version)
	}

	snapshot := e.buildSnapshotForState(ctx, machineID, machine, interfaces.SnapshotRequest{
		MachineID: req.MachineID,
		EntityID:  req.EntityID,
		Msg:       req.Msg,
		ExecCtx:   req.ExecCtx,
	}, next, outVersion)

	return &interfaces.ApplyEventResponse{
		EventID:        fmt.Sprintf("evt_%d", e.now().UnixNano()),
		Version:        outVersion,
		Transition:     result,
		Snapshot:       snapshot,
		IdempotencyHit: false,
	}, nil
}

// Snapshot computes transition availability for the current machine/entity state.
func (e *Engine) Snapshot(ctx context.Context, req interfaces.SnapshotRequest) (*interfaces.Snapshot, error) {
	machineID, machine, err := e.machineForRequest(req.MachineID)
	if err != nil {
		return nil, err
	}
	entityID := strings.TrimSpace(req.EntityID)
	if entityID == "" {
		return nil, runtimeError(flow.ErrPreconditionFailed, "entity id is required", nil, map[string]any{"machine_id": machineID})
	}

	e.mu.RLock()
	record, hasRecord := e.states[machineID][entityID]
	e.mu.RUnlock()
	if !hasRecord {
		fallback := fallbackState(req)
		if fallback == "" {
			return nil, runtimeError(
				flow.ErrStateNotFound,
				"current state missing",
				nil,
				map[string]any{"machine_id": machineID, "entity_id": entityID},
			)
		}
		record = stateRecord{state: fallback, version: 0}
	} else if fallback := fallbackState(req); fallback != "" {
		record.state = fallback
	}

	return e.buildSnapshotForState(ctx, machineID, machine, req, record.state, record.version), nil
}

func (e *Engine) machineForRequest(requestMachineID string) (string, *workflowMachine, error) {
	machineID := normalizeMachineID(requestMachineID)
	if machineID == "" {
		machineID = entityTypePage
	}

	e.mu.RLock()
	machine, ok := e.machines[machineID]
	e.mu.RUnlock()
	if !ok || machine == nil {
		return "", nil, runtimeError(
			flow.ErrStateNotFound,
			fmt.Sprintf("machine %q not registered", machineID),
			ErrMachineNotRegistered,
			map[string]any{"machine_id": machineID},
		)
	}
	return machineID, machine, nil
}

func (e *Engine) currentState(machineID string, machine *workflowMachine, entityID string, req interfaces.ApplyEventRequest) (string, int) {
	e.mu.RLock()
	record, ok := e.states[machineID][entityID]
	e.mu.RUnlock()
	if ok {
		return normalizeState(record.state), record.version
	}
	if expected := normalizeState(req.ExpectedState); expected != "" {
		return expected, 0
	}
	if fallback := fallbackStateFromMessage(req.Msg); fallback != "" {
		return fallback, 0
	}
	initial := machine.definitionInitialState()
	if initial == "" {
		initial = string(domain.WorkflowStateDraft)
	}
	return normalizeState(initial), 0
}

func (e *Engine) persistState(machineID, entityID, state string, priorVersion int) int {
	next := priorVersion + 1
	e.mu.Lock()
	if _, ok := e.states[machineID]; !ok {
		e.states[machineID] = make(map[string]stateRecord)
	}
	e.states[machineID][entityID] = stateRecord{state: state, version: next}
	e.mu.Unlock()
	return next
}

func (e *Engine) resolveTarget(ctx context.Context, req interfaces.ApplyEventRequest, machine *workflowMachine, tr flow.CompiledTransition[interfaces.WorkflowMessage]) (string, error) {
	next := normalizeState(tr.To)
	if tr.DynamicTo != nil {
		resolved, err := tr.DynamicTo(ctx, req.Msg, req.ExecCtx)
		if err != nil {
			return "", runtimeError(flow.ErrPreconditionFailed, "failed to resolve transition target", err, map[string]any{"transition_id": tr.ID})
		}
		next = normalizeState(resolved)
	}
	if next == "" {
		return "", runtimeError(flow.ErrStateNotFound, "resolved target state missing", nil, map[string]any{"transition_id": tr.ID})
	}
	if !slices.Contains(machine.stateCandidates, next) {
		return "", runtimeError(
			flow.ErrStateNotFound,
			fmt.Sprintf("target state %q is not declared", next),
			nil,
			map[string]any{"transition_id": tr.ID, "target_state": next},
		)
	}
	return next, nil
}

func (e *Engine) evaluateTransitionGuards(
	ctx context.Context,
	req interfaces.ApplyEventRequest,
	tr flow.CompiledTransition[interfaces.WorkflowMessage],
) (flow.GuardRejection, bool) {
	for idx, guard := range tr.Guards {
		if guard == nil {
			continue
		}
		if err := guard(ctx, req.Msg, req.ExecCtx); err != nil {
			return guardRejectionFromError(idx, err), true
		}
	}
	return flow.GuardRejection{}, false
}

func (e *Engine) buildSnapshotForState(
	ctx context.Context,
	machineID string,
	machine *workflowMachine,
	req interfaces.SnapshotRequest,
	state string,
	version int,
) *interfaces.Snapshot {
	current := normalizeState(state)
	transitions := machine.byState[current]
	items := make([]interfaces.TransitionInfo, 0, len(transitions))
	for _, tr := range transitions {
		info := interfaces.TransitionInfo{
			ID:       strings.TrimSpace(tr.ID),
			Event:    normalizeEvent(tr.Event),
			Target:   buildTargetInfo(ctx, req, machine, tr),
			Allowed:  true,
			Metadata: cloneMetadata(tr.Metadata),
		}

		if req.EvaluateGuards {
			rejections := e.collectGuardRejections(ctx, req, tr)
			info.Rejections = rejections
			info.Allowed = len(rejections) == 0
			if !info.Allowed && !req.IncludeBlocked {
				continue
			}
		}
		items = append(items, info)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Event == items[j].Event {
			return items[i].ID < items[j].ID
		}
		return items[i].Event < items[j].Event
	})

	return &interfaces.Snapshot{
		EntityID:           strings.TrimSpace(req.EntityID),
		CurrentState:       current,
		AllowedTransitions: items,
		Metadata: map[string]any{
			"machine_id":      machineID,
			"machine_version": strings.TrimSpace(machine.definition.Version),
			"version":         version,
		},
	}
}

func (e *Engine) collectGuardRejections(
	ctx context.Context,
	req interfaces.SnapshotRequest,
	tr flow.CompiledTransition[interfaces.WorkflowMessage],
) []interfaces.GuardRejection {
	if len(tr.Guards) == 0 {
		return nil
	}
	out := make([]interfaces.GuardRejection, 0, len(tr.Guards))
	for idx, guard := range tr.Guards {
		if guard == nil {
			continue
		}
		if err := guard(ctx, req.Msg, req.ExecCtx); err != nil {
			out = append(out, guardRejectionFromError(idx, err))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildTargetInfo(
	ctx context.Context,
	req interfaces.SnapshotRequest,
	machine *workflowMachine,
	tr flow.CompiledTransition[interfaces.WorkflowMessage],
) interfaces.TargetInfo {
	target := interfaces.TargetInfo{
		Kind:       "static",
		To:         normalizeState(tr.To),
		Candidates: append([]string(nil), machine.stateCandidates...),
	}
	if tr.DynamicTo == nil {
		return target
	}
	target.Kind = "dynamic"
	target.Resolver = strings.TrimSpace(tr.DynamicResolver)
	resolved, err := tr.DynamicTo(ctx, req.Msg, req.ExecCtx)
	if err != nil {
		return target
	}
	target.Resolved = true
	target.ResolvedTo = normalizeState(resolved)
	return target
}

func compileEffects(tr flow.CompiledTransition[interfaces.WorkflowMessage]) []interfaces.Effect {
	if len(tr.Plan.Nodes) == 0 {
		return nil
	}
	effects := make([]interfaces.Effect, 0, len(tr.Plan.Nodes))
	for _, node := range tr.Plan.Nodes {
		if node.Step == nil {
			continue
		}
		actionID := strings.TrimSpace(node.Step.ActionID)
		if actionID == "" {
			continue
		}
		effects = append(effects, interfaces.CommandEffect{
			ActionID: actionID,
			Async:    node.Step.Async,
			Delay:    node.Step.Delay,
			Timeout:  node.Step.Timeout,
			Metadata: cloneMetadata(node.Step.Metadata),
		})
	}
	if len(effects) == 0 {
		return nil
	}
	return effects
}

func normalizeMachineDefinition(definition interfaces.MachineDefinition) (interfaces.MachineDefinition, error) {
	id := normalizeMachineID(definition.ID)
	if id == "" {
		return interfaces.MachineDefinition{}, ErrMachineIDRequired
	}
	normalized := interfaces.MachineDefinition{
		ID:      id,
		Name:    strings.TrimSpace(definition.Name),
		Version: strings.TrimSpace(definition.Version),
		States:  make([]flow.StateDefinition, 0, len(definition.States)),
	}
	for _, state := range definition.States {
		normalized.States = append(normalized.States, flow.StateDefinition{
			Name:     normalizeState(state.Name),
			Initial:  state.Initial,
			Terminal: state.Terminal,
			Metadata: cloneMetadata(state.Metadata),
		})
	}
	normalized.Transitions = make([]flow.TransitionDefinition, 0, len(definition.Transitions))
	for _, transition := range definition.Transitions {
		copyTransition := flow.TransitionDefinition{
			ID:        strings.TrimSpace(transition.ID),
			Event:     normalizeEvent(transition.Event),
			From:      normalizeState(transition.From),
			To:        normalizeState(transition.To),
			DynamicTo: transition.DynamicTo,
			Workflow:  transition.Workflow,
			Metadata:  cloneMetadata(transition.Metadata),
		}
		if len(transition.Guards) > 0 {
			copyTransition.Guards = make([]flow.GuardDefinition, len(transition.Guards))
			copy(copyTransition.Guards, transition.Guards)
			for idx := range copyTransition.Guards {
				copyTransition.Guards[idx].Ref = strings.TrimSpace(copyTransition.Guards[idx].Ref)
				copyTransition.Guards[idx].Expr = strings.TrimSpace(copyTransition.Guards[idx].Expr)
				copyTransition.Guards[idx].Type = strings.TrimSpace(copyTransition.Guards[idx].Type)
			}
		}
		normalized.Transitions = append(normalized.Transitions, copyTransition)
	}
	return normalized, nil
}

func defaultPageWorkflowDefinition() interfaces.MachineDefinition {
	return interfaces.MachineDefinition{
		ID:      entityTypePage,
		Name:    "Page Workflow",
		Version: "v1",
		States: []flow.StateDefinition{
			{Name: string(domain.WorkflowStateDraft), Initial: true},
			{Name: string(domain.WorkflowStateReview)},
			{Name: string(domain.WorkflowStateApproved)},
			{Name: string(domain.WorkflowStateScheduled)},
			{Name: string(domain.WorkflowStatePublished)},
			{Name: string(domain.WorkflowStateArchived), Terminal: true},
		},
		Transitions: []flow.TransitionDefinition{
			{ID: "page.submit_review", Event: "submit_review", From: string(domain.WorkflowStateDraft), To: string(domain.WorkflowStateReview)},
			{ID: "page.approve", Event: "approve", From: string(domain.WorkflowStateReview), To: string(domain.WorkflowStateApproved)},
			{ID: "page.reject", Event: "reject", From: string(domain.WorkflowStateReview), To: string(domain.WorkflowStateDraft)},
			{ID: "page.request_changes", Event: "request_changes", From: string(domain.WorkflowStateApproved), To: string(domain.WorkflowStateReview)},
			{ID: "page.publish_approved", Event: "publish", From: string(domain.WorkflowStateApproved), To: string(domain.WorkflowStatePublished)},
			{ID: "page.publish_draft", Event: "publish", From: string(domain.WorkflowStateDraft), To: string(domain.WorkflowStatePublished)},
			{ID: "page.publish_scheduled", Event: "publish", From: string(domain.WorkflowStateScheduled), To: string(domain.WorkflowStatePublished)},
			{ID: "page.unpublish", Event: "unpublish", From: string(domain.WorkflowStatePublished), To: string(domain.WorkflowStateDraft)},
			{ID: "page.archive_draft", Event: "archive", From: string(domain.WorkflowStateDraft), To: string(domain.WorkflowStateArchived)},
			{ID: "page.archive_published", Event: "archive", From: string(domain.WorkflowStatePublished), To: string(domain.WorkflowStateArchived)},
			{ID: "page.restore", Event: "restore", From: string(domain.WorkflowStateArchived), To: string(domain.WorkflowStateDraft)},
			{ID: "page.schedule_draft", Event: "schedule", From: string(domain.WorkflowStateDraft), To: string(domain.WorkflowStateScheduled)},
			{ID: "page.schedule_approved", Event: "schedule", From: string(domain.WorkflowStateApproved), To: string(domain.WorkflowStateScheduled)},
			{ID: "page.cancel_schedule", Event: "cancel_schedule", From: string(domain.WorkflowStateScheduled), To: string(domain.WorkflowStateDraft)},
		},
	}
}

func (m *workflowMachine) definitionInitialState() string {
	if m == nil {
		return ""
	}
	for _, state := range m.definition.States {
		if state.Initial {
			return normalizeState(state.Name)
		}
	}
	if len(m.definition.States) == 0 {
		return ""
	}
	return normalizeState(m.definition.States[0].Name)
}

func fallbackState(req interfaces.SnapshotRequest) string {
	if state := fallbackStateFromMessage(req.Msg); state != "" {
		return state
	}
	return ""
}

func fallbackStateFromMessage(msg interfaces.WorkflowMessage) string {
	if msg.Payload == nil {
		return ""
	}
	if raw, ok := msg.Payload["current_state"].(string); ok {
		return normalizeState(raw)
	}
	return ""
}

func normalizeMachineID(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func normalizeState(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return string(domain.NormalizeWorkflowState(value))
}

func normalizeEvent(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func transitionKey(state, event string) string {
	return normalizeState(state) + "::" + normalizeEvent(event)
}

func cloneMetadata(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	maps.Copy(dst, src)
	return dst
}

func runtimeError(base *apperrors.Error, message string, source error, metadata map[string]any) error {
	if base == nil {
		base = flow.ErrPreconditionFailed
	}
	err := base.Clone()
	if text := strings.TrimSpace(message); text != "" {
		err.Message = text
	}
	if source != nil {
		err.Source = source
	}
	if len(metadata) > 0 {
		err = err.WithMetadata(metadata)
	}
	return err
}

func guardRejectionFromError(idx int, err error) flow.GuardRejection {
	rejection := flow.GuardRejection{
		Code:      fmt.Sprintf("GUARD_%d_REJECTED", idx),
		Category:  flow.GuardClassificationUnexpectedFailure,
		Message:   "guard rejected",
		Retryable: false,
	}
	if err == nil {
		return rejection
	}

	var structured *flow.GuardRejection
	if stdErrors.As(err, &structured) && structured != nil {
		rejection = *structured
		if strings.TrimSpace(rejection.Code) == "" {
			rejection.Code = fmt.Sprintf("GUARD_%d_REJECTED", idx)
		}
		if strings.TrimSpace(rejection.Category) == "" {
			rejection.Category = flow.GuardClassificationDomainReject
		}
		if strings.TrimSpace(rejection.Message) == "" {
			rejection.Message = strings.TrimSpace(err.Error())
		}
		return rejection
	}

	var appErr *apperrors.Error
	if stdErrors.As(err, &appErr) {
		if strings.TrimSpace(appErr.TextCode) != "" {
			rejection.Code = appErr.TextCode
		}
	}

	if message := strings.TrimSpace(err.Error()); message != "" {
		rejection.Message = message
	}
	return rejection
}
