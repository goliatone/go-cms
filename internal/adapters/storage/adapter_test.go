package storage_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/goliatone/go-cms/internal/adapters/storage"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

type stubExecutor struct{}

func (stubExecutor) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return stubResult{}, nil
}

func (stubExecutor) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}

func (stubExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	return &sql.Tx{}, nil
}

type stubResult struct{}

func (stubResult) LastInsertId() (int64, error) { return 0, nil }
func (stubResult) RowsAffected() (int64, error) { return 0, nil }

func TestProvidersImplementInterface(t *testing.T) {
	var (
		_ interfaces.StorageProvider = storage.NewBunStorageAdapter(stubExecutor{})
		_ interfaces.StorageProvider = storage.NewNoOpProvider()
	)

	if err := storage.NewBunStorageAdapter(stubExecutor{}).Transaction(context.Background(), func(tx interfaces.Transaction) error {
		_, err := tx.Exec(context.Background(), "SELECT 1")
		return err
	}); err != nil {
		t.Fatalf("unexpected transaction error: %v", err)
	}
}
