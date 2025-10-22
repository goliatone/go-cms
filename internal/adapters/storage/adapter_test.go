package storage_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/goliatone/go-cms/internal/adapters/storage"
	"github.com/goliatone/go-cms/pkg/interfaces"
	_ "github.com/mattn/go-sqlite3"
)

type sqliteExecutor struct {
	db *sql.DB
}

func newSQLiteExecutor(t *testing.T) *sqliteExecutor {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return &sqliteExecutor{db: db}
}

func (s *sqliteExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

func (s *sqliteExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

func (s *sqliteExecutor) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, opts)
}

func TestProvidersImplementInterface(t *testing.T) {
	executor := newSQLiteExecutor(t)

	var (
		_ interfaces.StorageProvider = storage.NewBunStorageAdapter(executor)
		_ interfaces.StorageProvider = storage.NewNoOpProvider()
	)

	err := storage.NewBunStorageAdapter(executor).Transaction(context.Background(), func(tx interfaces.Transaction) error {
		_, err := tx.Exec(context.Background(), "SELECT 1")
		return err
	})
	if err != nil {
		t.Fatalf("unexpected transaction error: %v", err)
	}
}
