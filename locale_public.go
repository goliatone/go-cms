package cms

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/goliatone/go-cms/content"
	internalcontent "github.com/goliatone/go-cms/internal/content"
	"github.com/google/uuid"
)

var (
	// ErrLocaleCodeRequired indicates locale lookups require a non-empty locale code.
	ErrLocaleCodeRequired = errors.New("cms: locale code is required")
	// ErrUnknownLocale indicates locale lookup failed because the locale code is unknown.
	ErrUnknownLocale = content.ErrUnknownLocale
)

// LocaleNotFoundError describes unknown locale-code lookups and unwraps to ErrUnknownLocale.
type LocaleNotFoundError struct {
	Code string
}

func (e *LocaleNotFoundError) Error() string {
	code := strings.TrimSpace(e.Code)
	if code == "" {
		return "cms: locale not found"
	}
	return fmt.Sprintf("cms: locale %q not found", code)
}

func (e *LocaleNotFoundError) Unwrap() error {
	return ErrUnknownLocale
}

// LocaleInfo is the stable public locale view exposed by cms.
type LocaleInfo struct {
	ID         uuid.UUID
	Code       string
	Display    string
	NativeName *string
	IsActive   bool
	IsDefault  bool
	Metadata   map[string]any
}

// LocaleService resolves locale records through the public cms contract.
type LocaleService interface {
	ResolveByCode(ctx context.Context, code string) (LocaleInfo, error)
}

type localeService struct {
	module *Module
}

func newLocaleService(m *Module) LocaleService {
	return &localeService{module: m}
}

func (s *localeService) ResolveByCode(ctx context.Context, code string) (LocaleInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil {
		return LocaleInfo{}, errNilModule
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return LocaleInfo{}, ErrLocaleCodeRequired
	}

	repo := s.module.container.LocaleRepository()
	if repo == nil {
		return LocaleInfo{}, errNilModule
	}

	locale, err := repo.GetByCode(ctx, code)
	if err != nil {
		var notFound *internalcontent.NotFoundError
		if errors.As(err, &notFound) {
			return LocaleInfo{}, &LocaleNotFoundError{Code: code}
		}
		return LocaleInfo{}, err
	}
	if locale == nil {
		return LocaleInfo{}, &LocaleNotFoundError{Code: code}
	}

	return LocaleInfo{
		ID:         locale.ID,
		Code:       locale.Code,
		Display:    locale.Display,
		NativeName: cloneStringPtr(locale.NativeName),
		IsActive:   locale.IsActive,
		IsDefault:  locale.IsDefault,
		Metadata:   cloneMap(locale.Metadata),
	}, nil
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func cloneStringPtr(src *string) *string {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}
