package commands

import (
	"strings"

	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

const commandModuleRoot = "cms.commands"

// CommandLogger returns a module-scoped logger for command handlers, enriching it with
// consistent structured fields so command executions align with Phase 7 observability.
func CommandLogger(provider interfaces.LoggerProvider, module string) interfaces.Logger {
	name := strings.TrimSpace(module)
	if name == "" {
		name = "core"
	}
	logger := logging.ModuleLogger(provider, commandModuleRoot+"."+name)
	return logging.WithFields(logger, map[string]any{
		"component":      "command",
		"command_module": name,
	})
}
