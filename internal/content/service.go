package content

import "context"

type Service interface {
	Create(ctx context.Context, input Content) (Content, error)
}

type NoOpService struct{}

func NewNoOpService() Service {
	return NoOpService{}
}
func (NoOpService) Create(_ context.Context, input Content) (Content, error) {
	return input, nil
}
