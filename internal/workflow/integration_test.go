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
	seedTypes, ok := typeRepo.(interface{ Put(*content.ContentType) error })
	if !ok {
		t.Fatalf("expected seedable content type repository, got %T", typeRepo)
	}
	contentTypeID := uuid.New()
	if err := seedTypes.Put(&content.ContentType{ID: contentTypeID, Name: "integration", Slug: "integration"}); err != nil {
		t.Fatalf("seed content type: %v", err)
	}

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

func TestWorkflowIntegration_StatusTransitionsWithoutTranslations(t *testing.T) {
	ctx := context.Background()

	baseStates := []cms.WorkflowStateConfig{
		{Name: "draft", Description: "Draft", Initial: true},
		{Name: "published", Description: "Published", Terminal: true},
	}
	baseTransitions := []cms.WorkflowTransitionConfig{
		{Name: "publish", Description: "Publish draft", From: "draft", To: "published"},
	}

	testCases := []struct {
		name         string
		configure    func(cfg *cms.Config)
		allowMissing bool
	}{
		{
			name: "per_request_override",
			configure: func(cfg *cms.Config) {
				cfg.I18N.RequireTranslations = true
			},
			allowMissing: true,
		},
		{
			name: "global_optional",
			configure: func(cfg *cms.Config) {
				cfg.I18N.RequireTranslations = false
			},
			allowMissing: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := cms.DefaultConfig()
			cfg.DefaultLocale = "en"
			cfg.I18N.Locales = []string{"en"}
			cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{
				{
					Entity:      "page",
					States:      baseStates,
					Transitions: baseTransitions,
				},
			}
			if tc.configure != nil {
				tc.configure(&cfg)
			}

			module, err := cms.New(cfg)
			if err != nil {
				t.Fatalf("new cms module: %v", err)
			}

			typeRepo := module.Container().ContentTypeRepository()
			seedTypes, ok := typeRepo.(interface{ Put(*content.ContentType) error })
			if !ok {
				t.Fatalf("expected seedable content type repository, got %T", typeRepo)
			}
			contentTypeID := uuid.New()
			if err := seedTypes.Put(&content.ContentType{ID: contentTypeID, Name: "integration", Slug: "integration"}); err != nil {
				t.Fatalf("seed content type: %v", err)
			}

			localeRepo := module.Container().LocaleRepository()
			seedLocales, ok := localeRepo.(interface{ Put(*content.Locale) })
			if !ok {
				t.Fatalf("expected seedable locale repository, got %T", localeRepo)
			}
			localeID := uuid.New()
			seedLocales.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

			contentSvc := module.Content()
			authorID := uuid.New()
			contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
				ContentTypeID: contentTypeID,
				Slug:          "workflow-no-i18n",
				Status:        "draft",
				CreatedBy:     authorID,
				UpdatedBy:     authorID,
				Translations: []content.ContentTranslationInput{
					{Locale: "en", Title: "Workflow Without Translations"},
				},
			})
			if err != nil {
				t.Fatalf("create content: %v", err)
			}

			pageSvc := module.Pages()
			page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
				ContentID:  contentRecord.ID,
				TemplateID: uuid.New(),
				Slug:       "workflow-no-i18n",
				Status:     "draft",
				CreatedBy:  authorID,
				UpdatedBy:  authorID,
				Translations: []pages.PageTranslationInput{
					{Locale: "en", Title: "Workflow Without Translations", Path: "/workflow-no-i18n"},
				},
			})
			if err != nil {
				t.Fatalf("create page: %v", err)
			}

			engine := module.Container().WorkflowEngine()
			if engine == nil {
				t.Fatalf("expected workflow engine to be configured")
			}

			result, err := engine.Transition(ctx, interfaces.TransitionInput{
				EntityID:     page.ID,
				EntityType:   "page",
				CurrentState: interfaces.WorkflowState(page.Status),
				Transition:   "publish",
				ActorID:      authorID,
			})
			if err != nil {
				t.Fatalf("transition draft->publish: %v", err)
			}

			updated, err := pageSvc.Update(ctx, pages.UpdatePageRequest{
				ID:                       page.ID,
				Status:                   string(result.ToState),
				UpdatedBy:                authorID,
				AllowMissingTranslations: tc.allowMissing,
			})
			if err != nil {
				t.Fatalf("update page without translations (allow=%v): %v", tc.allowMissing, err)
			}

			if !strings.EqualFold(updated.Status, string(result.ToState)) {
				t.Fatalf("expected status %q, got %q", result.ToState, updated.Status)
			}
			if len(updated.Translations) == 0 {
				t.Fatalf("expected existing translations to be preserved")
			}
			if updated.Translations[0].LocaleID != localeID {
				t.Fatalf("expected translation locale %s, got %s", localeID, updated.Translations[0].LocaleID)
			}
		})
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
