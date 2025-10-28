package cms_test

import (
	"errors"
	"testing"

	"github.com/goliatone/go-cms"
)

func TestConfigValidateSchedulingRequiresVersioning(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Scheduling = true
	if err := cfg.Validate(); !errors.Is(err, cms.ErrSchedulingFeatureRequiresVersioning) {
		t.Fatalf("expected ErrSchedulingFeatureRequiresVersioning, got %v", err)
	}
}

func TestConfigValidateAdvancedCacheRequiresCache(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Cache.Enabled = false
	cfg.Features.AdvancedCache = true

	if err := cfg.Validate(); !errors.Is(err, cms.ErrAdvancedCacheRequiresEnabledCache) {
		t.Fatalf("expected ErrAdvancedCacheRequiresEnabledCache, got %v", err)
	}
}

func TestConfigValidateCommandsCronRequiresScheduling(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Commands.Enabled = true
	cfg.Commands.AutoRegisterCron = true
	if err := cfg.Validate(); !errors.Is(err, cms.ErrCommandsCronRequiresScheduling) {
		t.Fatalf("expected ErrCommandsCronRequiresScheduling, got %v", err)
	}
}
