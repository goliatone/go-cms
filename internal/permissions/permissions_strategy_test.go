package permissions

import (
	"context"
	"reflect"
	"testing"
)

type recordingChecker struct {
	allowed map[string]bool
	calls   []string
}

func newRecordingChecker(allowed ...string) *recordingChecker {
	set := make(map[string]bool, len(allowed))
	for _, perm := range allowed {
		set[perm] = true
	}
	return &recordingChecker{allowed: set}
}

func (c *recordingChecker) Allowed(permission string) bool {
	if c == nil {
		return false
	}
	c.calls = append(c.calls, permission)
	return c.allowed[permission]
}

func withEnvironmentScopeConfig(cfg EnvironmentScopeConfig) func() {
	previous := currentEnvironmentScopeConfig()
	ConfigureEnvironmentScope(cfg)
	return func() {
		ConfigureEnvironmentScope(previous)
	}
}

func TestRequire_EnvFirstStrategy(t *testing.T) {
	restore := withEnvironmentScopeConfig(EnvironmentScopeConfig{
		Enabled:  true,
		Strategy: EnvFirstStrategy,
	})
	defer restore()

	checker := newRecordingChecker("content_types:read")
	ctx := WithChecker(WithEnvironmentKey(context.Background(), "staging"), checker)

	if err := Require(ctx, "content_types:read"); err != nil {
		t.Fatalf("expected permission allowed, got error: %v", err)
	}

	expected := []string{"content_types:read@staging", "content_types:read"}
	if !reflect.DeepEqual(checker.calls, expected) {
		t.Fatalf("expected checks %v, got %v", expected, checker.calls)
	}
}

func TestRequire_GlobalFirstStrategy(t *testing.T) {
	restore := withEnvironmentScopeConfig(EnvironmentScopeConfig{
		Enabled:  true,
		Strategy: GlobalFirstStrategy,
	})
	defer restore()

	checker := newRecordingChecker("content_types:read@staging")
	ctx := WithChecker(WithEnvironmentKey(context.Background(), "staging"), checker)

	if err := Require(ctx, "content_types:read"); err != nil {
		t.Fatalf("expected permission allowed, got error: %v", err)
	}

	expected := []string{"content_types:read", "content_types:read@staging"}
	if !reflect.DeepEqual(checker.calls, expected) {
		t.Fatalf("expected checks %v, got %v", expected, checker.calls)
	}
}

func TestRequire_CustomStrategy(t *testing.T) {
	custom := StrategyFunc(func(permission, envKey string) []string {
		return []string{"custom:" + permission + "@" + envKey}
	})
	restore := withEnvironmentScopeConfig(EnvironmentScopeConfig{
		Enabled:  true,
		Strategy: custom,
	})
	defer restore()

	checker := newRecordingChecker("custom:content_types:read@staging")
	ctx := WithChecker(WithEnvironmentKey(context.Background(), "staging"), checker)

	if err := Require(ctx, "content_types:read"); err != nil {
		t.Fatalf("expected permission allowed, got error: %v", err)
	}

	expected := []string{"custom:content_types:read@staging"}
	if !reflect.DeepEqual(checker.calls, expected) {
		t.Fatalf("expected checks %v, got %v", expected, checker.calls)
	}
}
