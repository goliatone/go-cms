package adapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

func TestEngine_TransitionBlockedByGuard(t *testing.T) {
	ctx := context.Background()
	entityID := uuid.New()
	actorID := uuid.New()

	machine := &stubMachine{}
	authorizer := &stubAuthorizer{err: errors.New("denied")}

	engine, err := NewEngine(machine,
		WithAuthorizer(authorizer),
		WithClock(func() time.Time { return time.Unix(1700000000, 0) }),
	)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	definition := interfaces.WorkflowDefinition{
		EntityType: "article",
		States: []interfaces.WorkflowStateDefinition{
			{Name: "draft", Description: "Draft"},
			{Name: "published", Description: "Published"},
		},
		Transitions: []interfaces.WorkflowTransition{
			{Name: "publish", From: "draft", To: "published", Guard: "is_editor"},
		},
	}
	if err := engine.RegisterWorkflow(ctx, definition); err != nil {
		t.Fatalf("register workflow: %v", err)
	}

	_, err = engine.Transition(ctx, interfaces.TransitionInput{
		EntityID:     entityID,
		EntityType:   "article",
		CurrentState: "Draft",
		Transition:   "publish",
		ActorID:      actorID,
		Metadata:     map[string]any{"key": "value"},
	})
	if !errors.Is(err, ErrGuardRejected) {
		t.Fatalf("expected guard rejection, got %v", err)
	}
	if machine.transitionCalled {
		t.Fatalf("expected machine transition to be skipped when guard fails")
	}
	if authorizer.calls != 1 {
		t.Fatalf("expected authorizer to be called once, got %d", authorizer.calls)
	}
}

func TestEngine_TransitionExecutesAndNormalizes(t *testing.T) {
	ctx := context.Background()
	entityID := uuid.New()
	actorID := uuid.New()
	now := time.Unix(1800000000, 0)

	machine := &stubMachine{
		transitionResult: &interfaces.TransitionResult{
			EntityID:   entityID,
			EntityType: "ARTICLE",
			FromState:  "Draft",
			ToState:    "Published",
		},
	}
	authorizer := &stubAuthorizer{}

	engine, err := NewEngine(machine,
		WithAuthorizer(authorizer),
		WithClock(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	definition := interfaces.WorkflowDefinition{
		EntityType:   "Article",
		InitialState: "Draft",
		States: []interfaces.WorkflowStateDefinition{
			{Name: "draft"},
			{Name: "published"},
		},
		Transitions: []interfaces.WorkflowTransition{
			{Name: "Publish", From: "Draft", To: "Published", Guard: "role == admin"},
		},
	}
	if err := engine.RegisterWorkflow(ctx, definition); err != nil {
		t.Fatalf("register workflow: %v", err)
	}

	result, err := engine.Transition(ctx, interfaces.TransitionInput{
		EntityID:     entityID,
		EntityType:   "ARTICLE",
		CurrentState: "Draft",
		Transition:   "Publish",
		ActorID:      actorID,
	})
	if err != nil {
		t.Fatalf("transition: %v", err)
	}

	if machine.lastInput.Transition != "publish" {
		t.Fatalf("expected normalized transition, got %q", machine.lastInput.Transition)
	}
	if machine.lastInput.CurrentState != interfaces.WorkflowState("draft") {
		t.Fatalf("expected normalized current state, got %s", machine.lastInput.CurrentState)
	}
	if authorizer.lastGuard != "role == admin" {
		t.Fatalf("expected guard string to be passed, got %q", authorizer.lastGuard)
	}
	if authorizer.lastInput.Transition != "publish" {
		t.Fatalf("expected authorizer to receive normalized transition, got %q", authorizer.lastInput.Transition)
	}

	if result.FromState != "draft" || result.ToState != "published" {
		t.Fatalf("expected normalized states, got %s -> %s", result.FromState, result.ToState)
	}
	if !result.CompletedAt.Equal(now) {
		t.Fatalf("expected completed at %v, got %v", now, result.CompletedAt)
	}
}

func TestEngine_ActionOutputAppended(t *testing.T) {
	ctx := context.Background()
	entityID := uuid.New()
	actorID := uuid.New()
	now := time.Unix(1900000000, 0)

	machine := &stubMachine{
		transitionResult: &interfaces.TransitionResult{
			EntityID:   entityID,
			EntityType: "article",
			FromState:  "draft",
			ToState:    "published",
			Metadata:   map[string]any{"source": "engine"},
		},
	}

	registry := ActionRegistry{
		"article::publish": func(ctx context.Context, input ActionInput) (ActionOutput, error) {
			return ActionOutput{
				Events: []interfaces.WorkflowEvent{
					{Name: "page_published", Payload: map[string]any{"id": input.Result.EntityID}},
				},
				Notifications: []interfaces.WorkflowNotification{
					{Channel: "email", Message: "published"},
				},
				Metadata: map[string]any{"action": "publish"},
			}, nil
		},
	}

	engine, err := NewEngine(machine, WithActionRegistry(registry), WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	definition := interfaces.WorkflowDefinition{
		EntityType: "article",
		States: []interfaces.WorkflowStateDefinition{
			{Name: "draft"},
			{Name: "published"},
		},
		Transitions: []interfaces.WorkflowTransition{
			{Name: "publish", From: "draft", To: "published"},
		},
	}
	if err := engine.RegisterWorkflow(ctx, definition); err != nil {
		t.Fatalf("register workflow: %v", err)
	}

	result, err := engine.Transition(ctx, interfaces.TransitionInput{
		EntityID:     entityID,
		EntityType:   "article",
		CurrentState: "draft",
		Transition:   "publish",
		ActorID:      actorID,
	})
	if err != nil {
		t.Fatalf("transition: %v", err)
	}

	if len(result.Events) != 1 || result.Events[0].Name != "page_published" {
		t.Fatalf("expected action event appended, got %+v", result.Events)
	}
	if len(result.Notifications) != 1 || result.Notifications[0].Channel != "email" {
		t.Fatalf("expected action notification appended, got %+v", result.Notifications)
	}
	if result.Metadata["source"] != "engine" || result.Metadata["action"] != "publish" {
		t.Fatalf("expected merged metadata, got %+v", result.Metadata)
	}
}

func TestEngine_AvailableTransitionsNormalizesOutput(t *testing.T) {
	ctx := context.Background()
	machine := &stubMachine{
		transitions: []interfaces.WorkflowTransition{
			{Name: "Publish", From: "Draft", To: "Published"},
			{Name: "Archive", From: "Published", To: "Archived"},
		},
	}

	engine, err := NewEngine(machine)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	definition := interfaces.WorkflowDefinition{
		EntityType: "article",
		States: []interfaces.WorkflowStateDefinition{
			{Name: "draft"},
			{Name: "published"},
			{Name: "archived"},
		},
		Transitions: []interfaces.WorkflowTransition{
			{Name: "publish", From: "draft", To: "published"},
			{Name: "archive", From: "published", To: "archived"},
		},
	}
	if err := engine.RegisterWorkflow(ctx, definition); err != nil {
		t.Fatalf("register workflow: %v", err)
	}

	transitions, err := engine.AvailableTransitions(ctx, interfaces.TransitionQuery{
		EntityType: "ARTICLE",
		State:      "DRAFT",
	})
	if err != nil {
		t.Fatalf("available transitions: %v", err)
	}

	if machine.lastQuery.State != "draft" {
		t.Fatalf("expected normalized query state, got %s", machine.lastQuery.State)
	}

	if len(transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(transitions))
	}
	if transitions[0].Name != "publish" || transitions[0].From != "draft" || transitions[0].To != "published" {
		t.Fatalf("expected normalized transitions, got %+v", transitions[0])
	}
}

func TestEngine_NoOpTransitionSkipsMachine(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(2000000000, 0)
	machine := &stubMachine{}

	engine, err := NewEngine(machine, WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	definition := interfaces.WorkflowDefinition{
		EntityType: "article",
		States: []interfaces.WorkflowStateDefinition{
			{Name: "draft"},
		},
	}
	if err := engine.RegisterWorkflow(ctx, definition); err != nil {
		t.Fatalf("register workflow: %v", err)
	}

	entityID := uuid.New()
	result, err := engine.Transition(ctx, interfaces.TransitionInput{
		EntityID:     entityID,
		EntityType:   "article",
		CurrentState: "draft",
		ActorID:      uuid.New(),
	})
	if err != nil {
		t.Fatalf("transition: %v", err)
	}
	if machine.transitionCalled {
		t.Fatalf("expected machine to be skipped for no-op transition")
	}
	if result.ToState != "draft" || result.FromState != "draft" {
		t.Fatalf("expected state to remain draft, got %+v", result)
	}
	if !result.CompletedAt.Equal(now) {
		t.Fatalf("expected timestamp to be filled, got %v", result.CompletedAt)
	}
}

type stubMachine struct {
	transitionResult *interfaces.TransitionResult
	transitionErr    error
	transitions      []interfaces.WorkflowTransition
	availableErr     error
	registerErr      error

	transitionCalled bool
	lastInput        interfaces.TransitionInput
	lastQuery        interfaces.TransitionQuery
}

func (s *stubMachine) Transition(_ context.Context, input interfaces.TransitionInput) (*interfaces.TransitionResult, error) {
	s.transitionCalled = true
	s.lastInput = input
	if s.transitionErr != nil {
		return nil, s.transitionErr
	}
	if s.transitionResult == nil {
		return &interfaces.TransitionResult{
			EntityID:    input.EntityID,
			EntityType:  input.EntityType,
			Transition:  input.Transition,
			FromState:   input.CurrentState,
			ToState:     input.TargetState,
			CompletedAt: time.Now(),
			ActorID:     input.ActorID,
			Metadata:    cloneMetadata(input.Metadata),
		}, nil
	}
	clone := *s.transitionResult
	clone.Metadata = cloneMetadata(s.transitionResult.Metadata)
	clone.Events = cloneEvents(s.transitionResult.Events)
	clone.Notifications = cloneNotifications(s.transitionResult.Notifications)
	return &clone, nil
}

func (s *stubMachine) AvailableTransitions(_ context.Context, query interfaces.TransitionQuery) ([]interfaces.WorkflowTransition, error) {
	s.lastQuery = query
	if s.availableErr != nil {
		return nil, s.availableErr
	}
	out := make([]interfaces.WorkflowTransition, len(s.transitions))
	copy(out, s.transitions)
	return out, nil
}

func (s *stubMachine) RegisterWorkflow(_ context.Context, _ interfaces.WorkflowDefinition) error {
	if s.registerErr != nil {
		return s.registerErr
	}
	return nil
}

type stubAuthorizer struct {
	err       error
	calls     int
	lastGuard string
	lastInput interfaces.TransitionInput
}

func (s *stubAuthorizer) AuthorizeTransition(_ context.Context, input interfaces.TransitionInput, guard string) error {
	s.calls++
	s.lastGuard = guard
	s.lastInput = input
	return s.err
}
