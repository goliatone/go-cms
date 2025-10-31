package runtimeconfig_test

import (
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/runtimeconfig"
)

func TestConfigValidate_AllowsDisabledGeneratorWithoutOutput(t *testing.T) {
	cfg := runtimeconfig.DefaultConfig()
	cfg.Generator.OutputDir = ""

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned unexpected error: %v", err)
	}
}

func TestConfigValidate_RequiresOutputDirWhenGeneratorEnabled(t *testing.T) {
	cfg := runtimeconfig.DefaultConfig()
	cfg.Generator.Enabled = true
	cfg.Generator.OutputDir = " "

	err := cfg.Validate()
	if !errors.Is(err, runtimeconfig.ErrGeneratorOutputDirRequired) {
		t.Fatalf("expected ErrGeneratorOutputDirRequired, got %v", err)
	}
}

func TestConfigValidate_RequiresLoggingProviderWhenFeatureEnabled(t *testing.T) {
	cfg := runtimeconfig.DefaultConfig()
	cfg.Features.Logger = true
	cfg.Logging.Provider = ""

	err := cfg.Validate()
	if !errors.Is(err, runtimeconfig.ErrLoggingProviderRequired) {
		t.Fatalf("expected ErrLoggingProviderRequired, got %v", err)
	}
}

func TestConfigValidate_RejectsUnknownLoggingProvider(t *testing.T) {
	cfg := runtimeconfig.DefaultConfig()
	cfg.Features.Logger = true
	cfg.Logging.Provider = "syslog"

	err := cfg.Validate()
	if !errors.Is(err, runtimeconfig.ErrLoggingProviderUnknown) {
		t.Fatalf("expected ErrLoggingProviderUnknown, got %v", err)
	}
}

func TestConfigValidate_RejectsInvalidLoggingFormat(t *testing.T) {
	cfg := runtimeconfig.DefaultConfig()
	cfg.Features.Logger = true
	cfg.Logging.Provider = "gologger"
	cfg.Logging.Format = "xml"

	err := cfg.Validate()
	if !errors.Is(err, runtimeconfig.ErrLoggingFormatInvalid) {
		t.Fatalf("expected ErrLoggingFormatInvalid, got %v", err)
	}
}
