package workflow_test

import (
	"strings"
	"testing"

	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
	"github.com/goliatone/go-cms/internal/workflow"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

func TestCompileDefinitionConfigs_Success(t *testing.T) {
	configs := []runtimeconfig.WorkflowDefinitionConfig{
		{
			Entity: "page",
			States: []runtimeconfig.WorkflowStateConfig{
				{Name: "draft", Description: "Draft content", Initial: true},
				{Name: "review", Description: "Under review"},
				{Name: "translated", Description: "Awaiting localisation"},
				{Name: "published", Description: "Published", Terminal: false},
			},
			Transitions: []runtimeconfig.WorkflowTransitionConfig{
				{Name: "submit_review", From: "draft", To: "review"},
				{Name: "translate", From: "review", To: "translated"},
				{Name: "publish", From: "translated", To: "published"},
			},
		},
	}

	defs, err := workflow.CompileDefinitionConfigs(configs)
	if err != nil {
		t.Fatalf("CompileDefinitionConfigs returned error: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected single definition, got %d", len(defs))
	}

	def := defs[0]
	if def.EntityType != "page" {
		t.Fatalf("expected entity 'page', got %q", def.EntityType)
	}
	if def.InitialState != interfaces.WorkflowState(domain.WorkflowStateDraft) {
		t.Fatalf("expected initial state 'draft', got %q", def.InitialState)
	}
	if len(def.States) != 4 {
		t.Fatalf("expected 4 states, got %d", len(def.States))
	}
	if def.States[0].Name != interfaces.WorkflowState(domain.WorkflowStateDraft) {
		t.Fatalf("expected first state to remain in input order, got %q", def.States[0].Name)
	}
	if len(def.Transitions) != 3 {
		t.Fatalf("expected 3 transitions, got %d", len(def.Transitions))
	}
}

func TestCompileDefinitionConfigs_DuplicateEntity(t *testing.T) {
	configs := []runtimeconfig.WorkflowDefinitionConfig{
		{Entity: "page", States: []runtimeconfig.WorkflowStateConfig{{Name: "draft"}}},
		{Entity: "page", States: []runtimeconfig.WorkflowStateConfig{{Name: "draft"}}},
	}

	_, err := workflow.CompileDefinitionConfigs(configs)
	if err == nil || !strings.Contains(err.Error(), "duplicate entity definition") {
		t.Fatalf("expected duplicate entity error, got %v", err)
	}
}

func TestCompileDefinitionConfigs_InvalidTransition(t *testing.T) {
	configs := []runtimeconfig.WorkflowDefinitionConfig{
		{
			Entity: "page",
			States: []runtimeconfig.WorkflowStateConfig{{Name: "draft"}},
			Transitions: []runtimeconfig.WorkflowTransitionConfig{
				{Name: "publish", From: "draft", To: "published"},
			},
		},
	}

	_, err := workflow.CompileDefinitionConfigs(configs)
	if err == nil || !strings.Contains(err.Error(), "unknown state") {
		t.Fatalf("expected unknown state error, got %v", err)
	}
}
