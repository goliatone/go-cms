# go-cms Command Adapters

This module hosts the legacy registry/collector/cron wiring for `go-cms` commands. Core commands remain direct structs in `github.com/goliatone/go-cms`; consumers that still want automatic registration or CLI collectors can depend on this submodule.

## Install

```
go get github.com/goliatone/go-cms/commands
```

`go.work` already wires this module for local development; external consumers should add a replace to point at their checkout when working locally.

## Usage

- Build a CMS module in core and pass its container to `commands.RegisterContainerCommands` to construct and optionally register command handlers against a registry/dispatcher/cron.
- Reuse the CLI bootstraps under `commands/bootstrap` if you still rely on collector-based CLIs for markdown or static commands.
- Registry helpers (e.g., `commands/markdown.RegisterMarkdownCommands`) and fixtures are available for tests and host integrations that need the old path.

This adapter surface will remain outside the core module; future rewires should prefer constructing command structs directly.
