package interfaces

import (
	"context"

	"github.com/goliatone/go-cms/pkg/storage"
)

// StorageProvider preserves backwards compatibility for callers still importing
// the legacy interface location. Implementations should prefer satisfying
// pkg/storage.Provider (and optional mix-ins) directly.
type StorageProvider = storage.Provider

// StorageReloadable mirrors storage.Reloadable for compatibility.
type StorageReloadable interface {
	Reload(ctx context.Context, cfg storage.Config) error
}

// StorageCapabilityReporter mirrors storage.CapabilityReporter for compatibility.
type StorageCapabilityReporter interface {
	Capabilities() storage.Capabilities
}

// Rows aliases the new storage.Rows type.
type Rows = storage.Rows

// Result aliases the new storage.Result type.
type Result = storage.Result

// Transaction aliases the new storage.Transaction type.
type Transaction = storage.Transaction
