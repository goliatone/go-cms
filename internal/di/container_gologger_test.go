package di

import (
	"testing"

	"github.com/goliatone/go-cms/internal/logging/gologger"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
)

func TestConfigureLoggerProviderUsesGoLoggerAdapter(t *testing.T) {
	cfg := runtimeconfig.DefaultConfig()
	cfg.Features.Logger = true
	cfg.Logging.Provider = "gologger"
	cfg.Logging.Level = "debug"
	cfg.Logging.Format = "json"

	container, err := NewContainer(cfg)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}

	provider, ok := container.loggerProvider.(*gologger.Provider)
	if !ok {
		t.Fatalf("expected go-logger provider, got %T", container.loggerProvider)
	}

	logger := provider.GetLogger("cms.test")
	if logger == nil {
		t.Fatal("expected logger from go-logger provider, got nil")
	}
}
