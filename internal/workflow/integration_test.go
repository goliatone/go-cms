package workflow_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
)

type multiStepWorkflowFixture struct {
	Definition struct {
		Entity      string                      `json:"entity"`
		States      []workflowStateFixture      `json:"states"`
		Transitions []workflowTransitionFixture `json:"transitions"`
	} `json:"definition"`
	Steps []workflowStepFixture `json:"steps"`
}

type workflowStateFixture struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Terminal    bool   `json:"terminal"`
	Initial     bool   `json:"initial"`
}

type workflowTransitionFixture struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	From        string `json:"from"`
	To          string `json:"to"`
	Guard       string `json:"guard"`
}

type workflowStepFixture struct {
	TargetState string `json:"target_state"`
}

func TestWorkflowIntegration_MultiStepPageLifecycle(t *testing.T) {
	ctx := context.Background()

	data, err := testsupport.LoadFixture("testdata/multistep_page_workflow.json")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	var fixture multiStepWorkflowFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	cfg := cms.DefaultConfig()
	cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{
		{
			Entity: fixture.Definition.Entity,
			States: convertFixtureStates(fixture.Definition.States),
			Transitions: convertFixtureTransitions(
				fixture.Definition.Transitions,
			),
		},
	}

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("new cms module: %v", err)
	}

	typeRepo := module.Container().ContentTypeRepository()
	seedTypes, ok := typeRepo.(interface{ Put(*content.ContentType) })
	if !ok {
		t.Fatalf("expected seedable content type repository, got %T", typeRepo)
	}
	contentTypeID := uuid.New()
	seedTypes.Put(&content.ContentType{ID: contentTypeID, Name: "integration"})

	localeRepo := module.Container().LocaleRepository()
	seedLocales, ok := localeRepo.(interface{ Put(*content.Locale) })
	if !ok {
		t.Fatalf("expected seedable locale repository, got %T", localeRepo)
	}
	seedLocales.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentSvc := module.Content()
	authorID := uuid.New()
	contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "workflow-multistep",
		Status:        "draft",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Workflow Multi-step"},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := module.Pages()
	translations := []pages.PageTranslationInput{
		{Locale: "en", Title: "Workflow Multi-step", Path: "/workflow-multistep"},
	}

	page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  contentRecord.ID,
		TemplateID: uuid.New(),
		Slug:       "workflow-multistep",
		CreatedBy:  authorID,
		UpdatedBy:  authorID,
		Translations: append([]pages.PageTranslationInput(nil),
			translations...),
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	initialState := deriveInitialState(fixture.Definition.States)
	if !strings.EqualFold(page.Status, initialState) {
		t.Fatalf("expected initial status %q, got %q", initialState, page.Status)
	}

	engine := module.Container().WorkflowEngine()
	if engine == nil {
		t.Fatalf("expected workflow engine to be configured")
	}

	// Ensure custom transitions registered.
	available, err := engine.AvailableTransitions(ctx, interfaces.TransitionQuery{
		EntityType: fixture.Definition.Entity,
		State:      interfaces.WorkflowState("review"),
	})
	if err != nil {
		t.Fatalf("available transitions: %v", err)
	}
	if !hasTransition(available, "translate") {
		t.Fatalf("expected translate transition for review state, got %+v", available)
	}

	if len(fixture.Steps) == 0 {
		t.Fatalf("fixture must include at least one workflow step")
	}

	for idx, step := range fixture.Steps {
		updated, err := pageSvc.Update(ctx, pages.UpdatePageRequest{
			ID:        page.ID,
			Status:    step.TargetState,
			UpdatedBy: authorID,
			Translations: append([]pages.PageTranslationInput(nil),
				translations...),
		})
		if err != nil {
			t.Fatalf("step %d transition to %q failed: %v", idx, step.TargetState, err)
		}
		if !strings.EqualFold(updated.Status, step.TargetState) {
			t.Fatalf("step %d expected status %q, got %q", idx, step.TargetState, updated.Status)
		}
		page = updated
	}

	finalState := fixture.Steps[len(fixture.Steps)-1].TargetState
	if !strings.EqualFold(page.Status, finalState) {
		t.Fatalf("expected final status %q, got %q", finalState, page.Status)
	}
}

func convertFixtureStates(states []workflowStateFixture) []cms.WorkflowStateConfig {
	result := make([]cms.WorkflowStateConfig, len(states))
	for i, state := range states {
		result[i] = cms.WorkflowStateConfig{
			Name:        state.Name,
			Description: state.Description,
			Terminal:    state.Terminal,
			Initial:     state.Initial,
		}
	}
	return result
}

func convertFixtureTransitions(transitions []workflowTransitionFixture) []cms.WorkflowTransitionConfig {
	result := make([]cms.WorkflowTransitionConfig, len(transitions))
	for i, transition := range transitions {
		result[i] = cms.WorkflowTransitionConfig{
			Name:        transition.Name,
			Description: transition.Description,
			From:        transition.From,
			To:          transition.To,
			Guard:       transition.Guard,
		}
	}
	return result
}

func deriveInitialState(states []workflowStateFixture) string {
	if len(states) == 0 {
		return "draft"
	}
	for _, state := range states {
		if state.Initial {
			return state.Name
		}
	}
	return states[0].Name
}

func hasTransition(transitions []interfaces.WorkflowTransition, name string) bool {
	for _, transition := range transitions {
		if strings.EqualFold(transition.Name, name) {
			return true
		}
	}
	return false
}
