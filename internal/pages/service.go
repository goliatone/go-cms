package pages

import "context"

type Service interface {
	Create(ctx context.Context, input Page) (Page, error)
}

type NoOpService struct{}

func NewNoOpService() Service {
	return NoOpService{}
}

func (NoOpService) Create(_ context.Context, p Page) (Page, error) {
	return p, nil
}
