package i18n

import (
	"github.com/goliatone/go-cms/pkg/interfaces"
)

type Service interface {
	interfaces.Service
}

type NoOpService struct{}

func NewNoOpService() Service {
	return NoOpService{}
}

func (NoOpService) Translator() interfaces.Translator {
	return noopTranslator{}
}

func (NoOpService) Culture() interfaces.CultureService {
	return nil
}

func (NoOpService) TemplateHelpers(_ interfaces.HelperConfig) map[string]any {
	return map[string]any{}
}

func (NoOpService) DefaultLocale() string {
	return ""
}

type noopTranslator struct{}

func (noopTranslator) Translate(_ string, key string, _ ...any) (string, error) {
	return key, nil
}
