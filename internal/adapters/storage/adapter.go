package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

type BunExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

type BunStorageAdapter struct {
	db BunExecutor
}

func NewBunStorageAdapter(db BunExecutor) interfaces.StorageProvider {
	return &BunStorageAdapter{db: db}
}

func (a *BunStorageAdapter) Query(ctx context.Context, query string, args ...any) (interfaces.Rows, error) {
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		return nil, nil
	}
	return &sqlRows{rows: rows}, nil
}

func (a *BunStorageAdapter) Exec(ctx context.Context, query string, args ...any) (interfaces.Result, error) {
	result, err := a.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return sqlResult{Result: result}, nil
}

func (a *BunStorageAdapter) Transaction(ctx context.Context, fn func(tx interfaces.Transaction) error) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	wrapped := &bunTx{tx: tx}
	if err := fn(wrapped); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed ater error %w: %v", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}

type sqlRows struct {
	rows *sql.Rows
}

func (r *sqlRows) Next() bool {
	if r.rows == nil {
		return false
	}
	return r.rows.Next()
}

func (r *sqlRows) Scan(dest ...any) error {
	if r.rows == nil {
		return errors.New("no rows available")
	}
	return r.rows.Scan(dest...)
}

func (r *sqlRows) Close() error {
	if r.rows == nil {
		return nil
	}
	return r.rows.Close()
}

type sqlResult struct {
	sql.Result
}

type bunTx struct {
	tx *sql.Tx
}

func (t *bunTx) Query(ctx context.Context, query string, args ...any) (interfaces.Rows, error) {
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		return nil, nil
	}
	return &sqlRows{rows: rows}, nil
}

func (t *bunTx) Exec(ctx context.Context, query string, args ...any) (interfaces.Result, error) {
	result, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return sqlResult{Result: result}, nil
}

func (t *bunTx) Transaction(context.Context, func(interfaces.Transaction) error) error {
	return errors.New("nested transactions not supported in stub")
}

func (t *bunTx) Commit() error {
	return t.tx.Commit()
}

func (t *bunTx) Rollback() error {
	return t.tx.Rollback()
}

type NoOpProvider struct{}

func NewNoOpProvider() interfaces.StorageProvider {
	return &NoOpProvider{}
}

func (*NoOpProvider) Query(context.Context, string, ...any) (interfaces.Rows, error) {
	return nil, nil
}

func (*NoOpProvider) Exec(context.Context, string, ...any) (interfaces.Result, error) {
	return sqlResult{}, nil
}

func (*NoOpProvider) Transaction(_ context.Context, fn func(tx interfaces.Transaction) error) error {
	if fn == nil {
		return nil
	}
	return fn(&noopTx{})
}

type noopTx struct{}

func (noopTx) Query(context.Context, string, ...any) (interfaces.Rows, error) {
	return nil, nil
}

func (noopTx) Exec(context.Context, string, ...any) (interfaces.Result, error) {
	return sqlResult{}, nil
}

func (noopTx) Transaction(context.Context, func(interfaces.Transaction) error) error {
	return nil
}

func (noopTx) Commit() error {
	return nil
}

func (noopTx) Rollback() error {
	return nil
}
