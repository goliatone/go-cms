package storage

import "context"

// Provider encapsulates the operations required by go-cms repositories. It embeds
// the legacy StorageProvider contract and adds optional hooks that dynamic
// implementations can satisfy to support runtime reconfiguration.
type Provider interface {
	Query(ctx context.Context, query string, args ...any) (Rows, error)
	Exec(ctx context.Context, query string, args ...any) (Result, error)
	Transaction(ctx context.Context, fn func(tx Transaction) error) error
}

// Reloadable providers can apply a new configuration at runtime. Implementations
// that do not support reloads may omit this interface; the container will keep
// using the existing provider.
type Reloadable interface {
	Reload(ctx context.Context, cfg Config) error
}

// CapabilityReporter exposes optional provider features (e.g., read replicas,
// sharding) so repositories can make runtime decisions.
type CapabilityReporter interface {
	Capabilities() Capabilities
}

// Config captures the runtime configuration for a storage provider. Detailed
// schema validation is handled by higher layers (runtimeconfig/admin services).
type Config struct {
	Name     string
	Driver   string
	DSN      string
	ReadOnly bool
	Options  map[string]any
}

// Capabilities documents optional behaviours supported by a provider.
type Capabilities struct {
	SupportsReload bool
	ReadReplicas   bool
	MultiTenant    bool
	Metadata       map[string]any
}

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
}

type Result interface {
	RowsAffected() (int64, error)
	LastInsertId() (int64, error)
}

type Transaction interface {
	Provider
	Commit() error
	Rollback() error
}
