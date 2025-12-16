# go-cms Command Adapters

This submodule provides an integration layer for wiring `go-cms` command handlers into host applications.

It builds command handler structs from a `go-cms` DI container and (optionally) registers them with:

- a command registry (to discover/execute commands),
- a dispatcher (to subscribe handlers), and/or
- a cron scheduler (for handlers that implement `go-command` cron support).

If you don’t need automatic registration/collection and prefer explicit construction, you can instantiate the command handler structs directly in the main `github.com/goliatone/go-cms` module.

## Install

```
go get github.com/goliatone/go-cms/commands
```

## Quick Start

```go
cfg := cms.DefaultConfig()
module, err := cms.New(cfg)
if err != nil {
	log.Fatal(err)
}

result, err := commands.RegisterContainerCommands(module.Container(), commands.RegistrationOptions{
	Registry:   myRegistry,
	Dispatcher: myDispatcher,
	CronRegistrar: myCronRegistrar,
})
if err != nil {
	log.Fatal(err)
}

// Handlers can be invoked directly or used by your dispatcher/cron scheduler.
_ = result.Handlers
```

## Usage

- Call `commands.RegisterContainerCommands(container, opts)`; it inspects the container for configured services and feature flags, then builds a set of handlers and registers them using the integrations you provide.
- `opts.Registry`, `opts.Dispatcher`, and `opts.CronRegistrar` are optional; if you pass none of them you still get back the constructed handlers so you can invoke them yourself.
- `commands/bootstrap/*` provides convenience bootstraps for building a `cms.Module` preconfigured for common CLIs (e.g. markdown and static generation) and can collect handlers for direct execution.
- The `commands/markdown` package exposes helper registration for markdown-specific handlers.

This submodule exists to support hosts that want centralized registration and/or cron/dispatcher wiring without duplicating that plumbing.
