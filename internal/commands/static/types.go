package staticcmd

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/google/uuid"
)

const (
	buildSiteMessageType = "cms.static.build"
	diffSiteMessageType  = "cms.static.diff"
	cleanSiteMessageType = "cms.static.clean"
)

// ResultCallback receives build results produced by generator operations. The callback is optional
// and is invoked synchronously from the handler when a BuildResult is available.
type ResultCallback func(ResultEnvelope)

// ResultEnvelope captures the outcome of a static command execution that generated a BuildResult.
type ResultEnvelope struct {
	Result   *generator.BuildResult
	Metadata map[string]any
}

// BuildSiteCommand executes a generator build using the provided filters.
type BuildSiteCommand struct {
	PageIDs        []uuid.UUID    `json:"page_ids,omitempty"`
	Locales        []string       `json:"locales,omitempty"`
	Force          bool           `json:"force,omitempty"`
	DryRun         bool           `json:"dry_run,omitempty"`
	AssetsOnly     bool           `json:"assets_only,omitempty"`
	ResultCallback ResultCallback `json:"-"`
}

// Type implements command.Message.
func (BuildSiteCommand) Type() string { return buildSiteMessageType }

// Validate ensures locales are well-formed and page identifiers are valid UUIDs.
func (m BuildSiteCommand) Validate() error {
	errs := validation.Errors{}
	if len(m.Locales) > 0 {
		for _, locale := range m.Locales {
			trimmed := strings.TrimSpace(locale)
			if trimmed == "" {
				errs["locales"] = validation.NewError("cms.static.build.locale_invalid", "locales must not contain empty values")
				break
			}
		}
	}
	for _, id := range m.PageIDs {
		if id == uuid.Nil {
			errs["page_ids"] = validation.NewError("cms.static.build.page_id_invalid", "page_ids must contain valid identifiers")
			break
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// DiffSiteCommand performs a dry-run build to surface differences without writing artifacts.
type DiffSiteCommand struct {
	PageIDs        []uuid.UUID    `json:"page_ids,omitempty"`
	Locales        []string       `json:"locales,omitempty"`
	Force          bool           `json:"force,omitempty"`
	ResultCallback ResultCallback `json:"-"`
}

// Type implements command.Message.
func (DiffSiteCommand) Type() string { return diffSiteMessageType }

// Validate ensures locales and page identifiers are well-formed.
func (m DiffSiteCommand) Validate() error {
	errs := validation.Errors{}
	for _, id := range m.PageIDs {
		if id == uuid.Nil {
			errs["page_ids"] = validation.NewError("cms.static.diff.page_id_invalid", "page_ids must contain valid identifiers")
			break
		}
	}
	for _, locale := range m.Locales {
		if strings.TrimSpace(locale) == "" {
			errs["locales"] = validation.NewError("cms.static.diff.locale_invalid", "locales must not contain empty values")
			break
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// CleanSiteCommand clears generator artifacts from the configured storage backend.
type CleanSiteCommand struct{}

// Type implements command.Message.
func (CleanSiteCommand) Type() string { return cleanSiteMessageType }

// Validate satisfies command.Message; there are no payload constraints.
func (CleanSiteCommand) Validate() error { return nil }

// FeatureGates exposes runtime switches used to guard handler execution.
type FeatureGates struct {
	GeneratorEnabled func() bool
}

func (g FeatureGates) generatorEnabled() bool {
	if g.GeneratorEnabled == nil {
		return false
	}
	return g.GeneratorEnabled()
}
