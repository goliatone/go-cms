package runtimeconfig_test

import (
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/runtimeconfig"
	"github.com/goliatone/go-cms/pkg/storage"
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

func TestConfigValidate_AllowsStorageProfiles(t *testing.T) {
	cfg := runtimeconfig.DefaultConfig()
	cfg.Storage.Profiles = []storage.Profile{
		{
			Name:     "primary",
			Provider: "bun",
			Config: storage.Config{
				Name:   "primary",
				Driver: "bun",
				DSN:    "postgres://primary",
			},
			Fallbacks: []string{"replica"},
			Default:   true,
		},
		{
			Name:     "replica",
			Provider: "bun",
			Config: storage.Config{
				Name:   "replica",
				Driver: "bun",
				DSN:    "postgres://replica",
			},
		},
	}
	cfg.Storage.Aliases = map[string]string{
		"content": "primary",
		"media":   "replica",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned unexpected error: %v", err)
	}
}

func TestConfigValidate_StorageProfileRequiresProvider(t *testing.T) {
	cfg := runtimeconfig.DefaultConfig()
	cfg.Storage.Profiles = []storage.Profile{
		{
			Name: "primary",
			Config: storage.Config{
				Name:   "primary",
				Driver: "bun",
				DSN:    "postgres://primary",
			},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, runtimeconfig.ErrStorageProfileProviderRequired) {
		t.Fatalf("expected ErrStorageProfileProviderRequired, got %v", err)
	}
}

func TestConfigValidate_StorageProfileFallbackMustExist(t *testing.T) {
	cfg := runtimeconfig.DefaultConfig()
	cfg.Storage.Profiles = []storage.Profile{
		{
			Name:     "primary",
			Provider: "bun",
			Config: storage.Config{
				Name:   "primary",
				Driver: "bun",
				DSN:    "postgres://primary",
			},
			Fallbacks: []string{"missing"},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, runtimeconfig.ErrStorageProfileFallbackUnknown) {
		t.Fatalf("expected ErrStorageProfileFallbackUnknown, got %v", err)
	}
}

func TestConfigValidate_StorageProfileAliasTargetMustExist(t *testing.T) {
	cfg := runtimeconfig.DefaultConfig()
	cfg.Storage.Profiles = []storage.Profile{
		{
			Name:     "primary",
			Provider: "bun",
			Config: storage.Config{
				Name:   "primary",
				Driver: "bun",
				DSN:    "postgres://primary",
			},
		},
	}
	cfg.Storage.Aliases = map[string]string{
		"content": "missing",
	}

	err := cfg.Validate()
	if !errors.Is(err, runtimeconfig.ErrStorageProfileAliasTargetUnknown) {
		t.Fatalf("expected ErrStorageProfileAliasTargetUnknown, got %v", err)
	}
}

func TestConfigValidate_StorageProfileSingleDefault(t *testing.T) {
	cfg := runtimeconfig.DefaultConfig()
	cfg.Storage.Profiles = []storage.Profile{
		{
			Name:     "primary",
			Provider: "bun",
			Config: storage.Config{
				Name:   "primary",
				Driver: "bun",
				DSN:    "postgres://primary",
			},
			Default: true,
		},
		{
			Name:     "replica",
			Provider: "bun",
			Config: storage.Config{
				Name:   "replica",
				Driver: "bun",
				DSN:    "postgres://replica",
			},
			Default: true,
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, runtimeconfig.ErrStorageProfileMultipleDefaults) {
		t.Fatalf("expected ErrStorageProfileMultipleDefaults, got %v", err)
	}
}

func TestConfigValidate_StorageProfileRequiresConfigFields(t *testing.T) {
	cfg := runtimeconfig.DefaultConfig()
	cfg.Storage.Profiles = []storage.Profile{
		{
			Name:     "primary",
			Provider: "bun",
			Config: storage.Config{
				Name:   "",
				Driver: " ",
				DSN:    "",
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected validation error for incomplete storage config")
	}
	if !errors.Is(err, runtimeconfig.ErrStorageProfileConfigNameRequired) &&
		!errors.Is(err, runtimeconfig.ErrStorageProfileConfigDriverRequired) &&
		!errors.Is(err, runtimeconfig.ErrStorageProfileConfigDSNRequired) {
		t.Fatalf("expected config field error, got %v", err)
	}
}
