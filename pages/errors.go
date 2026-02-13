package pages

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrContentRequired               = errors.New("pages: content does not exist")
	ErrTemplateRequired              = errors.New("pages: template is required")
	ErrSlugRequired                  = errors.New("pages: slug is required")
	ErrSlugInvalid                   = errors.New("pages: slug contains invalid characters")
	ErrSlugExists                    = errors.New("pages: slug already exists")
	ErrPathExists                    = errors.New("pages: translation path already exists")
	ErrPathConflict                  = errors.New("pages: path conflict")
	ErrUnknownLocale                 = errors.New("pages: unknown locale")
	ErrInvalidLocale                 = errors.New("pages: invalid locale")
	ErrSourceNotFound                = errors.New("pages: source not found")
	ErrTranslationAlreadyExists      = errors.New("pages: translation already exists")
	ErrTranslationInvariantViolation = errors.New("pages: translation invariant violation")
	ErrDuplicateLocale               = errors.New("pages: duplicate locale provided")
	ErrParentNotFound                = errors.New("pages: parent page not found")
	ErrNoPageTranslations            = errors.New("pages: at least one translation is required")
	ErrDefaultLocaleRequired         = errors.New("pages: default locale translation is required")
	ErrTemplateUnknown               = errors.New("pages: template not found")
	ErrPageRequired                  = errors.New("pages: page id required")
	ErrVersioningDisabled            = errors.New("pages: versioning feature disabled")
	ErrPageVersionRequired           = errors.New("pages: version identifier required")
	ErrVersionAlreadyPublished       = errors.New("pages: version already published")
	ErrVersionRetentionExceeded      = errors.New("pages: version retention limit reached")
	ErrVersionConflict               = errors.New("pages: base version mismatch")
	ErrSchedulingDisabled            = errors.New("pages: scheduling feature disabled")
	ErrScheduleWindowInvalid         = errors.New("pages: publish_at must be before unpublish_at")
	ErrScheduleTimestampInvalid      = errors.New("pages: schedule timestamp is invalid")
	ErrPageMediaReferenceRequired    = errors.New("pages: media reference requires id or path")
	ErrPageSoftDeleteUnsupported     = errors.New("pages: soft delete not supported")
	ErrPageTranslationsDisabled      = errors.New("pages: translations feature disabled")
	ErrPageTranslationNotFound       = errors.New("pages: translation not found")
	ErrPageParentCycle               = errors.New("pages: parent assignment creates hierarchy cycle")
	ErrPageDuplicateSlug             = errors.New("pages: unable to determine unique duplicate slug")
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

// PathConflictError captures path conflicts surfaced by create-translation commands.
type PathConflictError struct {
	EntityID     uuid.UUID
	SourceLocale string
	TargetLocale string
	Environment  string
	Path         string
}

func (e *PathConflictError) Error() string {
	if e == nil {
		return ErrPathConflict.Error()
	}
	path := strings.TrimSpace(e.Path)
	if path != "" {
		return fmt.Sprintf("%s: path=%s", ErrPathConflict.Error(), path)
	}
	return ErrPathConflict.Error()
}

func (e *PathConflictError) Unwrap() error {
	return ErrPathConflict
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
