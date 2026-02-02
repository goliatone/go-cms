# Activity Guide

This guide covers activity event emission, hook integration, audit logging, and testing strategies in `go-cms`. By the end you will understand how to enable the activity system, receive structured events from every CMS module, integrate with external sinks like `go-users`, and write tests that assert on emitted events without persistence.

## Activity Architecture Overview

The activity system in `go-cms` is a lightweight, decoupled event emission framework built on three layers:

- **Events** are structured records emitted by CMS services whenever a significant operation occurs. Each event carries a verb (what happened), actor IDs (who did it), object identifiers (what was affected), a channel tag, and module-specific metadata.
- **Hooks** are notification targets that receive events. A hook is any value satisfying the `ActivityHook` interface. Multiple hooks can be registered and all receive every event via fan-out.
- **Emitter** is the central dispatcher that gates emission behind feature flags, applies defaults (channel, timestamp), validates required fields, and fans out normalized events to all registered hooks.

```
Config (Features.Activity, Activity.Enabled, Activity.Channel)
  |
  v
Emitter (gates + normalizes + dispatches)
  |
  v
Hooks [ HookFunc, CaptureHook, usersink.Hook, ... ]
  |
  v
External systems (go-users, logging, analytics, queues)
```

Services never depend on specific sink implementations. The emitter abstracts hook dispatch so that adding or removing consumers requires no changes to business logic.

All event IDs are strings (not UUIDs) to avoid coupling emitters to specific ID formats. Timestamps are always UTC.

### Accessing the Emitter

Activity emission is wired automatically through the DI container. Services receive an `*activity.Emitter` during construction and emit events internally. You do not call the emitter directly -- instead, you configure hooks at container creation time and the services handle emission.

```go
cfg := cms.DefaultConfig()
cfg.Features.Activity = true
cfg.Activity.Enabled = true
cfg.Activity.Channel = "cms"

hook := activity.HookFunc(func(ctx context.Context, event activity.Event) error {
    log.Printf("[%s] %s %s/%s", event.Channel, event.Verb, event.ObjectType, event.ObjectID)
    return nil
})

module, err := cms.New(cfg, di.WithActivityHooks(activity.Hooks{hook}))
if err != nil {
    log.Fatal(err)
}

// All services now emit activity events to the registered hook
contentSvc := module.Content()
```

When `cfg.Features.Activity` is `false` (the default), no emitter is created and all emission calls are no-ops. No resources are allocated.

---

## Enabling Activity

Activity is disabled by default. Two flags must be set for emissions to occur:

```go
cfg := cms.DefaultConfig()
cfg.Features.Activity = true  // Enable the activity feature
cfg.Activity.Enabled = true   // Enable event emission
cfg.Activity.Channel = "cms"  // Default channel tag for all events
```

Both `Features.Activity` and `Activity.Enabled` must be `true`. If `Activity.Enabled` is `true` but `Features.Activity` is `false`, container initialization returns `ErrActivityFeatureRequired`.

The `Channel` field sets a default tag applied to all events that do not specify their own channel. This is useful for filtering events in multi-module systems where several subsystems emit to the same hook pipeline.

---

## Configuration

### ActivityConfig Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Enabled` | `bool` | `false` | Enable or disable event emission |
| `Channel` | `string` | `"cms"` | Default channel tag applied to events without an explicit channel |

### Feature Flag

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `Features.Activity` | `bool` | `false` | Master switch for the activity subsystem |

### Minimal Configuration

```go
cfg := cms.DefaultConfig()
cfg.Features.Activity = true
cfg.Activity.Enabled = true
```

### Configuration with Custom Channel

```go
cfg := cms.DefaultConfig()
cfg.Features.Activity = true
cfg.Activity.Enabled = true
cfg.Activity.Channel = "website-cms"
```

---

## Activity Event Structure

Every activity event uses the `activity.Event` type:

```go
type Event struct {
    Verb           string         // Action: "create", "update", "delete", "publish", etc.
    ActorID        string         // UUID of the actor performing the action
    UserID         string         // UUID of the associated user
    TenantID       string         // UUID of the tenant (multi-tenancy)
    ObjectType     string         // Entity type: "content", "page", "block", "menu", "widget"
    ObjectID       string         // UUID of the affected object
    Channel        string         // Channel tag for filtering (defaults to config value)
    DefinitionCode string         // Optional definition or template code
    Recipients     []string       // Optional list of recipient identifiers
    Metadata       map[string]any // Module-specific metadata
    OccurredAt     time.Time      // Timestamp (auto-set to now if zero)
}
```

### Required Fields

The hooks layer validates three fields before dispatching an event. Events missing any of these are silently skipped:

| Field | Description |
|-------|-------------|
| `Verb` | The action that occurred |
| `ObjectType` | The type of entity affected |
| `ObjectID` | The identifier of the affected entity |

### Event Normalization

Before an event reaches hooks, it is normalized:

1. All string fields are trimmed of leading/trailing whitespace
2. If `OccurredAt` is zero, it is set to `time.Now().UTC()`
3. If `Channel` is empty, the emitter applies the configured default channel
4. The `Metadata` map is cloned to prevent mutations after emission

### Common Verbs

| Verb | Description |
|------|-------------|
| `create` | A new entity was created |
| `update` | An existing entity was modified |
| `delete` | An entity was removed |
| `publish` | Content or page was published |
| `unpublish` | Content or page was unpublished |
| `schedule` | Content was scheduled for future publish/unpublish |
| `move` | A page was moved to a new parent |
| `reorder` | Items were reordered (menu items, area widgets) |
| `register` | A definition was registered (widget definitions) |
| `execute` | A scheduled job was executed |
| `enqueue` | A job was enqueued |
| `complete` | A job completed successfully |
| `fail` | A job failed |
| `retry` | A job was retried |

---

## Module-Specific Metadata

Each CMS module emits events with context-specific metadata fields in the `Metadata` map. The metadata keys vary by module and verb.

### Content Events

**Object type:** `"content"`

**Verbs:** `create`, `update`, `delete`, `schedule`

| Metadata Key | Type | Description |
|-------------|------|-------------|
| `slug` | `string` | Content slug |
| `status` | `string` | Content status (e.g. "draft", "published") |
| `locales` | `[]string` | Locale codes affected by the operation |
| `content_type_id` | `string` | Content type UUID |
| `environment_id` | `string` | Environment UUID (when environments are enabled) |
| `environment_key` | `string` | Environment key (resolved from environment ID) |
| `publish_at` | `time.Time` | Scheduled publish timestamp (schedule verb) |
| `unpublish_at` | `time.Time` | Scheduled unpublish timestamp (schedule verb) |

### Page Events

**Object type:** `"page"`

**Verbs:** `create`, `update`, `delete`, `publish`, `unpublish`, `move`, `reorder`

| Metadata Key | Type | Description |
|-------------|------|-------------|
| `path` | `string` | Page path |
| `locale` | `string` | Page locale |
| `template_id` | `string` | Template UUID |
| `status` | `string` | Page status |
| `parent_id` | `string` | Parent page UUID |
| `environment_id` | `string` | Environment UUID |
| `environment_key` | `string` | Environment key |

### Block Events

**Object type:** `"block"`

**Verbs:** `create`, `update`, `delete`

| Metadata Key | Type | Description |
|-------------|------|-------------|
| `template_id` | `string` | Block definition/template ID |
| `locales` | `[]string` | Affected locale codes |
| `environment_id` | `string` | Environment UUID |
| `environment_key` | `string` | Environment key |

### Widget Events

**Object type:** `"widget"`

**Verbs:** `register`, `create`, `update`, `delete`

| Metadata Key | Type | Description |
|-------------|------|-------------|
| `definition_code` | `string` | Widget definition code |
| `area` | `string` | Widget area placement code |
| `environment_id` | `string` | Environment UUID |
| `environment_key` | `string` | Environment key |

### Menu Events

**Object type:** `"menu"`

**Verbs:** `create`, `update`, `delete`, `reorder`

| Metadata Key | Type | Description |
|-------------|------|-------------|
| `code` | `string` | Menu code |
| `locale` | `string` | Menu locale |
| `item_count` | `int` | Number of menu items |
| `environment_id` | `string` | Environment UUID |
| `environment_key` | `string` | Environment key |

### Job Events

**Object type:** `"job"`

**Verbs:** `execute`, `enqueue`, `retry`, `complete`, `fail`

| Metadata Key | Type | Description |
|-------------|------|-------------|
| `job_id` | `string` | Job UUID |
| `job_name` | `string` | Job name |
| `status` | `string` | Job status |
| `error` | `string` | Error message (fail verb) |
| `duration_ms` | `int64` | Execution duration in milliseconds |

---

## Hook Injection

Hooks are registered at container creation time via DI options. Two injection methods are available.

### Custom Hooks with `di.WithActivityHooks`

Register one or more hooks that implement the `ActivityHook` interface:

```go
type ActivityHook interface {
    Notify(ctx context.Context, event Event) error
}
```

The simplest way to create a hook is with `HookFunc`, which adapts a plain function:

```go
logHook := activity.HookFunc(func(ctx context.Context, event activity.Event) error {
    log.Printf("[%s] %s %s/%s by %s",
        event.Channel,
        event.Verb,
        event.ObjectType,
        event.ObjectID,
        event.ActorID,
    )
    return nil
})

module, err := cms.New(cfg, di.WithActivityHooks(activity.Hooks{logHook}))
```

Multiple hooks receive every event via fan-out:

```go
auditHook := activity.HookFunc(func(ctx context.Context, event activity.Event) error {
    return auditLog.Write(event)
})

analyticsHook := activity.HookFunc(func(ctx context.Context, event activity.Event) error {
    return analytics.Track(event.Verb, event.ObjectType, event.Metadata)
})

module, err := cms.New(cfg,
    di.WithActivityHooks(activity.Hooks{auditHook, analyticsHook}),
)
```

### go-users Integration with `di.WithActivitySink`

If you use `go-users` for user management, pass an `ActivitySink` to bridge CMS events into the go-users activity log:

```go
// goUsersSink satisfies the usertypes.ActivitySink interface
module, err := cms.New(cfg, di.WithActivitySink(goUsersSink))
```

The `usersink.Hook` adapter internally converts `activity.Event` to `usertypes.ActivityRecord`:

- String IDs are parsed to UUIDs (falling back to `uuid.Nil` on parse failure)
- `Metadata` is cloned into the record's `Data` map
- `DefinitionCode` and `Recipients` are added to `Data` when present
- The adapter calls `Sink.Log(ctx, record)` to persist the activity

### Combining Both Methods

Both options can be used together. The container appends all hooks into a single list:

```go
captureHook := &activity.CaptureHook{}

module, err := cms.New(cfg,
    di.WithActivitySink(goUsersSink),                          // go-users persistence
    di.WithActivityHooks(activity.Hooks{captureHook, logHook}), // custom hooks
)
```

All three hooks (the usersink adapter, captureHook, and logHook) receive every event.

---

## No-Op Behavior

The activity system is designed for graceful degradation. When hooks are not provided or the feature is disabled, no emissions occur and no errors are raised.

### When Activity is Disabled

If `Features.Activity` is `false` or `Activity.Enabled` is `false`:

- The emitter's `Enabled()` method returns `false`
- All `Emit()` calls return `nil` immediately
- Services skip emission entirely (they check `Enabled()` before constructing events)
- No event objects are allocated

### When No Hooks Are Registered

If both flags are `true` but no hooks are provided:

- `Hooks.Enabled()` returns `false` (empty hook list)
- The emitter treats this as disabled
- Events are not constructed or dispatched

### When Hooks Return Errors

Hook errors do not propagate to the calling service:

- Services call `_ = s.activity.Emit(ctx, event)`, explicitly discarding the error
- Failed hooks do not prevent business operations from completing
- Multiple hooks are called independently; one hook's failure does not prevent others from receiving the event
- Errors from all hooks are collected and joined, but the combined error is discarded at the service level

This design ensures that activity tracking is purely observational and never blocks or breaks CMS operations.

---

## Writing Custom Hooks

### Basic Hook

Implement the `ActivityHook` interface directly:

```go
type WebhookNotifier struct {
    URL    string
    Client *http.Client
}

func (w *WebhookNotifier) Notify(ctx context.Context, event activity.Event) error {
    payload, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("marshal event: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(payload))
    if err != nil {
        return fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := w.Client.Do(req)
    if err != nil {
        return fmt.Errorf("send webhook: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned %d", resp.StatusCode)
    }
    return nil
}
```

Register it with the container:

```go
webhook := &WebhookNotifier{
    URL:    "https://hooks.example.com/cms-events",
    Client: &http.Client{Timeout: 5 * time.Second},
}

module, err := cms.New(cfg,
    di.WithActivityHooks(activity.Hooks{webhook}),
)
```

### Function Hook

For simpler cases, use `HookFunc` to avoid defining a struct:

```go
metricsHook := activity.HookFunc(func(ctx context.Context, event activity.Event) error {
    metrics.IncrCounter([]string{"cms", "activity", event.Verb, event.ObjectType}, 1)
    return nil
})
```

### Filtering Hook

Create a hook that only processes events matching specific criteria:

```go
type FilteredHook struct {
    Verbs  map[string]bool
    Inner  activity.ActivityHook
}

func (f *FilteredHook) Notify(ctx context.Context, event activity.Event) error {
    if !f.Verbs[event.Verb] {
        return nil // skip events we don't care about
    }
    return f.Inner.Notify(ctx, event)
}

// Only forward create and delete events
filteredWebhook := &FilteredHook{
    Verbs: map[string]bool{"create": true, "delete": true},
    Inner: webhook,
}
```

---

## Testing Activities

The `activity` package provides `CaptureHook`, a thread-safe hook that records all received events for assertion in tests.

### CaptureHook

```go
type CaptureHook struct {
    Events []Event    // Recorded events (normalized)
    Err    error      // Error to return from Notify (for error simulation)
    mu     sync.Mutex // Thread safety
}
```

`CaptureHook` stores every event it receives (after normalization) and returns the configured `Err` from `Notify`. It is safe for concurrent use.

### Basic Test Pattern

```go
func TestContentCreateEmitsActivity(t *testing.T) {
    // 1. Create a capture hook and emitter
    hook := &activity.CaptureHook{}
    emitter := activity.NewEmitter(activity.Hooks{hook}, activity.Config{
        Enabled: true,
        Channel: "cms",
    })

    // 2. Create the service with the emitter
    svc := content.NewService(
        contentStore,
        typeStore,
        localeStore,
        content.WithActivityEmitter(emitter),
    )

    // 3. Perform the operation
    actorID := uuid.New()
    record, err := svc.Create(ctx, content.CreateContentRequest{
        ContentTypeID: contentTypeID,
        Slug:          "test-article",
        Status:        "draft",
        CreatedBy:     actorID,
        UpdatedBy:     actorID,
        Translations: []content.ContentTranslationInput{
            {Locale: "en", Title: "Test Article"},
        },
    })
    if err != nil {
        t.Fatalf("create: %v", err)
    }

    // 4. Assert on captured events
    if len(hook.Events) != 1 {
        t.Fatalf("expected 1 event, got %d", len(hook.Events))
    }

    event := hook.Events[0]

    if event.Verb != "create" {
        t.Errorf("expected verb 'create', got %q", event.Verb)
    }
    if event.ObjectType != "content" {
        t.Errorf("expected object type 'content', got %q", event.ObjectType)
    }
    if event.ObjectID != record.ID.String() {
        t.Errorf("expected object ID %s, got %s", record.ID, event.ObjectID)
    }
    if event.ActorID != actorID.String() {
        t.Errorf("expected actor ID %s, got %s", actorID, event.ActorID)
    }
    if event.Channel != "cms" {
        t.Errorf("expected channel 'cms', got %q", event.Channel)
    }
}
```

### Asserting Metadata

```go
func TestContentCreateMetadata(t *testing.T) {
    hook := &activity.CaptureHook{}
    emitter := activity.NewEmitter(activity.Hooks{hook}, activity.Config{
        Enabled: true,
        Channel: "cms",
    })

    svc := content.NewService(
        contentStore, typeStore, localeStore,
        content.WithActivityEmitter(emitter),
    )

    _, err := svc.Create(ctx, content.CreateContentRequest{
        ContentTypeID: contentTypeID,
        Slug:          "activity-test",
        Status:        "draft",
        CreatedBy:     uuid.New(),
        UpdatedBy:     uuid.New(),
        Translations: []content.ContentTranslationInput{
            {Locale: "en", Title: "English"},
            {Locale: "es", Title: "Spanish"},
        },
    })
    if err != nil {
        t.Fatalf("create: %v", err)
    }

    event := hook.Events[0]

    // Assert slug metadata
    slug, ok := event.Metadata["slug"].(string)
    if !ok || slug != "activity-test" {
        t.Errorf("expected slug 'activity-test', got %v", event.Metadata["slug"])
    }

    // Assert status metadata
    status, ok := event.Metadata["status"].(string)
    if !ok || status != "draft" {
        t.Errorf("expected status 'draft', got %v", event.Metadata["status"])
    }

    // Assert locales metadata
    locales, ok := event.Metadata["locales"].([]string)
    if !ok || len(locales) != 2 {
        t.Errorf("expected 2 locales, got %v", event.Metadata["locales"])
    }
}
```

### Asserting Multiple Events

```go
func TestPageLifecycleEvents(t *testing.T) {
    hook := &activity.CaptureHook{}
    emitter := activity.NewEmitter(activity.Hooks{hook}, activity.Config{
        Enabled: true,
        Channel: "cms",
    })

    svc := pages.NewService(
        pageStore, contentStore,
        pages.WithActivityEmitter(emitter),
    )

    // Create
    page, _ := svc.Create(ctx, pages.CreatePageRequest{/* ... */})
    // Update
    svc.Update(ctx, pages.UpdatePageRequest{ID: page.ID /* ... */})
    // Delete
    svc.Delete(ctx, pages.DeletePageRequest{ID: page.ID /* ... */})

    // Assert the full event sequence
    if len(hook.Events) != 3 {
        t.Fatalf("expected 3 events, got %d", len(hook.Events))
    }

    verbs := make([]string, len(hook.Events))
    for i, e := range hook.Events {
        verbs[i] = e.Verb
    }

    expected := []string{"create", "update", "delete"}
    for i, v := range expected {
        if verbs[i] != v {
            t.Errorf("event %d: expected verb %q, got %q", i, v, verbs[i])
        }
    }
}
```

### Simulating Hook Errors

Test that hook errors do not disrupt service operations:

```go
func TestHookErrorDoesNotBreakService(t *testing.T) {
    hook := &activity.CaptureHook{
        Err: errors.New("webhook unavailable"),
    }
    emitter := activity.NewEmitter(activity.Hooks{hook}, activity.Config{
        Enabled: true,
        Channel: "cms",
    })

    svc := content.NewService(
        contentStore, typeStore, localeStore,
        content.WithActivityEmitter(emitter),
    )

    // The operation succeeds despite the hook error
    _, err := svc.Create(ctx, content.CreateContentRequest{
        ContentTypeID: contentTypeID,
        Slug:          "resilient-content",
        Status:        "draft",
        CreatedBy:     uuid.New(),
        UpdatedBy:     uuid.New(),
        Translations: []content.ContentTranslationInput{
            {Locale: "en", Title: "Resilient"},
        },
    })
    if err != nil {
        t.Fatalf("create should succeed despite hook error: %v", err)
    }

    // The event was still captured (CaptureHook records before returning error)
    if len(hook.Events) != 1 {
        t.Fatalf("expected 1 captured event, got %d", len(hook.Events))
    }
}
```

### Testing Without Activity

Verify that services work correctly when activity is disabled:

```go
func TestServiceWorksWithoutActivity(t *testing.T) {
    // No emitter provided -- activity is a no-op
    svc := content.NewService(contentStore, typeStore, localeStore)

    _, err := svc.Create(ctx, content.CreateContentRequest{
        ContentTypeID: contentTypeID,
        Slug:          "no-activity",
        Status:        "draft",
        CreatedBy:     uuid.New(),
        UpdatedBy:     uuid.New(),
        Translations: []content.ContentTranslationInput{
            {Locale: "en", Title: "No Activity"},
        },
    })
    if err != nil {
        t.Fatalf("create should succeed without activity emitter: %v", err)
    }
}
```

---

## Integration with the DI Container

The DI container handles all wiring between configuration, hooks, emitter, and services.

### Wiring Flow

1. Hooks are collected from all `di.WithActivityHooks()` and `di.WithActivitySink()` options
2. The container builds an emitter with the combined hooks and activity config
3. The emitter is passed to every service via `WithActivityEmitter()` service options
4. Services that receive the emitter call `emitActivity()` after successful operations

### Services That Emit Events

The following services receive the activity emitter:

| Service | Object Type | Typical Verbs |
|---------|-------------|---------------|
| `content.Service` | `content` | create, update, delete, schedule |
| `pages.Service` | `page` | create, update, delete, publish, unpublish, move, reorder |
| `blocks.Service` | `block` | create, update, delete |
| `widgets.Service` | `widget` | register, create, update, delete |
| `menus.Service` | `menu` | create, update, delete, reorder |
| Jobs Worker | `job` | execute, enqueue, retry, complete, fail |

### Service-Level Emission Pattern

Every service follows the same internal pattern:

```go
func (s *service) emitActivity(
    ctx context.Context,
    actor uuid.UUID,
    verb, objectType string,
    objectID uuid.UUID,
    meta map[string]any,
) {
    if s.activity == nil || !s.activity.Enabled() || objectID == uuid.Nil {
        return
    }

    event := activity.Event{
        Verb:       verb,
        ActorID:    actor.String(),
        ObjectType: objectType,
        ObjectID:   objectID.String(),
        Metadata:   meta,
    }
    _ = s.activity.Emit(ctx, event)
}
```

Key characteristics:
- **Nil-safe**: Checks for nil emitter before use
- **ID guard**: Skips emission when the object ID is `uuid.Nil`
- **Fire-and-forget**: Discards emission errors to avoid disrupting business logic
- **Environment enrichment**: When environments are enabled, the emitter resolves `environment_key` from `environment_id` automatically

---

## Common Patterns

### Audit Log

Persist all CMS events to an audit log table:

```go
type AuditLogger struct {
    DB *sql.DB
}

func (a *AuditLogger) Notify(ctx context.Context, event activity.Event) error {
    metadata, _ := json.Marshal(event.Metadata)
    _, err := a.DB.ExecContext(ctx,
        `INSERT INTO audit_log (verb, actor_id, object_type, object_id, channel, metadata, occurred_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        event.Verb, event.ActorID, event.ObjectType, event.ObjectID,
        event.Channel, metadata, event.OccurredAt,
    )
    return err
}

module, err := cms.New(cfg,
    di.WithActivityHooks(activity.Hooks{&AuditLogger{DB: db}}),
)
```

### Real-Time Notifications

Forward events to a message broker for real-time processing:

```go
type EventPublisher struct {
    Topic *pubsub.Topic
}

func (p *EventPublisher) Notify(ctx context.Context, event activity.Event) error {
    data, _ := json.Marshal(event)
    _, err := p.Topic.Publish(ctx, &pubsub.Message{Data: data})
    return err
}

module, err := cms.New(cfg,
    di.WithActivityHooks(activity.Hooks{&EventPublisher{Topic: topic}}),
)
```

### Selective Channel Routing

Use the channel field to route events to different destinations:

```go
cfg.Activity.Channel = "cms-production"

router := activity.HookFunc(func(ctx context.Context, event activity.Event) error {
    switch event.Channel {
    case "cms-production":
        return productionSink.Log(ctx, event)
    case "cms-staging":
        return stagingSink.Log(ctx, event)
    default:
        return defaultSink.Log(ctx, event)
    }
})
```

### go-users Activity Sink

Bridge CMS events into the go-users activity system for unified user activity tracking:

```go
import "github.com/goliatone/go-users/pkg/types"

// The sink satisfies types.ActivitySink from go-users
var sink types.ActivitySink = myUserActivityStore

module, err := cms.New(cfg, di.WithActivitySink(sink))
```

The `usersink.Hook` adapter converts CMS `activity.Event` to go-users `types.ActivityRecord`:

| CMS Event Field | Activity Record Field | Conversion |
|----------------|----------------------|------------|
| `Verb` | `Verb` | Direct copy |
| `ActorID` | `ActorID` | Parsed to UUID; `uuid.Nil` on failure |
| `UserID` | `UserID` | Parsed to UUID; `uuid.Nil` on failure |
| `TenantID` | `TenantID` | Parsed to UUID; `uuid.Nil` on failure |
| `ObjectType` | `ObjectType` | Direct copy |
| `ObjectID` | `ObjectID` | Direct copy (string) |
| `Channel` | `Channel` | Direct copy |
| `Metadata` | `Data` | Cloned map |
| `DefinitionCode` | `Data["definition_code"]` | Added when non-empty |
| `Recipients` | `Data["recipients"]` | Added when non-empty |
| `OccurredAt` | `OccurredAt` | Direct copy |

---

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/internal/di"
    "github.com/goliatone/go-cms/pkg/activity"
    "github.com/google/uuid"
)

func main() {
    ctx := context.Background()

    // 1. Configure with activity enabled
    cfg := cms.DefaultConfig()
    cfg.Features.Activity = true
    cfg.Activity.Enabled = true
    cfg.Activity.Channel = "cms"
    cfg.DefaultLocale = "en"
    cfg.I18N.Locales = []string{"en"}

    // 2. Create hooks
    logHook := activity.HookFunc(func(ctx context.Context, event activity.Event) error {
        fmt.Printf("[%s] %s %s/%s (actor=%s)\n",
            event.Channel,
            event.Verb,
            event.ObjectType,
            event.ObjectID,
            event.ActorID,
        )
        if slug, ok := event.Metadata["slug"]; ok {
            fmt.Printf("  slug: %s\n", slug)
        }
        if status, ok := event.Metadata["status"]; ok {
            fmt.Printf("  status: %s\n", status)
        }
        return nil
    })

    captureHook := &activity.CaptureHook{}

    // 3. Create the module with both hooks
    module, err := cms.New(cfg,
        di.WithActivityHooks(activity.Hooks{logHook, captureHook}),
    )
    if err != nil {
        log.Fatal(err)
    }

    // 4. Use the content service -- events are emitted automatically
    contentSvc := module.Content()
    actorID := uuid.New()

    // Create a content type
    contentType, err := contentSvc.CreateContentType(ctx, cms.CreateContentTypeRequest{
        Name: "Article",
        Slug: "article",
        Schema: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "body": map[string]any{"type": "string"},
            },
        },
    })
    if err != nil {
        log.Fatalf("create content type: %v", err)
    }

    // Create content -- this emits a "create" event
    record, err := contentSvc.Create(ctx, cms.CreateContentRequest{
        ContentTypeID: contentType.ID,
        Slug:          "hello-world",
        Status:        "draft",
        CreatedBy:     actorID,
        UpdatedBy:     actorID,
        Translations: []cms.ContentTranslationInput{
            {Locale: "en", Title: "Hello World", Body: "Welcome to go-cms"},
        },
    })
    if err != nil {
        log.Fatalf("create content: %v", err)
    }

    // Update content -- this emits an "update" event
    _, err = contentSvc.Update(ctx, cms.UpdateContentRequest{
        ID:        record.ID,
        Slug:      stringPtr("hello-world-updated"),
        UpdatedBy: actorID,
    })
    if err != nil {
        log.Fatalf("update content: %v", err)
    }

    // Delete content -- this emits a "delete" event
    err = contentSvc.Delete(ctx, cms.DeleteContentRequest{
        ID:        record.ID,
        DeletedBy: actorID,
    })
    if err != nil {
        log.Fatalf("delete content: %v", err)
    }

    // 5. Review captured events
    fmt.Printf("\nCaptured %d events:\n", len(captureHook.Events))
    for i, event := range captureHook.Events {
        fmt.Printf("  %d. %s %s/%s\n", i+1, event.Verb, event.ObjectType, event.ObjectID)
    }
}

func stringPtr(s string) *string { return &s }
```

**Expected output:**

```
[cms] create content/<uuid> (actor=<uuid>)
  slug: hello-world
  status: draft
[cms] update content/<uuid> (actor=<uuid>)
  slug: hello-world-updated
[cms] delete content/<uuid> (actor=<uuid>)

Captured 3 events:
  1. create content/<uuid>
  2. update content/<uuid>
  3. delete content/<uuid>
```

---

## Reference

For the technical design specification behind the activity system, see [ACTIVITY_TDD.md](ACTIVITY_TDD.md).

---

## Next Steps

- [GUIDE_CONFIGURATION.md](GUIDE_CONFIGURATION.md) -- full config reference and DI container wiring
- [GUIDE_CONTENT.md](GUIDE_CONTENT.md) -- content types, entries, translations, and versioning
- [GUIDE_WORKFLOW.md](GUIDE_WORKFLOW.md) -- content lifecycle orchestration with state machines
- [GUIDE_TESTING.md](GUIDE_TESTING.md) -- testing strategies for applications using go-cms
