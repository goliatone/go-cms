package i18n

import "context"

type Loader struct{}

func NewLoader() *Loader {
	return &Loader{}
}

func (l *Loader) Load(_ context.Context) error {
	_ = l
	return nil
}
