package schema

import "errors"

var (
	ErrUnsupportedKeyword   = errors.New("schema: unsupported keyword")
	ErrOverlayPathNotFound  = errors.New("schema: overlay path not found")
	ErrInvalidSchemaVersion = errors.New("schema: invalid schema version")
)
