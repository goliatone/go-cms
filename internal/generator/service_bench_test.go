package generator

import (
	"context"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/logging"
)

func BenchmarkBuildSequential(b *testing.B) {
	benchmarkBuild(b, 1, false)
}

func BenchmarkBuildConcurrentWithAssets(b *testing.B) {
	benchmarkBuild(b, 4, true)
}

func benchmarkBuild(b *testing.B, workers int, includeAssets bool) {
	ctx := context.Background()
	now := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)

	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		fixtures := newRenderFixtures(now)
		fixtures.Config.Workers = workers

		renderer := &recordingRenderer{}
		storage := &recordingStorage{}
		deps := Dependencies{
			Pages:    fixtures.Pages,
			Content:  fixtures.Content,
			Menus:    fixtures.Menus,
			Themes:   fixtures.Themes,
			Locales:  fixtures.Locales,
			Renderer: renderer,
			Storage:  storage,
			Logger:   logging.NoOp(),
		}
		if includeAssets {
			deps.Assets = newStubAssetResolver()
		}
		svc := NewService(fixtures.Config, deps).(*service)
		svc.now = func() time.Time { return now }

		b.StartTimer()
		_, err := svc.Build(ctx, BuildOptions{})
		b.StopTimer()
		if err != nil {
			b.Fatalf("benchmark build: %v", err)
		}
	}
}
