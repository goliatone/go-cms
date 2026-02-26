package interfaces

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/goliatone/go-command/flow"
)

func TestWorkflowEnvelopeParityWithFlow(t *testing.T) {
	apply := ApplyEventRequest{
		MachineID:       "page",
		EntityID:        "page-1",
		Event:           "publish",
		ExpectedState:   "draft",
		ExpectedVersion: 3,
		IdempotencyKey:  "idem-1",
		ExecCtx: ExecutionContext{
			ActorID: "user-1",
			Tenant:  "tenant-1",
		},
		Metadata: map[string]any{"requestId": "req-1"},
		Msg: WorkflowMessage{
			TypeName: "workflow.test",
			Payload:  map[string]any{"current_state": "draft"},
		},
	}

	canonicalApply := flow.ApplyEventRequest[WorkflowMessage](apply)
	assertJSONEqual(t, apply, canonicalApply)

	snapshot := SnapshotRequest{
		MachineID:      "page",
		EntityID:       "page-1",
		EvaluateGuards: true,
		IncludeBlocked: true,
		ExecCtx: ExecutionContext{
			ActorID: "user-1",
			Tenant:  "tenant-1",
		},
		Msg: WorkflowMessage{
			TypeName: "workflow.test",
			Payload:  map[string]any{"current_state": "review"},
		},
	}

	canonicalSnapshot := flow.SnapshotRequest[WorkflowMessage](snapshot)
	assertJSONEqual(t, snapshot, canonicalSnapshot)
}

func assertJSONEqual(t *testing.T, left, right any) {
	t.Helper()
	leftJSON, err := json.Marshal(left)
	if err != nil {
		t.Fatalf("marshal left: %v", err)
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		t.Fatalf("marshal right: %v", err)
	}
	if !reflect.DeepEqual(leftJSON, rightJSON) {
		t.Fatalf("json mismatch\nleft:  %s\nright: %s", string(leftJSON), string(rightJSON))
	}
}
