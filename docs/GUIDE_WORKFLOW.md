# Workflow Guide

This guide covers content lifecycle orchestration using the workflow engine in `go-cms`. The workflow module provides state machine management for domain entities such as content entries (pages/posts) and legacy pages, enabling editorial review flows, scheduled publishing, and custom lifecycle transitions.

## Workflow Architecture Overview

The workflow engine is a state machine that governs how entities move through lifecycle stages. It is composed of three layers:

1. **Configuration** -- workflow definitions declared in `cms.Config` describe states, transitions, and optional guard expressions.
2. **Compilation** -- configuration definitions are compiled and validated at container startup, catching misconfigurations early.
3. **Execution** -- the engine evaluates transitions at runtime, enforcing allowed state changes, guard expressions, and terminal states.

The engine is entity-agnostic: while `go-cms` ships a default page workflow, you can register workflows for any entity type. The engine does not persist state -- it validates transitions and returns results. The caller (a service or your application) is responsible for persisting the resulting status.

```
                +-----------------+
                |   Config Layer  |   WorkflowDefinitionConfig
                +-----------------+
                        |
                   compile & validate
                        |
                +-----------------+
                |  Engine Layer   |   WorkflowEngine interface
                +-----------------+
                        |
                   Transition()
                        |
                +-----------------+
                |  Service Layer  |   apply workflow, persist status
                +-----------------+
                        |
                   persist status
                        |
                +-----------------+
                |   Repository    |   SQL / Memory
                +-----------------+
```

## Configuration

### Enabling the Workflow Engine

Workflow is **enabled by default** with the `"simple"` in-memory provider:

```go
cfg := cms.DefaultConfig()
// cfg.Workflow.Enabled = true    (default)
// cfg.Workflow.Provider = "simple" (default)
```

To disable workflows entirely:

```go
cfg.Workflow.Enabled = false
```

When disabled, `container.WorkflowEngine()` returns `nil` and callers fall back to using the requested status directly without validation (legacy pages service behaves this way as well).

### Provider Options

The `Provider` field selects the workflow engine implementation:

| Provider   | Description                                                 |
|------------|-------------------------------------------------------------|
| `"simple"` | Default in-memory engine with deterministic transitions     |
| `"custom"` | Requires a user-provided engine via `di.WithWorkflowEngine()` |

An unrecognised provider value falls back to `"simple"`.

### Defining Workflows

Workflows are defined in configuration as state machines with explicit states and transitions:

```go
cfg := cms.DefaultConfig()
cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{
    {
        Entity:      "page",
        Description: "Page publishing workflow",
        States: []cms.WorkflowStateConfig{
            {Name: "draft", Description: "Draft content", Initial: true},
            {Name: "review", Description: "Under review"},
            {Name: "approved", Description: "Approved"},
            {Name: "published", Description: "Published"},
            {Name: "archived", Description: "Archived", Terminal: true},
        },
        Transitions: []cms.WorkflowTransitionConfig{
            {Name: "submit_review", From: "draft", To: "review"},
            {Name: "approve", From: "review", To: "approved"},
            {Name: "reject", From: "review", To: "draft"},
            {Name: "publish", From: "approved", To: "published"},
            {Name: "unpublish", From: "published", To: "draft"},
            {Name: "archive", From: "published", To: "archived"},
        },
    },
}
```

### Content Entry Workflows (Pages/Posts)

Pages and posts are modeled as content entries. In admin integrations (go-admin), workflows are selected via content type capabilities (for example, `workflow: "pages"` or `workflow: "posts"`). The admin layer applies transitions and writes the resulting status to content entries. The workflow engine remains available for legacy page service usage or for custom application logic that wants to enforce transitions before persisting content status.

### Configuration Types

**`WorkflowConfig`** -- top-level workflow settings:

```go
type WorkflowConfig struct {
    Enabled     bool                       // Enable/disable the workflow engine
    Provider    string                     // "simple" or "custom"
    Definitions []WorkflowDefinitionConfig // State machine definitions
}
```

**`WorkflowDefinitionConfig`** -- a single state machine definition:

```go
type WorkflowDefinitionConfig struct {
    Entity      string                     // Entity type (e.g. "page")
    Description string                     // Optional description
    States      []WorkflowStateConfig      // At least one state required
    Transitions []WorkflowTransitionConfig // Transitions between states
}
```

**`WorkflowStateConfig`** -- a state within a workflow:

```go
type WorkflowStateConfig struct {
    Name        string // Unique name (normalized to lowercase)
    Description string // Optional description
    Terminal    bool   // If true, no transitions allowed FROM this state
    Initial     bool   // Starting state (only one per definition)
}
```

**`WorkflowTransitionConfig`** -- an allowed transition between two states:

```go
type WorkflowTransitionConfig struct {
    Name        string // Transition name (unique per source state)
    Description string // Optional description
    From        string // Source state
    To          string // Destination state
    Guard       string // Optional guard expression
}
```

### Validation Rules

Definitions are compiled and validated during container startup. The compiler enforces:

- **Entity required** -- every definition must specify an entity type.
- **States required** -- at least one state must be declared.
- **Unique state names** -- duplicate state names within a definition are rejected.
- **Single initial state** -- at most one state may be marked `Initial: true`. If none is marked, the first declared state becomes the initial state.
- **Valid transition endpoints** -- both `From` and `To` must reference declared states.
- **Unique transitions** -- the same transition name cannot appear twice for the same source state.
- **Terminal state enforcement** -- transitions originating from a terminal state are rejected at compile time.
- **No duplicate entities** -- two definitions for the same entity type in the same config are rejected.

## Workflow States and Transitions

### Built-in Domain States

The `domain` package defines standard workflow states that map to persisted `Status` values:

| Workflow State | Status      | Description                       |
|----------------|-------------|-----------------------------------|
| `draft`        | `draft`     | Initial state for new content     |
| `published`    | `published` | Live and visible                  |
| `archived`     | `archived`  | Retained but not visible          |
| `scheduled`    | `scheduled` | Future publication                |
| `review`       | `review`    | Awaiting editorial review         |
| `approved`     | `approved`  | Review approved, ready to publish |
| `rejected`     | `rejected`  | Changes requested                 |
| `translated`   | `translated`| Localization complete             |

State names are normalized to lowercase. Custom states (beyond the built-in set) are supported -- any lowercase string is a valid state name.

### Status-to-State Mapping

The `domain` package provides bidirectional mapping between persisted `Status` values and workflow states:

```go
// Status -> WorkflowState
state := domain.WorkflowStateFromStatus(domain.StatusDraft)  // "draft"

// WorkflowState -> Status
status := domain.StatusFromWorkflowState(domain.WorkflowStateDraft) // "draft"
```

Custom states that don't match a built-in mapping pass through as-is.

### Default Page Workflow

When no custom workflow definitions are provided, the simple engine seeds a default page workflow:

```
States:
  draft (initial) -- Draft content awaiting validation
  review          -- Under editorial review
  approved        -- Approved and ready to publish
  scheduled       -- Scheduled to publish at a future time
  published       -- Published and visible
  archived        -- Archived and hidden

Transitions:
  submit_review:   draft     -> review
  approve:         review    -> approved
  reject:          review    -> draft
  request_changes: approved  -> review
  publish:         approved  -> published
  publish:         draft     -> published
  publish:         scheduled -> published
  unpublish:       published -> draft
  archive:         draft     -> archived
  archive:         published -> archived
  restore:         archived  -> draft
  schedule:        draft     -> scheduled
  schedule:        approved  -> scheduled
  cancel_schedule: scheduled -> draft
```

Note that a transition name can appear multiple times with different source states. For example, `"publish"` is valid from `draft`, `approved`, and `scheduled`.

## WorkflowEngine Interface

The core engine contract lives in `pkg/interfaces/workflow.go`:

```go
type WorkflowEngine interface {
    // Transition applies a named transition or explicit state change.
    Transition(ctx context.Context, input TransitionInput) (*TransitionResult, error)

    // AvailableTransitions lists possible transitions from a state.
    AvailableTransitions(ctx context.Context, query TransitionQuery) ([]WorkflowTransition, error)

    // RegisterWorkflow installs or replaces a workflow definition.
    RegisterWorkflow(ctx context.Context, definition WorkflowDefinition) error
}
```

### TransitionInput

```go
type TransitionInput struct {
    EntityID     uuid.UUID     // Required: entity being transitioned
    EntityType   string        // Required: "page" or custom entity type
    CurrentState WorkflowState // Current state (defaults to initial state if empty)
    Transition   string        // Transition name (option A)
    TargetState  WorkflowState // Target state (option B)
    ActorID      uuid.UUID     // Actor performing the transition
    Metadata     map[string]any // Arbitrary context
}
```

You can trigger a transition in two ways:

- **By name** -- set `Transition` to a transition name (e.g. `"publish"`). The engine looks up the transition from the current state.
- **By target state** -- set `TargetState` to the desired state (e.g. `"published"`). The engine finds the first matching transition from the current state to the target.

If both are empty and the current state equals the target, the engine returns a no-op result.

### TransitionResult

```go
type TransitionResult struct {
    EntityID      uuid.UUID
    EntityType    string
    Transition    string              // Transition name that was executed
    FromState     WorkflowState       // Starting state
    ToState       WorkflowState       // Ending state
    CompletedAt   time.Time           // UTC timestamp
    ActorID       uuid.UUID
    Metadata      map[string]any
    Events        []WorkflowEvent     // Emitted domain events
    Notifications []WorkflowNotification // Notification requests
}
```

### TransitionQuery

```go
type TransitionQuery struct {
    EntityType string
    State      WorkflowState
    Context    map[string]any
}
```

### WorkflowDefinition

```go
type WorkflowDefinition struct {
    EntityType   string
    InitialState WorkflowState
    States       []WorkflowStateDefinition
    Transitions  []WorkflowTransition
}
```

### WorkflowAuthorizer

Guards are evaluated by an optional authorizer:

```go
type WorkflowAuthorizer interface {
    AuthorizeTransition(ctx context.Context, input TransitionInput, guard string) error
}
```

Return `nil` to authorise the transition, or an error to block it.

## Simple Engine (Default)

The simple engine (`internal/workflow/simple/engine.go`) is an in-memory implementation with these characteristics:

- Thread-safe via `sync.RWMutex`
- Deterministic state transitions
- Guard support via an optional `WorkflowAuthorizer`
- Seeds the default page workflow on construction

### Construction

```go
import workflowsimple "github.com/goliatone/go-cms/internal/workflow/simple"

engine := workflowsimple.New(
    workflowsimple.WithClock(func() time.Time { return fixedTime }), // testing
    workflowsimple.WithAuthorizer(myAuthorizer),                     // guard enforcement
)
```

### Executing a Transition

```go
result, err := engine.Transition(ctx, interfaces.TransitionInput{
    EntityID:     pageID,
    EntityType:   "page",
    CurrentState: interfaces.WorkflowState("draft"),
    Transition:   "submit_review",
    ActorID:      userID,
    Metadata: map[string]any{
        "reason": "Ready for review",
    },
})
// result.FromState == "draft"
// result.ToState   == "review"
// result.Transition == "submit_review"
```

### Querying Available Transitions

```go
transitions, err := engine.AvailableTransitions(ctx, interfaces.TransitionQuery{
    EntityType: "page",
    State:      interfaces.WorkflowState("draft"),
})
// Returns: submit_review, publish, schedule, archive
```

### Registering a Custom Workflow

```go
err := engine.RegisterWorkflow(ctx, interfaces.WorkflowDefinition{
    EntityType:   "article",
    InitialState: "draft",
    States: []interfaces.WorkflowStateDefinition{
        {Name: "draft", Description: "New article"},
        {Name: "published"},
        {Name: "retired", Terminal: true},
    },
    Transitions: []interfaces.WorkflowTransition{
        {Name: "publish", From: "draft", To: "published"},
        {Name: "retire", From: "published", To: "retired"},
    },
})
```

## Custom Engine via Adapter

The adapter module (`internal/workflow/adapter/`) wraps an external workflow engine to integrate it with `go-cms`. The adapter adds:

- Input/output normalization
- Guard authorization
- Action resolution (side effects triggered by transitions)
- Event and notification aggregation

### Construction

```go
import "github.com/goliatone/go-cms/internal/workflow/adapter"

backingEngine := myCustomEngine() // implements workflowEngine interface

adapted, err := adapter.NewEngine(backingEngine,
    adapter.WithAuthorizer(authService),
    adapter.WithActionRegistry(adapter.ActionRegistry{
        "page::publish": func(ctx context.Context, input adapter.ActionInput) (adapter.ActionOutput, error) {
            // Side effect: send notification, update cache, etc.
            return adapter.ActionOutput{
                Events: []interfaces.WorkflowEvent{{
                    Name:    "page.published",
                    Payload: map[string]any{"slug": input.Transition.Metadata["slug"]},
                }},
            }, nil
        },
    }),
    adapter.WithClock(time.Now),
)
```

### Action Types

**`Action`** -- a function executed as a side effect of a transition:

```go
type Action func(ctx context.Context, input ActionInput) (ActionOutput, error)
```

**`ActionInput`** -- context passed to the action:

```go
type ActionInput struct {
    Transition interfaces.TransitionInput
    Result     *interfaces.TransitionResult
}
```

**`ActionOutput`** -- side effects produced by the action:

```go
type ActionOutput struct {
    Events        []interfaces.WorkflowEvent
    Notifications []interfaces.WorkflowNotification
    Metadata      map[string]any
}
```

**`ActionRegistry`** -- a map-backed action resolver keyed by `"entityType::transitionName"`:

```go
type ActionRegistry map[string]Action
```

The registry supports two lookup strategies:
1. Entity-scoped: `"page::publish"` -- matches only page publish transitions.
2. Global: `"::publish"` -- matches publish transitions for any entity type.

### ActionResolver

For dynamic action resolution, implement the `ActionResolver` function type:

```go
type ActionResolver func(entityType string, transition interfaces.WorkflowTransition) (Action, bool)
```

Wire it with `adapter.WithActionResolver(resolver)`.

## Dependency Injection

### Wiring the Workflow Engine

The DI container (`di.Container`) configures the workflow engine during initialization:

```go
cfg := cms.DefaultConfig()
cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{...}

// Option A: Use the default simple engine
module, err := cms.New(cfg)

// Option B: Provide a custom engine
cfg.Workflow.Provider = "custom"
module, err := cms.New(cfg,
    di.WithWorkflowEngine(myEngine),
)
```

### DI Options

**`di.WithWorkflowEngine(engine)`** -- override the workflow engine. Required when `Provider` is `"custom"`:

```go
di.WithWorkflowEngine(engine interfaces.WorkflowEngine)
```

**`di.WithWorkflowDefinitionStore(store)`** -- register an external source for workflow definitions. Definitions from the store are loaded and registered alongside config-based definitions:

```go
di.WithWorkflowDefinitionStore(store interfaces.WorkflowDefinitionStore)
```

The `WorkflowDefinitionStore` interface:

```go
type WorkflowDefinitionStore interface {
    ListWorkflowDefinitions(ctx context.Context) ([]WorkflowDefinition, error)
}
```

### Container Initialization Sequence

1. Check `cfg.Workflow.Enabled` -- if `false`, set engine to `nil` and return.
2. Resolve provider -- `"simple"` creates `workflowsimple.New()`, `"custom"` requires a pre-wired engine.
3. Compile config definitions -- `workflow.CompileDefinitionConfigs()` validates and converts config definitions.
4. Load store definitions -- if a `WorkflowDefinitionStore` is provided, its definitions are appended.
5. Register all definitions -- each definition is registered with `engine.RegisterWorkflow()`.

### Accessing the Engine

Via the module facade:

```go
engine := module.WorkflowEngine()
```

Via the container directly:

```go
engine := module.Container().WorkflowEngine()
```

Both return `nil` when workflow is disabled.

## Integration with Page Status Transitions (Legacy)

The legacy pages service automatically applies workflow transitions when pages are created or updated. For content entries, workflows are typically applied by the caller (for example, go-admin) before setting the entry status.

### How It Works

When a page status change is requested (via `Create`, `Update`, or other operations), the legacy pages service:

1. Determines the current workflow state from the page's persisted status.
2. Builds a `PageContext` with metadata about the page (ID, slug, version, timestamps).
3. Calls `engine.Transition()` with the current state and desired target.
4. Persists the resulting status from the engine's response.
5. Logs any emitted workflow events.

If no workflow engine is configured, the service uses the requested status directly.

### PageContext

The `workflow.PageContext` captures page metadata that is included in transition metadata payloads:

```go
type PageContext struct {
    ID               uuid.UUID
    ContentID        uuid.UUID
    TemplateID       uuid.UUID
    ParentID         *uuid.UUID
    Slug             string
    Status           domain.Status
    WorkflowState    domain.WorkflowState
    CurrentVersion   int
    PublishedVersion *int
    PublishAt        *time.Time
    UnpublishAt      *time.Time
    CreatedBy        uuid.UUID
    UpdatedBy        uuid.UUID
}
```

The `Metadata()` method converts the context to a `map[string]any` suitable for workflow metadata, including fields like `page_id`, `slug`, `status`, `workflow_state`, `current_version`, and timestamps.

### Workflow Events

When a transition produces events, the legacy pages service logs them with structured fields including:

- `page_id`, `page_slug` -- the affected page
- `workflow_from`, `workflow_to` -- state transition
- `workflow_transition` -- transition name
- `workflow_event` -- event name from the result
- `actor_id` -- who triggered the transition

## Guarded Transitions

Guards are optional expressions attached to transitions that must be authorized before the transition proceeds.

### Defining Guards

```go
cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{
    {
        Entity: "page",
        States: []cms.WorkflowStateConfig{
            {Name: "draft", Initial: true},
            {Name: "review"},
            {Name: "published"},
        },
        Transitions: []cms.WorkflowTransitionConfig{
            {Name: "submit_review", From: "draft", To: "review"},
            {Name: "publish", From: "review", To: "published", Guard: "role == editor"},
        },
    },
}
```

### Implementing an Authorizer

```go
type RoleAuthorizer struct {
    roles map[uuid.UUID]string // actor ID -> role
}

func (a *RoleAuthorizer) AuthorizeTransition(
    ctx context.Context,
    input interfaces.TransitionInput,
    guard string,
) error {
    if guard == "role == editor" {
        role, ok := a.roles[input.ActorID]
        if !ok || role != "editor" {
            return fmt.Errorf("actor %s does not have editor role", input.ActorID)
        }
    }
    return nil
}
```

### Wiring the Authorizer

For the simple engine:

```go
engine := workflowsimple.New(
    workflowsimple.WithAuthorizer(&RoleAuthorizer{roles: roleMap}),
)
module, err := cms.New(cfg, di.WithWorkflowEngine(engine))
```

For the adapter engine:

```go
adapted, err := adapter.NewEngine(backingEngine,
    adapter.WithAuthorizer(&RoleAuthorizer{roles: roleMap}),
)
module, err := cms.New(cfg, di.WithWorkflowEngine(adapted))
```

If a transition has a guard but no authorizer is configured, the engine returns `ErrGuardAuthorizerRequired`.

## Running Workflow Tests

```bash
# Run all workflow tests
./taskfile workflow:test

# Run simple engine tests
go test ./internal/workflow/simple/...

# Run adapter engine tests
go test ./internal/workflow/adapter/...

# Run definition compilation tests
go test ./internal/workflow/... -run TestCompile

# Run integration tests
go test ./internal/workflow/... -run TestWorkflowIntegration

# Run with a custom engine (requires running engine server)
CMS_WORKFLOW_PROVIDER=custom CMS_WORKFLOW_ENGINE_ADDR=http://localhost:8080 \
    go test ./internal/workflow/... ./internal/integration/...
```

### Integration Test Example

The integration tests demonstrate a full multi-step page lifecycle with a custom workflow:

```go
func TestWorkflowIntegration_MultiStepPageLifecycle(t *testing.T) {
    ctx := context.Background()

    cfg := cms.DefaultConfig()
    cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{
        {
            Entity: "page",
            States: []cms.WorkflowStateConfig{
                {Name: "draft", Initial: true},
                {Name: "review"},
                {Name: "published", Terminal: true},
            },
            Transitions: []cms.WorkflowTransitionConfig{
                {Name: "submit_review", From: "draft", To: "review"},
                {Name: "publish", From: "review", To: "published"},
            },
        },
    }

    module, err := cms.New(cfg)
    if err != nil {
        t.Fatal(err)
    }

    engine := module.Container().WorkflowEngine()

    // Query available transitions from draft
    transitions, _ := engine.AvailableTransitions(ctx, interfaces.TransitionQuery{
        EntityType: "page",
        State:      interfaces.WorkflowState("draft"),
    })
    // transitions contains: submit_review

    // Execute transition
    result, _ := engine.Transition(ctx, interfaces.TransitionInput{
        EntityID:     pageID,
        EntityType:   "page",
        CurrentState: interfaces.WorkflowState("draft"),
        Transition:   "submit_review",
        ActorID:      authorID,
    })
    // result.ToState == "review"
}
```

## Error Handling

### Compilation Errors

Returned during container startup when definitions are invalid:

| Error                        | Cause                                           |
|------------------------------|--------------------------------------------------|
| `ErrDefinitionEntityRequired`  | Definition missing entity type                  |
| `ErrDefinitionStatesRequired`  | No states declared                              |
| `ErrStateNameRequired`         | State has empty name                            |
| `ErrDuplicateState`            | Two states share the same name                  |
| `ErrDuplicateDefinition`       | Two definitions for the same entity             |
| `ErrTransitionNameRequired`    | Transition missing name                         |
| `ErrTransitionStateUnknown`    | Transition references an undeclared state        |
| `ErrDuplicateTransition`       | Same transition name repeated for a source state |
| `ErrInitialStateInvalid`       | Multiple initial states or unknown initial state |

### Runtime Errors

Returned during `Transition()` calls:

| Error                       | Cause                                              |
|-----------------------------|----------------------------------------------------|
| `ErrUnknownEntityType`        | No definition registered for the entity type       |
| `ErrInvalidTransition`        | Transition not allowed from the current state       |
| `ErrMissingTransition`        | Neither transition name nor target state provided   |
| `ErrNilEntityID`              | Entity ID is `uuid.Nil`                            |
| `ErrTerminalState`            | Cannot transition from a terminal state             |
| `ErrGuardAuthorizerRequired`  | Guard present but no authorizer configured          |
| `ErrGuardRejected`            | Authorizer blocked the transition (adapter only)    |

## Common Patterns

### Simple Publish Workflow

For sites where content goes directly from draft to published:

```go
cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{
    {
        Entity: "page",
        States: []cms.WorkflowStateConfig{
            {Name: "draft", Initial: true},
            {Name: "published"},
            {Name: "archived", Terminal: true},
        },
        Transitions: []cms.WorkflowTransitionConfig{
            {Name: "publish", From: "draft", To: "published"},
            {Name: "unpublish", From: "published", To: "draft"},
            {Name: "archive", From: "draft", To: "archived"},
            {Name: "archive", From: "published", To: "archived"},
        },
    },
}
```

### Multi-Step Editorial Workflow

For teams that require editorial review before publishing:

```go
cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{
    {
        Entity: "page",
        States: []cms.WorkflowStateConfig{
            {Name: "draft", Initial: true},
            {Name: "review"},
            {Name: "approved"},
            {Name: "published"},
            {Name: "archived", Terminal: true},
        },
        Transitions: []cms.WorkflowTransitionConfig{
            {Name: "submit_review", From: "draft", To: "review"},
            {Name: "approve", From: "review", To: "approved"},
            {Name: "reject", From: "review", To: "draft"},
            {Name: "request_changes", From: "approved", To: "review"},
            {Name: "publish", From: "approved", To: "published"},
            {Name: "unpublish", From: "published", To: "draft"},
            {Name: "archive", From: "published", To: "archived"},
        },
    },
}
```

### Scheduled Publishing

Combine workflow transitions with scheduling states:

```go
cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{
    {
        Entity: "page",
        States: []cms.WorkflowStateConfig{
            {Name: "draft", Initial: true},
            {Name: "approved"},
            {Name: "scheduled"},
            {Name: "published"},
        },
        Transitions: []cms.WorkflowTransitionConfig{
            {Name: "approve", From: "draft", To: "approved"},
            {Name: "schedule", From: "approved", To: "scheduled"},
            {Name: "schedule", From: "draft", To: "scheduled"},
            {Name: "publish", From: "scheduled", To: "published"},
            {Name: "cancel_schedule", From: "scheduled", To: "draft"},
            {Name: "unpublish", From: "published", To: "draft"},
        },
    },
}
```

When the scheduling feature is enabled (`cfg.Features.Scheduling = true`), the scheduler polls for pages in the `scheduled` state and transitions them to `published` when their `publishAt` timestamp is reached.

### Custom Entity Types

Register workflows for entity types beyond pages:

```go
cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{
    {
        Entity: "page",
        States: []cms.WorkflowStateConfig{...},
        Transitions: []cms.WorkflowTransitionConfig{...},
    },
    {
        Entity: "product",
        States: []cms.WorkflowStateConfig{
            {Name: "draft", Initial: true},
            {Name: "active"},
            {Name: "discontinued", Terminal: true},
        },
        Transitions: []cms.WorkflowTransitionConfig{
            {Name: "activate", From: "draft", To: "active"},
            {Name: "discontinue", From: "active", To: "discontinued"},
        },
    },
}
```

Custom entity workflows are accessed through the engine directly:

```go
engine := module.WorkflowEngine()
result, err := engine.Transition(ctx, interfaces.TransitionInput{
    EntityID:   productID,
    EntityType: "product",
    Transition: "activate",
    ActorID:    userID,
})
```

### External Workflow Definitions

Load workflow definitions from a database or external service using `WorkflowDefinitionStore`:

```go
type DBWorkflowStore struct {
    db *bun.DB
}

func (s *DBWorkflowStore) ListWorkflowDefinitions(ctx context.Context) ([]interfaces.WorkflowDefinition, error) {
    // Load definitions from database
    var definitions []interfaces.WorkflowDefinition
    // ...
    return definitions, nil
}

module, err := cms.New(cfg,
    di.WithWorkflowDefinitionStore(&DBWorkflowStore{db: db}),
)
```

Store-sourced definitions are registered alongside config-based definitions during container startup.

## Disabling Workflows

When `cfg.Workflow.Enabled = false`:

- `container.WorkflowEngine()` and `module.WorkflowEngine()` return `nil`.
- The legacy pages service bypasses workflow validation and uses the requested status directly.
- No compilation or registration occurs at startup.

This is suitable for applications that manage status transitions in their own logic or that use a simple draft/published model without guards or editorial review.
