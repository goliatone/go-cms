package workflow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

var (
	// ErrDefinitionEntityRequired indicates the workflow definition lacks an entity identifier.
	ErrDefinitionEntityRequired = errors.New("workflow: definition entity required")
	// ErrDefinitionStatesRequired indicates the workflow definition does not declare any states.
	ErrDefinitionStatesRequired = errors.New("workflow: definition requires at least one state")
	// ErrStateNameRequired indicates a workflow state is missing its name.
	ErrStateNameRequired = errors.New("workflow: state name required")
	// ErrDuplicateState indicates duplicate workflow state names were declared.
	ErrDuplicateState = errors.New("workflow: duplicate state")
	// ErrDuplicateDefinition indicates multiple definitions were provided for the same entity.
	ErrDuplicateDefinition = errors.New("workflow: duplicate entity definition")
	// ErrTransitionNameRequired indicates a transition lacks a name.
	ErrTransitionNameRequired = errors.New("workflow: transition name required")
	// ErrTransitionStateUnknown indicates a transition references a state that was not declared.
	ErrTransitionStateUnknown = errors.New("workflow: transition references unknown state")
	// ErrDuplicateTransition indicates the same transition name is declared multiple times for a state.
	ErrDuplicateTransition = errors.New("workflow: duplicate transition for state")
	// ErrInitialStateInvalid indicates the supplied initial state flag is inconsistent or unknown.
	ErrInitialStateInvalid = errors.New("workflow: invalid initial state")
)

// CompileDefinitionConfigs converts configuration-driven workflow definitions into runtime definitions.
// Validation is applied to ensure state and transition integrity before registration.
func CompileDefinitionConfigs(configs []runtimeconfig.WorkflowDefinitionConfig) ([]interfaces.WorkflowDefinition, error) {
	if len(configs) == 0 {
		return nil, nil
	}

	definitions := make([]interfaces.WorkflowDefinition, 0, len(configs))
	seenEntities := make(map[string]struct{}, len(configs))

	for _, cfg := range configs {
		definition, err := compileDefinitionConfig(cfg)
		if err != nil {
			return nil, err
		}

		entityKey := strings.ToLower(strings.TrimSpace(definition.EntityType))
		if _, exists := seenEntities[entityKey]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateDefinition, definition.EntityType)
		}
		seenEntities[entityKey] = struct{}{}
		definitions = append(definitions, definition)
	}

	return definitions, nil
}

func compileDefinitionConfig(cfg runtimeconfig.WorkflowDefinitionConfig) (interfaces.WorkflowDefinition, error) {
	entity := strings.TrimSpace(cfg.Entity)
	if entity == "" {
		return interfaces.WorkflowDefinition{}, ErrDefinitionEntityRequired
	}

	if len(cfg.States) == 0 {
		return interfaces.WorkflowDefinition{}, fmt.Errorf("%w: %s", ErrDefinitionStatesRequired, entity)
	}

	stateMap, stateDefs, initialState, err := compileStates(cfg.States)
	if err != nil {
		return interfaces.WorkflowDefinition{}, err
	}

	transitions, err := compileTransitions(cfg.Transitions, stateMap)
	if err != nil {
		return interfaces.WorkflowDefinition{}, err
	}

	return interfaces.WorkflowDefinition{
		EntityType:   strings.ToLower(entity),
		InitialState: interfaces.WorkflowState(initialState),
		States:       stateDefs,
		Transitions:  transitions,
	}, nil
}

type compiledState struct {
	name        interfaces.WorkflowState
	description string
	terminal    bool
}

func compileStates(configs []runtimeconfig.WorkflowStateConfig) (map[string]compiledState, []interfaces.WorkflowStateDefinition, string, error) {
	result := make(map[string]compiledState, len(configs))
	ordered := make([]interfaces.WorkflowStateDefinition, 0, len(configs))
	var initial interfaces.WorkflowState
	var initialDeclared bool

	for idx, cfg := range configs {
		name := strings.TrimSpace(cfg.Name)
		if name == "" {
			return nil, nil, "", fmt.Errorf("%w at index %d", ErrStateNameRequired, idx)
		}
		normalized := interfaces.WorkflowState(domain.NormalizeWorkflowState(name))
		key := string(normalized)
		if _, exists := result[key]; exists {
			return nil, nil, "", fmt.Errorf("%w: %s", ErrDuplicateState, key)
		}
		if cfg.Initial {
			if initialDeclared {
				return nil, nil, "", ErrInitialStateInvalid
			}
			initial = normalized
			initialDeclared = true
		}
		result[key] = compiledState{
			name:        normalized,
			description: strings.TrimSpace(cfg.Description),
			terminal:    cfg.Terminal,
		}
		ordered = append(ordered, interfaces.WorkflowStateDefinition{
			Name:        normalized,
			Description: strings.TrimSpace(cfg.Description),
			Terminal:    cfg.Terminal,
		})
	}

	if !initialDeclared {
		first := configs[0]
		firstName := interfaces.WorkflowState(domain.NormalizeWorkflowState(strings.TrimSpace(first.Name)))
		initial = firstName
	}

	if _, ok := result[string(initial)]; !ok {
		return nil, nil, "", fmt.Errorf("%w: %s", ErrInitialStateInvalid, initial)
	}

	return result, ordered, string(initial), nil
}

func compileTransitions(configs []runtimeconfig.WorkflowTransitionConfig, states map[string]compiledState) ([]interfaces.WorkflowTransition, error) {
	if len(configs) == 0 {
		return nil, nil
	}

	result := make([]interfaces.WorkflowTransition, 0, len(configs))
	seen := make(map[string]struct{}, len(configs))

	for idx, cfg := range configs {
		name := strings.TrimSpace(cfg.Name)
		if name == "" {
			return nil, fmt.Errorf("%w at index %d", ErrTransitionNameRequired, idx)
		}

		fromRaw := strings.TrimSpace(cfg.From)
		toRaw := strings.TrimSpace(cfg.To)
		if fromRaw == "" || toRaw == "" {
			return nil, fmt.Errorf("%w: %s -> %s", ErrTransitionStateUnknown, cfg.From, cfg.To)
		}

		from := interfaces.WorkflowState(domain.NormalizeWorkflowState(fromRaw))
		to := interfaces.WorkflowState(domain.NormalizeWorkflowState(toRaw))
		if _, ok := states[string(from)]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrTransitionStateUnknown, from)
		}
		if _, ok := states[string(to)]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrTransitionStateUnknown, to)
		}

		key := transitionKey(name, from)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("%w: %s from %s", ErrDuplicateTransition, name, from)
		}
		seen[key] = struct{}{}

		result = append(result, interfaces.WorkflowTransition{
			Name:        name,
			Description: strings.TrimSpace(cfg.Description),
			From:        from,
			To:          to,
			Guard:       strings.TrimSpace(cfg.Guard),
		})
	}

	return result, nil
}

func transitionKey(name string, from interfaces.WorkflowState) string {
	return strings.ToLower(strings.TrimSpace(name)) + "::" + string(from)
}
