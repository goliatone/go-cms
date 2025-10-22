package interfaces

import (
	"io"
)

type TemplateRenderer interface {
	Render(name string, data any, out ...io.Writer) (string, error)
	RenderTemplate(name string, data any, out ...io.Writer) (string, error)
	RenderString(templateContent string, data any, out ...io.Writer) (string, error)
	RegisterFilter(name string, fn func(input any, param any) (any, error)) error
	GlobalContext(data any) error
}
