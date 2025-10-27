package commands

import (
	"context"
	"errors"
	"testing"
	"time"

	goerrors "github.com/goliatone/go-errors"
)

type testMessage struct{}

func (testMessage) Type() string { return "cms.test.message" }

func (testMessage) Validate() error { return nil }

type invalidMessage struct{}

func (invalidMessage) Type() string { return "cms.test.invalid" }

func (invalidMessage) Validate() error {
	return validationError()
}

func validationError() error {
	return errors.New("invalid")
}

func TestHandlerExecuteSuccess(t *testing.T) {
	called := false
	h := NewHandler[testMessage](func(ctx context.Context, msg testMessage) error {
		called = true
		return nil
	})

	if err := h.Execute(context.Background(), testMessage{}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !called {
		t.Fatal("expected handler to be invoked")
	}
}

func TestHandlerValidationShortCircuitsExecution(t *testing.T) {
	called := false
	h := NewHandler[invalidMessage](func(ctx context.Context, msg invalidMessage) error {
		called = true
		return nil
	})

	err := h.Execute(context.Background(), invalidMessage{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryValidation) {
		t.Fatalf("expected validation category, got %v", err)
	}
	if called {
		t.Fatal("expected handler not to run when validation fails")
	}
}

func TestHandlerContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	h := NewHandler[testMessage](func(ctx context.Context, msg testMessage) error {
		called = true
		return nil
	})

	err := h.Execute(ctx, testMessage{})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category, got %v", err)
	}
	if called {
		t.Fatal("expected handler not to run when context is cancelled")
	}
}

func TestHandlerWrapsExecutionError(t *testing.T) {
	execErr := errors.New("boom")
	h := NewHandler[testMessage](func(ctx context.Context, msg testMessage) error {
		return execErr
	})

	err := h.Execute(context.Background(), testMessage{})
	if err == nil {
		t.Fatal("expected wrapped execution error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category, got %v", err)
	}
	if !goerrors.HasCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category to propagate, got %v", err)
	}
}

func TestHandlerHonoursTimeoutOption(t *testing.T) {
	h := NewHandler[testMessage](func(ctx context.Context, msg testMessage) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
			return nil
		}
	}, WithTimeout[testMessage](10*time.Millisecond))

	err := h.Execute(context.Background(), testMessage{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category for timeout, got %v", err)
	}
}
