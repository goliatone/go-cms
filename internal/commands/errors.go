package commands

import (
	"context"

	goerrors "github.com/goliatone/go-errors"
)

const (
	commandValidationCode   = "COMMAND_VALIDATION_FAILED"
	commandContextCanceled  = "COMMAND_CONTEXT_CANCELED"
	commandContextTimeout   = "COMMAND_CONTEXT_TIMEOUT"
	commandContextErrorCode = "COMMAND_CONTEXT_ERROR"
	commandExecuteFailed    = "COMMAND_EXECUTION_FAILED"
)

// WrapValidationError ensures validation failures carry a consistent category and code.
func WrapValidationError(err error) error {
	if err == nil {
		return nil
	}
	if goerrors.IsWrapped(err) {
		return err
	}
	return goerrors.Wrap(err, goerrors.CategoryValidation, "command validation failed").
		WithTextCode(commandValidationCode)
}

// WrapContextError normalises context cancellation and timeout errors.
func WrapContextError(err error) error {
	if err == nil {
		return nil
	}
	if goerrors.IsWrapped(err) {
		return err
	}
	switch err {
	case context.Canceled:
		return goerrors.Wrap(err, goerrors.CategoryCommand, "command execution cancelled").
			WithTextCode(commandContextCanceled)
	case context.DeadlineExceeded:
		return goerrors.Wrap(err, goerrors.CategoryCommand, "command execution deadline exceeded").
			WithTextCode(commandContextTimeout)
	default:
		return goerrors.Wrap(err, goerrors.CategoryCommand, "command context error").
			WithTextCode(commandContextErrorCode)
	}
}

// WrapExecuteError attaches command execution metadata to returned errors.
func WrapExecuteError(err error) error {
	if err == nil {
		return nil
	}
	if goerrors.IsWrapped(err) {
		return err
	}
	return goerrors.Wrap(err, goerrors.CategoryCommand, "command execution failed").
		WithTextCode(commandExecuteFailed)
}
