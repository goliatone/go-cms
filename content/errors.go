package content

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrContentTypeRequired                   = errors.New("content: content type does not exist")
	ErrSlugRequired                          = errors.New("content: slug is required")
	ErrSlugInvalid                           = errors.New("content: slug contains invalid characters")
	ErrSlugExists                            = errors.New("content: slug already exists")
	ErrSlugConflict                          = errors.New("content: slug conflict")
	ErrNoTranslations                        = errors.New("content: at least one translation is required")
	ErrDefaultLocaleRequired                 = errors.New("content: default locale translation is required")
	ErrDuplicateLocale                       = errors.New("content: duplicate locale provided")
	ErrUnknownLocale                         = errors.New("content: unknown locale")
	ErrInvalidLocale                         = errors.New("content: invalid locale")
	ErrSourceNotFound                        = errors.New("content: source not found")
	ErrTranslationAlreadyExists              = errors.New("content: translation already exists")
	ErrTranslationInvariantViolation         = errors.New("content: translation invariant violation")
	ErrContentSchemaInvalid                  = errors.New("content: schema validation failed")
	ErrContentSoftDeleteUnsupported          = errors.New("content: soft delete not supported")
	ErrContentIDRequired                     = errors.New("content: content id required")
	ErrContentMetadataInvalid                = errors.New("content: metadata invalid")
	ErrVersioningDisabled                    = errors.New("content: versioning feature disabled")
	ErrContentVersionRequired                = errors.New("content: version identifier required")
	ErrContentVersionConflict                = errors.New("content: base version mismatch")
	ErrContentVersionAlreadyPublished        = errors.New("content: version already published")
	ErrContentVersionRetentionExceeded       = errors.New("content: version retention limit reached")
	ErrSchedulingDisabled                    = errors.New("content: scheduling feature disabled")
	ErrScheduleWindowInvalid                 = errors.New("content: publish_at must be before unpublish_at")
	ErrScheduleTimestampInvalid              = errors.New("content: schedule timestamp is invalid")
	ErrContentTranslationsDisabled           = errors.New("content: translations feature disabled")
	ErrContentTranslationNotFound            = errors.New("content: translation not found")
	ErrContentSchemaMigrationRequired        = errors.New("content: schema migration required")
	ErrContentTranslationLookupUnsupported   = errors.New("content: translation lookup unsupported")
	ErrContentProjectionModeInvalid          = errors.New("content: projection translation mode is invalid")
	ErrContentProjectionUnsupported          = errors.New("content: projection is not supported")
	ErrContentProjectionRequiresTranslations = errors.New("content: projection requires translations")
	ErrEmbeddedBlocksResolverMissing         = errors.New("content: embedded blocks resolver not configured")

	ErrContentTypeNameRequired   = errors.New("content type: name is required")
	ErrContentTypeSchemaRequired = errors.New("content type: schema is required")
	ErrContentTypeSchemaInvalid  = errors.New("content type: schema is invalid")
	ErrContentTypeIDRequired     = errors.New("content type: id required")
	ErrContentTypeSlugInvalid    = errors.New("content type: slug contains invalid characters")
	ErrContentTypeSchemaVersion  = errors.New("content type: schema version invalid")
	ErrContentTypeSchemaBreaking = errors.New("content type: schema has breaking changes")
	ErrContentTypeStatusInvalid  = errors.New("content type: status invalid")
	ErrContentTypeStatusChange   = errors.New("content type: status transition invalid")
)

// TranslationAlreadyExistsError captures duplicate translation conflicts.
type TranslationAlreadyExistsError struct {
	EntityID           uuid.UUID
	SourceLocale       string
	TargetLocale       string
	TranslationGroupID *uuid.UUID
	ExistingID         uuid.UUID
	Environment        string
}

func (e *TranslationAlreadyExistsError) Error() string {
	if e == nil {
		return ErrTranslationAlreadyExists.Error()
	}
	target := strings.TrimSpace(e.TargetLocale)
	if target != "" {
		return fmt.Sprintf("%s: locale=%s", ErrTranslationAlreadyExists.Error(), target)
	}
	return ErrTranslationAlreadyExists.Error()
}

func (e *TranslationAlreadyExistsError) Unwrap() error {
	return ErrTranslationAlreadyExists
}

// InvalidLocaleError captures invalid locale inputs for translation operations.
type InvalidLocaleError struct {
	EntityID     uuid.UUID
	SourceLocale string
	TargetLocale string
	Environment  string
}

func (e *InvalidLocaleError) Error() string {
	if e == nil {
		return ErrInvalidLocale.Error()
	}
	target := strings.TrimSpace(e.TargetLocale)
	if target != "" {
		return fmt.Sprintf("%s: locale=%s", ErrInvalidLocale.Error(), target)
	}
	return ErrInvalidLocale.Error()
}

func (e *InvalidLocaleError) Unwrap() error {
	return ErrInvalidLocale
}

// SourceNotFoundError captures missing source entity lookups for translation creation.
type SourceNotFoundError struct {
	EntityID    uuid.UUID
	Environment string
}

func (e *SourceNotFoundError) Error() string {
	if e == nil {
		return ErrSourceNotFound.Error()
	}
	if e.EntityID != uuid.Nil {
		return fmt.Sprintf("%s: id=%s", ErrSourceNotFound.Error(), e.EntityID.String())
	}
	return ErrSourceNotFound.Error()
}

func (e *SourceNotFoundError) Unwrap() error {
	return ErrSourceNotFound
}

// SlugConflictError captures non-translation slug conflicts surfaced by the create-translation command.
type SlugConflictError struct {
	EntityID     uuid.UUID
	SourceLocale string
	TargetLocale string
	Environment  string
	Slug         string
}

func (e *SlugConflictError) Error() string {
	if e == nil {
		return ErrSlugConflict.Error()
	}
	slug := strings.TrimSpace(e.Slug)
	if slug != "" {
		return fmt.Sprintf("%s: slug=%s", ErrSlugConflict.Error(), slug)
	}
	return ErrSlugConflict.Error()
}

func (e *SlugConflictError) Unwrap() error {
	return ErrSlugConflict
}

// TranslationInvariantViolationError captures invariant failures in translation grouping operations.
type TranslationInvariantViolationError struct {
	EntityID           uuid.UUID
	SourceLocale       string
	TargetLocale       string
	TranslationGroupID *uuid.UUID
	Environment        string
	Message            string
}

func (e *TranslationInvariantViolationError) Error() string {
	if e == nil {
		return ErrTranslationInvariantViolation.Error()
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		return ErrTranslationInvariantViolation.Error()
	}
	return fmt.Sprintf("%s: %s", ErrTranslationInvariantViolation.Error(), message)
}

func (e *TranslationInvariantViolationError) Unwrap() error {
	return ErrTranslationInvariantViolation
}
