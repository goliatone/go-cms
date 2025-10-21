package interfaces

import "context"

type StorageProvider interface {
	Query(ctx context.Context, query string, args ...any) (Rows, error)
	Exec(ctx context.Context, query string, args ...any) (Result, error)
	Transaction(ctx context.Context, fn func(tx Transaction) error) error
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
	StorageProvider
	Commit() error
	Rollback() error
}
