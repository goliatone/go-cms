package markdown

import (
	internal "github.com/goliatone/go-cms/internal/markdown"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

type (
	Config        = internal.Config
	Service       = internal.Service
	ServiceOption = internal.ServiceOption
)

func NewService(cfg Config, parser interfaces.MarkdownParser, opts ...ServiceOption) (*Service, error) {
	return internal.NewService(cfg, parser, opts...)
}

func NewGoldmarkParser(opts interfaces.ParseOptions) interfaces.MarkdownParser {
	return internal.NewGoldmarkParser(opts)
}

func WithContentService(svc interfaces.ContentService) ServiceOption {
	return internal.WithContentService(svc)
}

func WithLogger(logger interfaces.Logger) ServiceOption {
	return internal.WithLogger(logger)
}

func WithShortcodeService(svc interfaces.ShortcodeService) ServiceOption {
	return internal.WithShortcodeService(svc)
}
