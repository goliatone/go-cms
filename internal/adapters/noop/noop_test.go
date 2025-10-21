package noop_test

import (
	"testing"

	"github.com/goliatone/go-cms/internal/adapters/noop"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

func TestAdaptersImplementInterfaces(t *testing.T) {
	var (
		_ interfaces.CacheProvider    = noop.Cache()
		_ interfaces.TemplateRenderer = noop.Template()
		_ interfaces.MediaProvider    = noop.Media()
		_ interfaces.AuthProvider     = noop.Auth()
	)
}
