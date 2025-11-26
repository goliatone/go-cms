# go-cms Command Adapters

This module hosts the legacy registry/collector/cron wiring for `go-cms` commands. Core commands remain direct structs in `github.com/goliatone/go-cms`; consumers that still want automatic registration or CLI collectors can depend on this submodule.

## Install

```
go get github.com/goliatone/go-cms/commands
```

`go.work` already wires this module for local development; external consumers should add a replace to point at their checkout when working locally.

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
	Cron:       myCron,
})
if err != nil {
	log.Fatal(err)
}

// Handlers can be invoked directly or used by your dispatcher/cron scheduler.
_ = result.Handlers
```

## Usage

- Build a CMS module in core and pass its container to `commands.RegisterContainerCommands` to construct and optionally register command handlers against a registry/dispatcher/cron.
- Reuse the CLI bootstraps under `commands/bootstrap` if you still rely on collector-based CLIs for markdown or static commands.
- Registry helpers (e.g., `commands/markdown.RegisterMarkdownCommands`) and fixtures are available for tests and host integrations that need the old path.

This adapter surface will remain outside the core module; future rewires should prefer constructing command structs directly.
