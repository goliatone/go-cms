package cms

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/goliatone/go-cms/content"
	internalcontent "github.com/goliatone/go-cms/internal/content"
	sharedi18n "github.com/goliatone/go-i18n"
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

// ActiveLocaleService exposes the active locale catalog when the underlying
// repository supports listing locale records.
type ActiveLocaleService interface {
	ActiveLocales(ctx context.Context) ([]LocaleInfo, error)
}

type localeService struct {
	module *Module
}

func newLocaleService(m *Module) LocaleService {
	return &localeService{module: m}
}

type localeListRepository interface {
	List(ctx context.Context) ([]*internalcontent.Locale, error)
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

	return localeInfoFromRecord(locale), nil
}

func (s *localeService) ActiveLocales(ctx context.Context) ([]LocaleInfo, error) {
	if s == nil || s.module == nil || s.module.container == nil {
		return nil, errNilModule
	}

	repo := s.module.container.LocaleRepository()
	if repo == nil {
		return nil, errNilModule
	}

	lister, ok := repo.(localeListRepository)
	if !ok || lister == nil {
		return nil, fmt.Errorf("cms: locale repository does not support listing")
	}

	records, err := lister.List(ctx)
	if err != nil {
		return nil, err
	}

	locales := make([]LocaleInfo, 0, len(records))
	for _, locale := range records {
		if locale == nil || !locale.IsActive {
			continue
		}
		locales = append(locales, localeInfoFromRecord(locale))
	}
	return locales, nil
}

func localeInfoFromRecord(locale *internalcontent.Locale) LocaleInfo {
	if locale == nil {
		return LocaleInfo{}
	}
	return LocaleInfo{
		ID:         locale.ID,
		Code:       sharedi18n.NormalizeLocale(locale.Code),
		Display:    locale.Display,
		NativeName: cloneStringPtr(locale.NativeName),
		IsActive:   locale.IsActive,
		IsDefault:  locale.IsDefault,
		Metadata:   cloneMap(locale.Metadata),
	}
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	maps.Copy(out, src)
	return out
}

func cloneStringPtr(src *string) *string {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}
