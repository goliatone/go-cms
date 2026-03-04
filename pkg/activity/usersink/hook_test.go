package usersink_test

import (
	"context"
	"testing"
	"time"

	"github.com/goliatone/go-cms/pkg/activity"
	"github.com/goliatone/go-cms/pkg/activity/usersink"
	usertypes "github.com/goliatone/go-users/pkg/types"
	"github.com/google/uuid"
)

type recordingSink struct {
	records []usertypes.ActivityRecord
	err     error
}

func (s *recordingSink) Log(_ context.Context, record usertypes.ActivityRecord) error {
	s.records = append(s.records, record)
	return s.err
}

func TestHookNotifyMapsEvent(t *testing.T) {
	sink := &recordingSink{}
	hook := usersink.Hook{Sink: sink}

	now := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	actorID := uuid.New()
	userID := uuid.New()
	tenantID := uuid.New()
	orgID := uuid.New()
	objectID := uuid.New().String()

	event := activity.Event{
		Verb:           "update",
		ActorID:        actorID.String(),
		UserID:         userID.String(),
		TenantID:       tenantID.String(),
		ObjectType:     "content",
		ObjectID:       objectID,
		Channel:        "cms",
		DefinitionCode: "content:update",
		Recipients:     []string{"recipient@example.com"},
		Metadata: map[string]any{
			"locale":  "en",
			"org_id":  orgID.String(),
			"ip":      "203.0.113.10",
			"unused":  "value",
			"numeric": 7,
		},
		OccurredAt: now,
	}

	if err := hook.Notify(context.Background(), event); err != nil {
		t.Fatalf("notify: %v", err)
	}

	if len(sink.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sink.records))
	}
	record := sink.records[0]
	if record.ActorID != actorID {
		t.Fatalf("expected actor %s got %s", actorID, record.ActorID)
	}
	if record.UserID != userID {
		t.Fatalf("expected user %s got %s", userID, record.UserID)
	}
	if record.TenantID != tenantID {
		t.Fatalf("expected tenant %s got %s", tenantID, record.TenantID)
	}
	if record.OrgID != orgID {
		t.Fatalf("expected org %s got %s", orgID, record.OrgID)
	}
	if record.Verb != "update" || record.ObjectType != "content" || record.ObjectID != objectID {
		t.Fatalf("unexpected record payload: %+v", record)
	}
	if record.Channel != "cms" {
		t.Fatalf("expected channel cms got %q", record.Channel)
	}
	if record.IP != "203.0.113.10" {
		t.Fatalf("expected ip 203.0.113.10 got %q", record.IP)
	}
	if record.OccurredAt != now {
		t.Fatalf("expected occurred_at %v got %v", now, record.OccurredAt)
	}
	if record.Data["definition_code"] != "content:update" {
		t.Fatalf("expected definition_code metadata got %v", record.Data["definition_code"])
	}
	if record.Data["locale"] != "en" {
		t.Fatalf("expected locale metadata got %v", record.Data["locale"])
	}
	recipients, ok := record.Data["recipients"].([]string)
	if !ok || len(recipients) != 1 || recipients[0] != "recipient@example.com" {
		t.Fatalf("expected recipients metadata got %v", record.Data["recipients"])
	}
}

func TestHookNotifyMetadataFallbackTenantID(t *testing.T) {
	sink := &recordingSink{}
	hook := usersink.Hook{Sink: sink}

	tenantID := uuid.New()
	orgID := uuid.New()
	if err := hook.Notify(context.Background(), activity.Event{
		Verb:       "create",
		ObjectType: "content",
		ObjectID:   "cnt_1",
		Metadata: map[string]any{
			"tenant_id": tenantID.String(),
			"org_id":    orgID,
			"ip":        "198.51.100.20",
		},
	}); err != nil {
		t.Fatalf("notify: %v", err)
	}

	if len(sink.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(sink.records))
	}
	record := sink.records[0]
	if record.TenantID != tenantID {
		t.Fatalf("expected tenant fallback %s got %s", tenantID, record.TenantID)
	}
	if record.OrgID != orgID {
		t.Fatalf("expected org fallback %s got %s", orgID, record.OrgID)
	}
	if record.IP != "198.51.100.20" {
		t.Fatalf("expected ip fallback got %q", record.IP)
	}
}

func TestHookNotifySkipsMissingVerb(t *testing.T) {
	sink := &recordingSink{}
	hook := usersink.Hook{Sink: sink}

	_ = hook.Notify(context.Background(), activity.Event{})

	if len(sink.records) != 0 {
		t.Fatalf("expected no records for empty event, got %d", len(sink.records))
	}
}
