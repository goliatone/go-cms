package i18n

import "context"

type Service interface {
	Translate(ctx context.Context, key string) (string, error)
}

type NoOpService struct{}

func NewNoOpService() Service {
	return NoOpService{}
}

func (NoOpService) Translate(_ context.Context, key string) (string, error) {
	return key, nil
}
