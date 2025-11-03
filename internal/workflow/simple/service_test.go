package simple

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
)

type transitionFixture struct {
	EntityType   string                  `json:"entity_type"`
	InitialState string                  `json:"initial_state"`
	Steps        []transitionFixtureStep `json:"steps"`
}

type transitionFixtureStep struct {
	Transition string `json:"transition"`
	WantState  string `json:"want_state"`
}

type transitionSummary struct {
	Transition string `json:"transition"`
	From       string `json:"from"`
	To         string `json:"to"`
}

type availableTransitionSummary struct {
	Name string `json:"name"`
	From string `json:"from"`
	To   string `json:"to"`
}

func TestEngine_DefaultWorkflowTransitions(t *testing.T) {
	ctx := context.Background()
	engine := New(WithClock(func() time.Time {
		return time.Unix(1700000000, 0).UTC()
	}))

	fixturePath := filepath.Join("testdata", "default_transitions.json")
	data, err := testsupport.LoadFixture(fixturePath)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	var fixture transitionFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	currentState := interfaces.WorkflowState(fixture.InitialState)
	entityID := uuid.New()
	actorID := uuid.New()
	var results []transitionSummary

	for idx, step := range fixture.Steps {
		res, err := engine.Transition(ctx, interfaces.TransitionInput{
			EntityID:     entityID,
			EntityType:   fixture.EntityType,
			CurrentState: currentState,
			Transition:   step.Transition,
			ActorID:      actorID,
		})
		if err != nil {
			t.Fatalf("step %d transition %q: %v", idx, step.Transition, err)
		}
		if string(res.ToState) != step.WantState {
			t.Fatalf("step %d transition %q: want %s got %s", idx, step.Transition, step.WantState, res.ToState)
		}
		if !res.CompletedAt.Equal(time.Unix(1700000000, 0).UTC()) {
			t.Fatalf("step %d transition %q: unexpected timestamp %s", idx, step.Transition, res.CompletedAt)
		}
		results = append(results, transitionSummary{
			Transition: res.Transition,
			From:       string(res.FromState),
			To:         string(res.ToState),
		})
		currentState = res.ToState
	}

	var want []transitionSummary
	goldenPath := filepath.Join("testdata", "default_transitions_golden.json")
	if err := testsupport.LoadGolden(goldenPath, &want); err != nil {
		t.Fatalf("load golden: %v", err)
	}

	if !reflect.DeepEqual(want, results) {
		wantJSON, _ := json.MarshalIndent(want, "", "  ")
		gotJSON, _ := json.MarshalIndent(results, "", "  ")
		t.Fatalf("transition results mismatch\nwant: %s\n got: %s", string(wantJSON), string(gotJSON))
	}

	available, err := engine.AvailableTransitions(ctx, interfaces.TransitionQuery{
		EntityType: fixture.EntityType,
		State:      interfaces.WorkflowState("approved"),
	})
	if err != nil {
		t.Fatalf("available transitions: %v", err)
	}

	gotAvail := make([]availableTransitionSummary, len(available))
	for i, item := range available {
		gotAvail[i] = availableTransitionSummary{
			Name: item.Name,
			From: string(item.From),
			To:   string(item.To),
		}
	}

	var wantAvail []availableTransitionSummary
	if err := testsupport.LoadGolden(filepath.Join("testdata", "approved_transitions_golden.json"), &wantAvail); err != nil {
		t.Fatalf("load available transitions golden: %v", err)
	}

	if !reflect.DeepEqual(wantAvail, gotAvail) {
		wantJSON, _ := json.MarshalIndent(wantAvail, "", "  ")
		gotJSON, _ := json.MarshalIndent(gotAvail, "", "  ")
		t.Fatalf("available transitions mismatch\nwant: %s\n got: %s", string(wantJSON), string(gotJSON))
	}
}
