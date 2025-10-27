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

func wrapValidationError(err error) error {
	if err == nil {
		return nil
	}
	if goerrors.IsWrapped(err) {
		return err
	}
	return goerrors.Wrap(err, goerrors.CategoryValidation, "command validation failed").
		WithTextCode(commandValidationCode)
}

func wrapContextError(err error) error {
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

func wrapExecuteError(err error) error {
	if err == nil {
		return nil
	}
	if goerrors.IsWrapped(err) {
		return err
	}
	return goerrors.Wrap(err, goerrors.CategoryCommand, "command execution failed").
		WithTextCode(commandExecuteFailed)
}
