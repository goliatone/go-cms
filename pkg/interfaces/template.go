package interfaces

import "context"

type TemplateRenderer interface {
	Render(ctx context.Context, template string, dada any) (string, error)
	RegisterFunction(name string, fn any) error
}
