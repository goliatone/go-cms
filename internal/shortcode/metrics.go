package shortcode

import (
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// NoOpMetrics returns a metrics recorder that drops every observation.
func NoOpMetrics() interfaces.ShortcodeMetrics {
	return noopMetrics{}
}

type noopMetrics struct{}

func (noopMetrics) ObserveRenderDuration(string, time.Duration) {}

func (noopMetrics) IncrementRenderError(string) {}

func (noopMetrics) IncrementCacheHit(string) {}
