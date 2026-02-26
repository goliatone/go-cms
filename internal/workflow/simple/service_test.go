package simple

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-cms/pkg/testsupport"
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

	currentState := strings.TrimSpace(strings.ToLower(fixture.InitialState))
	entityID := "fixture-entity-1"
	var results []transitionSummary

	for idx, step := range fixture.Steps {
		res, err := engine.ApplyEvent(ctx, interfaces.ApplyEventRequest{
			MachineID:     fixture.EntityType,
			EntityID:      entityID,
			Event:         step.Transition,
			ExpectedState: currentState,
			ExecCtx: interfaces.ExecutionContext{
				ActorID: "tester",
			},
			Msg: interfaces.WorkflowMessage{
				TypeName: "test.workflow",
				Payload: map[string]any{
					"current_state": currentState,
				},
			},
		})
		if err != nil {
			t.Fatalf("step %d transition %q: %v", idx, step.Transition, err)
		}
		if res == nil || res.Transition == nil {
			t.Fatalf("step %d transition %q: expected transition response", idx, step.Transition)
		}
		if strings.TrimSpace(res.Transition.CurrentState) != step.WantState {
			t.Fatalf("step %d transition %q: want %s got %s", idx, step.Transition, step.WantState, res.Transition.CurrentState)
		}
		results = append(results, transitionSummary{
			Transition: step.Transition,
			From:       strings.TrimSpace(res.Transition.PreviousState),
			To:         strings.TrimSpace(res.Transition.CurrentState),
		})
		currentState = strings.TrimSpace(res.Transition.CurrentState)
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

	snapshot, err := engine.Snapshot(ctx, interfaces.SnapshotRequest{
		MachineID:      fixture.EntityType,
		EntityID:       entityID,
		EvaluateGuards: true,
		IncludeBlocked: true,
		Msg: interfaces.WorkflowMessage{
			TypeName: "test.workflow.snapshot",
			Payload: map[string]any{
				"current_state": "approved",
			},
		},
	})
	if err != nil {
		t.Fatalf("available transitions: %v", err)
	}

	gotAvail := make([]availableTransitionSummary, len(snapshot.AllowedTransitions))
	for i, item := range snapshot.AllowedTransitions {
		gotAvail[i] = availableTransitionSummary{
			Name: item.Event,
			From: "approved",
			To: func() string {
				if strings.TrimSpace(item.Target.ResolvedTo) != "" {
					return strings.TrimSpace(item.Target.ResolvedTo)
				}
				return strings.TrimSpace(item.Target.To)
			}(),
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
