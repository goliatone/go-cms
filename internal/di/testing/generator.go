package ditesting

import (
	"context"
	"errors"
	"sync"

	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

// MemoryStorage records generator storage interactions for assertions in tests.
type MemoryStorage struct {
	mu         sync.Mutex
	execCalls  []ExecCall
	queryCalls []QueryCall
}

// ExecCall captures an Exec invocation against the memory storage.
type ExecCall struct {
	Query         string
	Args          []any
	InTransaction bool
}

// QueryCall captures a Query invocation against the memory storage.
type QueryCall struct {
	Query         string
	Args          []any
	InTransaction bool
}

// NewMemoryStorage constructs a new in-memory storage adapter.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

// Exec records the executed statement.
func (m *MemoryStorage) Exec(_ context.Context, query string, args ...any) (interfaces.Result, error) {
	m.recordExec(query, false, args)
	return memoryResult{}, nil
}

// Query records the query and returns a stateless rows iterator.
func (m *MemoryStorage) Query(_ context.Context, query string, args ...any) (interfaces.Rows, error) {
	m.recordQuery(query, false, args)
	return memoryRows{}, nil
}

// Transaction executes the provided function with a transactional view of the storage.
func (m *MemoryStorage) Transaction(_ context.Context, fn func(tx interfaces.Transaction) error) error {
	if fn == nil {
		return nil
	}
	tx := &memoryTx{storage: m}
	if err := fn(tx); err != nil {
		tx.rollback = true
		return err
	}
	tx.commit = true
	return nil
}

// ExecCalls returns a copy of recorded Exec calls.
func (m *MemoryStorage) ExecCalls() []ExecCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	calls := make([]ExecCall, len(m.execCalls))
	copy(calls, m.execCalls)
	return calls
}

// QueryCalls returns a copy of recorded Query calls.
func (m *MemoryStorage) QueryCalls() []QueryCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	calls := make([]QueryCall, len(m.queryCalls))
	copy(calls, m.queryCalls)
	return calls
}

func (m *MemoryStorage) recordExec(query string, inTx bool, args []any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := append([]any(nil), args...)
	m.execCalls = append(m.execCalls, ExecCall{
		Query:         query,
		Args:          cloned,
		InTransaction: inTx,
	})
}

func (m *MemoryStorage) recordQuery(query string, inTx bool, args []any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := append([]any(nil), args...)
	m.queryCalls = append(m.queryCalls, QueryCall{
		Query:         query,
		Args:          cloned,
		InTransaction: inTx,
	})
}

type memoryRows struct{}

func (memoryRows) Next() bool {
	return false
}

func (memoryRows) Scan(dest ...any) error {
	return errors.New("memory storage: no rows available")
}

func (memoryRows) Close() error {
	return nil
}

type memoryResult struct{}

func (memoryResult) RowsAffected() (int64, error) {
	return 0, nil
}

func (memoryResult) LastInsertId() (int64, error) {
	return 0, nil
}

type memoryTx struct {
	storage  *MemoryStorage
	commit   bool
	rollback bool
}

func (tx *memoryTx) Query(ctx context.Context, query string, args ...any) (interfaces.Rows, error) {
	tx.storage.recordQuery(query, true, args)
	return memoryRows{}, nil
}

func (tx *memoryTx) Exec(ctx context.Context, query string, args ...any) (interfaces.Result, error) {
	tx.storage.recordExec(query, true, args)
	return memoryResult{}, nil
}

func (tx *memoryTx) Transaction(context.Context, func(interfaces.Transaction) error) error {
	return errors.New("memory storage: nested transactions not supported")
}

func (tx *memoryTx) Commit() error {
	tx.commit = true
	return nil
}

func (tx *memoryTx) Rollback() error {
	tx.rollback = true
	return nil
}

// NewGeneratorContainer creates a DI container configured with memory-based generator storage.
func NewGeneratorContainer(cfg runtimeconfig.Config, opts ...di.Option) (*di.Container, *MemoryStorage, error) {
	memStorage := NewMemoryStorage()
	options := append([]di.Option{di.WithGeneratorStorage(memStorage)}, opts...)

	container, err := di.NewContainer(cfg, options...)
	if err != nil {
		return nil, nil, err
	}
	return container, memStorage, nil
}
