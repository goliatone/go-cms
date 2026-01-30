package permissions

import (
	"context"
	"strings"
	"sync/atomic"
)

// Strategy resolves permission tokens for an environment scope.
type Strategy interface {
	Resolve(permission, envKey string) []string
}

// StrategyFunc adapts a function into a Strategy.
type StrategyFunc func(permission, envKey string) []string

func (fn StrategyFunc) Resolve(permission, envKey string) []string {
	if fn == nil {
		return nil
	}
	return fn(permission, envKey)
}

const (
	StrategyEnvFirst    = "env_first"
	StrategyGlobalFirst = "global_first"
	StrategyCustom      = "custom"
)

var (
	EnvFirstStrategy Strategy = StrategyFunc(func(permission, envKey string) []string {
		if permission == "" {
			return nil
		}
		if envKey == "" {
			return []string{permission}
		}
		scoped := scopePermission(permission, envKey)
		if scoped == permission {
			return []string{permission}
		}
		return []string{scoped, permission}
	})
	GlobalFirstStrategy Strategy = StrategyFunc(func(permission, envKey string) []string {
		if permission == "" {
			return nil
		}
		if envKey == "" {
			return []string{permission}
		}
		scoped := scopePermission(permission, envKey)
		if scoped == permission {
			return []string{permission}
		}
		return []string{permission, scoped}
	})
)

// EnvironmentScopeConfig configures environment-scoped permission resolution.
type EnvironmentScopeConfig struct {
	Enabled  bool
	Strategy Strategy
}

var (
	defaultEnvironmentScopeConfig = EnvironmentScopeConfig{
		Enabled:  false,
		Strategy: EnvFirstStrategy,
	}
	environmentScopeConfig atomic.Value
)

// ConfigureEnvironmentScope updates the global environment permission scope configuration.
func ConfigureEnvironmentScope(cfg EnvironmentScopeConfig) {
	if cfg.Strategy == nil {
		cfg.Strategy = EnvFirstStrategy
	}
	environmentScopeConfig.Store(cfg)
}

func currentEnvironmentScopeConfig() EnvironmentScopeConfig {
	if value := environmentScopeConfig.Load(); value != nil {
		if cfg, ok := value.(EnvironmentScopeConfig); ok {
			return cfg
		}
	}
	return defaultEnvironmentScopeConfig
}

type environmentKeyContextKey string

const environmentKeyContext environmentKeyContextKey = "cms.permissions.environment_key"

// WithEnvironmentKey stores the environment key on the context for permission checks.
func WithEnvironmentKey(ctx context.Context, key string) context.Context {
	if ctx == nil {
		return ctx
	}
	normalized := normalizeEnvironmentKey(key)
	if normalized == "" {
		return ctx
	}
	return context.WithValue(ctx, environmentKeyContext, normalized)
}

// EnvironmentKeyFromContext returns the stored environment key, if any.
func EnvironmentKeyFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value := ctx.Value(environmentKeyContext); value != nil {
		if key, ok := value.(string); ok {
			return normalizeEnvironmentKey(key)
		}
	}
	return ""
}

func normalizeEnvironmentKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func resolveScopedPermission(ctx context.Context, permission string) (string, string) {
	base, env := splitPermissionScope(permission)
	base = normalizePermission(base)
	if base == "" {
		return "", ""
	}
	env = normalizeEnvironmentKey(env)
	if env == "" {
		env = EnvironmentKeyFromContext(ctx)
	}
	return base, env
}

func splitPermissionScope(permission string) (string, string) {
	if permission == "" {
		return "", ""
	}
	parts := strings.SplitN(permission, "@", 2)
	if len(parts) == 2 {
		env := strings.TrimSpace(parts[1])
		if env != "" {
			return parts[0], env
		}
	}
	return permission, ""
}

func scopePermission(permission, envKey string) string {
	if permission == "" || envKey == "" {
		return permission
	}
	if strings.Contains(permission, "@") {
		return permission
	}
	return permission + "@" + envKey
}

func allowedWithScope(ctx context.Context, checker Checker, permission string) bool {
	if checker == nil {
		return true
	}
	base, envKey := resolveScopedPermission(ctx, permission)
	if base == "" {
		return true
	}
	cfg := currentEnvironmentScopeConfig()
	if !cfg.Enabled || envKey == "" {
		return checker.Allowed(base)
	}
	strategy := cfg.Strategy
	if strategy == nil {
		strategy = EnvFirstStrategy
	}
	for _, candidate := range strategy.Resolve(base, envKey) {
		normalized := normalizePermission(candidate)
		if normalized == "" {
			continue
		}
		if checker.Allowed(normalized) {
			return true
		}
	}
	return false
}
