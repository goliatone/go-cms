package widgets

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func BenchmarkEvaluateVisibility(b *testing.B) {
	ctx := context.Background()
	now := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)

	service := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
		WithClock(func() time.Time { return now }),
	)

	locale := uuid.New()
	instance := &Instance{
		PublishOn: cloneTimePtr(now.Add(-time.Hour)),
		VisibilityRules: map[string]any{
			"schedule": map[string]any{
				"starts_at": now.Add(-30 * time.Minute).Format(time.RFC3339),
				"ends_at":   now.Add(30 * time.Minute).Format(time.RFC3339),
			},
			"audience": []string{"guest", "member"},
			"segments": []any{"beta", "control"},
			"locales":  []string{locale.String()},
		},
	}

	input := VisibilityContext{
		Now:      now,
		Audience: []string{"guest"},
		Segments: []string{"beta"},
		LocaleID: &locale,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if visible, err := service.EvaluateVisibility(ctx, instance, input); err != nil || !visible {
			b.Fatalf("evaluate visibility failed: visible=%v err=%v", visible, err)
		}
	}
}

func BenchmarkResolveAreaVisibility(b *testing.B) {
	ctx := context.Background()
	now := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)

	service := newServiceWithAreas(
		WithClock(func() time.Time { return now }),
	)

	if _, err := service.RegisterAreaDefinition(ctx, RegisterAreaDefinitionInput{
		Code: "sidebar.primary",
		Name: "Primary Sidebar",
	}); err != nil {
		b.Fatalf("register area definition: %v", err)
	}

	definition, err := service.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name: "promo",
		Schema: map[string]any{
			"fields": []any{map[string]any{"name": "headline"}},
		},
		Defaults: map[string]any{"headline": "Default"},
	})
	if err != nil {
		b.Fatalf("register definition: %v", err)
	}

	editorID := uuid.MustParse("00000000-0000-0000-0000-00000000beef")
	for i := 0; i < 64; i++ {
		rules := map[string]any{
			"segments": []any{"beta"},
		}
		if i%2 == 0 {
			rules["schedule"] = map[string]any{
				"starts_at": now.Add(time.Hour).Format(time.RFC3339),
			}
		}
		instance, err := service.CreateInstance(ctx, CreateInstanceInput{
			DefinitionID:    definition.ID,
			Configuration:   map[string]any{"headline": fmt.Sprintf("Announcement %d", i)},
			VisibilityRules: rules,
			CreatedBy:       editorID,
			UpdatedBy:       editorID,
			Position:        i,
		})
		if err != nil {
			b.Fatalf("create instance %d: %v", i, err)
		}

		if _, err := service.AssignWidgetToArea(ctx, AssignWidgetToAreaInput{
			AreaCode:   "sidebar.primary",
			InstanceID: instance.ID,
		}); err != nil {
			b.Fatalf("assign widget %d: %v", i, err)
		}
	}

	input := ResolveAreaInput{
		AreaCode: "sidebar.primary",
		Segments: []string{"beta"},
		Now:      now,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolved, err := service.ResolveArea(ctx, input)
		if err != nil {
			b.Fatalf("resolve area: %v", err)
		}
		if len(resolved) == 0 {
			b.Fatalf("expected resolved widgets")
		}
	}
}

func cloneTimePtr(value time.Time) *time.Time {
	v := value
	return &v
}
