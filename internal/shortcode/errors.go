package shortcode

import "errors"

var (
	// ErrDuplicateDefinition indicates an attempt to register a shortcode name twice.
	ErrDuplicateDefinition = errors.New("shortcode: duplicate definition")
	// ErrInvalidDefinition occurs when a definition fails schema validation.
	ErrInvalidDefinition = errors.New("shortcode: invalid definition")
)
