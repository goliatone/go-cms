package mediacmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

const importAssetsMessageType = "cms.media.asset.import"

// ImportAssetsCommand resolves media bindings to warm caches or prefetch assets.
type ImportAssetsCommand struct {
	Bindings            media.BindingSet `json:"bindings"`
	IncludeSignedURLs   bool             `json:"include_signed_urls,omitempty"`
	SignedURLTTLSeconds *int             `json:"signed_url_ttl_seconds,omitempty"`
	CacheTTLSeconds     *int             `json:"cache_ttl_seconds,omitempty"`
}

// Type implements command.Message.
func (ImportAssetsCommand) Type() string { return importAssetsMessageType }

// Validate ensures the command payload contains at least one binding with a valid reference.
func (m ImportAssetsCommand) Validate() error {
	errs := validation.Errors{}
	if len(m.Bindings) == 0 {
		errs["bindings"] = validation.NewError("cms.media.asset.import.bindings_required", "bindings must include at least one media reference")
	} else if refErr := validateBindingSet(m.Bindings); refErr != nil {
		errs["bindings"] = refErr
	}
	if ttl := m.SignedURLTTLSeconds; ttl != nil && *ttl < 0 {
		errs["signed_url_ttl_seconds"] = validation.NewError("cms.media.asset.import.signed_url_ttl_invalid", "signed_url_ttl_seconds must be zero or positive")
	}
	if ttl := m.CacheTTLSeconds; ttl != nil && *ttl < 0 {
		errs["cache_ttl_seconds"] = validation.NewError("cms.media.asset.import.cache_ttl_invalid", "cache_ttl_seconds must be zero or positive")
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// ImportAssetsHandler resolves media bindings using the shared command handler foundation.
type ImportAssetsHandler struct {
	inner *commands.Handler[ImportAssetsCommand]
}

// NewImportAssetsHandler constructs a handler wired to the provided media service.
func NewImportAssetsHandler(service media.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[ImportAssetsCommand]) *ImportAssetsHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg ImportAssetsCommand) error {
		if !gates.mediaLibraryEnabled() {
			return media.ErrProviderUnavailable
		}

		options := media.ResolveOptions{
			IncludeSignedURLs: msg.IncludeSignedURLs,
		}
		if msg.SignedURLTTLSeconds != nil {
			options.SignedURLTTL = time.Duration(*msg.SignedURLTTLSeconds) * time.Second
		}
		if msg.CacheTTLSeconds != nil {
			options.CacheTTL = time.Duration(*msg.CacheTTLSeconds) * time.Second
		}

		attachments, err := service.ResolveBindings(ctx, msg.Bindings, options)
		if err != nil {
			return err
		}

		if len(attachments) > 0 {
			logging.WithFields(baseLogger, map[string]any{
				"binding_sets": len(attachments),
			}).Debug("media.command.assets_resolved")
		}
		return nil
	}

	handlerOpts := []commands.HandlerOption[ImportAssetsCommand]{
		commands.WithLogger[ImportAssetsCommand](baseLogger),
		commands.WithOperation[ImportAssetsCommand]("media.asset.import"),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &ImportAssetsHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[ImportAssetsCommand].
func (h *ImportAssetsHandler) Execute(ctx context.Context, msg ImportAssetsCommand) error {
	return h.inner.Execute(ctx, msg)
}

func validateBindingSet(bindings media.BindingSet) error {
	for key, list := range bindings {
		if len(list) == 0 {
			return validation.NewError("cms.media.asset.binding_empty", "binding list cannot be empty for "+key)
		}
		for idx, binding := range list {
			if strings.TrimSpace(binding.Slot) == "" {
				return validation.NewError("cms.media.asset.slot_required", "slot is required for binding "+key)
			}
			if !validMediaReference(binding.Reference) {
				return validation.NewError("cms.media.asset.reference_required", fmt.Sprintf("binding %s[%d] must include an id or path reference", key, idx))
			}
			if binding.Position < 0 {
				return validation.NewError("cms.media.asset.position_invalid", "position must be zero or positive")
			}
		}
	}
	return nil
}

func validMediaReference(ref interfaces.MediaReference) bool {
	if strings.TrimSpace(ref.ID) != "" {
		return true
	}
	if strings.TrimSpace(ref.Path) != "" {
		return true
	}
	if strings.TrimSpace(ref.Collection) != "" && strings.TrimSpace(ref.Locale) != "" && strings.TrimSpace(ref.Variant) != "" {
		return true
	}
	return false
}
