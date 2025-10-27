package mediacmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/pkg/interfaces"
	goerrors "github.com/goliatone/go-errors"
)

type stubMediaService struct {
	resolveCalls   []resolveInvocation
	invalidateArgs []media.BindingSet
	resolveErr     error
	invalidateErr  error
}

type resolveInvocation struct {
	bindings media.BindingSet
	options  media.ResolveOptions
}

func (s *stubMediaService) ResolveBindings(ctx context.Context, bindings media.BindingSet, opts media.ResolveOptions) (map[string][]*media.Attachment, error) {
	s.resolveCalls = append(s.resolveCalls, resolveInvocation{
		bindings: media.CloneBindingSet(bindings),
		options:  opts,
	})
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}
	return map[string][]*media.Attachment{}, nil
}

func (s *stubMediaService) Invalidate(ctx context.Context, bindings media.BindingSet) error {
	s.invalidateArgs = append(s.invalidateArgs, media.CloneBindingSet(bindings))
	if s.invalidateErr != nil {
		return s.invalidateErr
	}
	return nil
}

func TestImportAssetsHandlerResolvesBindings(t *testing.T) {
	service := &stubMediaService{}
	logger := commands.CommandLogger(nil, "media")
	handler := NewImportAssetsHandler(service, logger, FeatureGates{
		MediaLibraryEnabled: func() bool { return true },
	})

	bindings := media.BindingSet{
		"hero": {
			{
				Slot: "primary",
				Reference: interfaces.MediaReference{
					ID: "asset-1",
				},
			},
		},
	}
	signedTTL := 120
	cacheTTL := 300
	cmd := ImportAssetsCommand{
		Bindings:            bindings,
		IncludeSignedURLs:   true,
		SignedURLTTLSeconds: &signedTTL,
		CacheTTLSeconds:     &cacheTTL,
	}

	if err := handler.Execute(context.Background(), cmd); err != nil {
		t.Fatalf("execute import: %v", err)
	}
	if len(service.resolveCalls) != 1 {
		t.Fatalf("expected resolve call, got %d", len(service.resolveCalls))
	}
	call := service.resolveCalls[0]
	if _, ok := call.bindings["hero"]; !ok {
		t.Fatalf("expected hero binding recorded, got %#v", call.bindings)
	}
	if !call.options.IncludeSignedURLs {
		t.Fatalf("expected include signed URLs")
	}
	if call.options.SignedURLTTL != 120*time.Second {
		t.Fatalf("expected signed ttl 120s, got %v", call.options.SignedURLTTL)
	}
	if call.options.CacheTTL != 300*time.Second {
		t.Fatalf("expected cache ttl 300s, got %v", call.options.CacheTTL)
	}
}

func TestImportAssetsHandlerFeatureDisabled(t *testing.T) {
	service := &stubMediaService{}
	handler := NewImportAssetsHandler(service, logging.NoOp(), FeatureGates{
		MediaLibraryEnabled: func() bool { return false },
	})

	err := handler.Execute(context.Background(), ImportAssetsCommand{
		Bindings: media.BindingSet{
			"default": {
				{
					Slot: "a",
					Reference: interfaces.MediaReference{
						ID: "asset-2",
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error when media library disabled")
	}
	if !errors.Is(err, media.ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable, got %v", err)
	}
	if len(service.resolveCalls) != 0 {
		t.Fatalf("expected no resolve calls, got %d", len(service.resolveCalls))
	}
}

func TestImportAssetsHandlerValidationError(t *testing.T) {
	service := &stubMediaService{}
	handler := NewImportAssetsHandler(service, logging.NoOp(), FeatureGates{
		MediaLibraryEnabled: func() bool { return true },
	})

	err := handler.Execute(context.Background(), ImportAssetsCommand{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryValidation) {
		t.Fatalf("expected validation category, got %v", err)
	}
	if len(service.resolveCalls) != 0 {
		t.Fatalf("expected no resolve calls, got %d", len(service.resolveCalls))
	}
}

func TestCleanupAssetsHandlerInvalidates(t *testing.T) {
	service := &stubMediaService{}
	handler := NewCleanupAssetsHandler(service, logging.NoOp(), FeatureGates{
		MediaLibraryEnabled: func() bool { return true },
	})

	cmd := CleanupAssetsCommand{
		Bindings: media.BindingSet{
			"gallery": {
				{
					Slot: "thumb",
					Reference: interfaces.MediaReference{
						Path: "/assets/thumb.jpg",
					},
				},
			},
		},
	}

	if err := handler.Execute(context.Background(), cmd); err != nil {
		t.Fatalf("execute cleanup: %v", err)
	}
	if len(service.invalidateArgs) != 1 {
		t.Fatalf("expected invalidate call, got %d", len(service.invalidateArgs))
	}
	if _, ok := service.invalidateArgs[0]["gallery"]; !ok {
		t.Fatalf("expected gallery binding recorded, got %#v", service.invalidateArgs[0])
	}
}

func TestCleanupAssetsHandlerDryRun(t *testing.T) {
	service := &stubMediaService{}
	handler := NewCleanupAssetsHandler(service, logging.NoOp(), FeatureGates{
		MediaLibraryEnabled: func() bool { return true },
	})

	err := handler.Execute(context.Background(), CleanupAssetsCommand{
		Bindings: media.BindingSet{
			"default": {
				{
					Slot: "logo",
					Reference: interfaces.MediaReference{
						ID: "asset-3",
					},
				},
			},
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("execute cleanup dry run: %v", err)
	}
	if len(service.invalidateArgs) != 0 {
		t.Fatalf("expected no invalidate calls, got %d", len(service.invalidateArgs))
	}
}

func TestCleanupAssetsHandlerFeatureDisabled(t *testing.T) {
	service := &stubMediaService{}
	handler := NewCleanupAssetsHandler(service, logging.NoOp(), FeatureGates{
		MediaLibraryEnabled: func() bool { return false },
	})

	err := handler.Execute(context.Background(), CleanupAssetsCommand{
		Bindings: media.BindingSet{
			"default": {
				{
					Slot: "logo",
					Reference: interfaces.MediaReference{
						ID: "asset-3",
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected feature disabled error")
	}
	if !errors.Is(err, media.ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable, got %v", err)
	}
}
