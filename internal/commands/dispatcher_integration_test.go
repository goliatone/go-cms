package commands

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-command/dispatcher"
	"github.com/goliatone/go-command/runner"
)

type dispatcherTestCommand struct {
	ID string
}

func (dispatcherTestCommand) Type() string { return "cms.test.dispatcher" }

func (dispatcherTestCommand) Validate() error { return nil }

func TestDispatcherRetriesUntilSuccess(t *testing.T) {
	t.Parallel()

	var attempts int
	handler := NewHandler(func(ctx context.Context, _ dispatcherTestCommand) error {
		attempts++
		if attempts == 1 {
			return errors.New("transient failure")
		}
		return nil
	}, WithTimeout[dispatcherTestCommand](time.Second))

	sub := dispatcher.SubscribeCommand(handler, runner.WithMaxRetries(1))
	t.Cleanup(sub.Unsubscribe)

	if err := dispatcher.Dispatch(context.Background(), dispatcherTestCommand{ID: "abc"}); err != nil {
		t.Fatalf("dispatch: expected success after retry, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts (initial + retry), got %d", attempts)
	}
}

func TestDispatcherRetryExhaustionPropagatesError(t *testing.T) {
	t.Parallel()

	var attempts int
	handler := NewHandler(func(ctx context.Context, _ dispatcherTestCommand) error {
		attempts++
		return errors.New("permanent failure")
	}, WithTimeout[dispatcherTestCommand](time.Second))

	sub := dispatcher.SubscribeCommand(handler, runner.WithMaxRetries(2))
	t.Cleanup(sub.Unsubscribe)

	err := dispatcher.Dispatch(context.Background(), dispatcherTestCommand{ID: "xyz"})
	if err == nil {
		t.Fatal("expected dispatcher to return error after exhausting retries")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts (initial + 2 retries), got %d", attempts)
	}
}
