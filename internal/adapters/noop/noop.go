package noop

import (
	"context"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

func Cache() interfaces.CacheProvider {
	return cacheAdapter{}
}

type cacheAdapter struct{}

func (cacheAdapter) Get(context.Context, string) (any, error) {
	return nil, nil
}

func (cacheAdapter) Set(context.Context, string, any, time.Duration) error {
	return nil
}

func (cacheAdapter) Delete(context.Context, string) error {
	return nil
}

func (cacheAdapter) Clear(context.Context) error {
	return nil
}

func Template() interfaces.TemplateRenderer {
	return templateAdapter{}
}

type templateAdapter struct{}

func (templateAdapter) Render(context.Context, string, any) (string, error) {
	return "", nil
}

func (templateAdapter) RegisterFunction(string, any) error {
	return nil
}

func Media() interfaces.MediaProvider {
	return mediaAdapter{}
}

type mediaAdapter struct{}

func (mediaAdapter) GetURL(context.Context, string) (string, error) {
	return "", nil
}

func (mediaAdapter) GetMetadata(context.Context, string) (interfaces.MediaMetadata, error) {
	return interfaces.MediaMetadata{}, nil
}

func Auth() interfaces.AuthProvider {
	return authAdapter{}
}

type authAdapter struct{}

func (authAdapter) CurrentUserID(context.Context) (string, error) {
	return "", nil
}

func (authAdapter) HasPermission(context.Context, string) (bool, error) {
	return false, nil
}
