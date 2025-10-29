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
