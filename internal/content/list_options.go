package content

import (
	"strings"

	cmscontent "github.com/goliatone/go-cms/content"
	"github.com/google/uuid"
)

// ContentListOption configures content list behavior. It is an alias to string to
// preserve the existing List(ctx, env ...string) call pattern.
type ContentListOption = cmscontent.ContentListOption

// ContentGetOption configures content get behavior. It reuses list option tokens.
type ContentGetOption = cmscontent.ContentGetOption

const (
	contentListWithTranslations     ContentListOption = "content:list:with_translations"
	contentListProjectionPrefix     ContentListOption = "content:list:projection:"
	contentListProjectionModePrefix ContentListOption = "content:list:projection_mode:"
	contentListContentTypePrefix    ContentListOption = "content:list:content_type:"
)

// WithTranslations preloads translations when listing content records.
func WithTranslations() ContentListOption {
	return cmscontent.WithTranslations()
}

// WithProjection configures a named projection for list/get reads.
func WithProjection(name string) ContentListOption {
	return cmscontent.WithProjection(name)
}

// WithDerivedFields enables the canonical derived-content-fields projection.
func WithDerivedFields() ContentListOption {
	return cmscontent.WithDerivedFields()
}

// WithProjectionMode controls projection behavior when translations are not loaded.
func WithProjectionMode(mode ProjectionTranslationMode) ContentListOption {
	return cmscontent.WithProjectionMode(mode)
}

// WithContentTypeID scopes list reads to one content type before loading
// translations or projections.
func WithContentTypeID(id uuid.UUID) ContentListOption {
	return cmscontent.WithContentTypeID(id)
}

type contentListOptions struct {
	envKey              string
	includeTranslations bool
	projection          string
	projectionMode      ProjectionTranslationMode
	projectionModeSet   bool
	contentTypeID       uuid.UUID
}

func parseContentListOptions(args ...ContentListOption) contentListOptions {
	var opts contentListOptions
	for _, raw := range args {
		token := strings.TrimSpace(raw)
		if token == "" {
			continue
		}
		switch token {
		case contentListWithTranslations:
			opts.includeTranslations = true
		default:
			if after, ok := strings.CutPrefix(token, string(contentListProjectionPrefix)); ok {
				opts.projection = strings.ToLower(strings.TrimSpace(after))
				continue
			}
			if after, ok := strings.CutPrefix(token, string(contentListProjectionModePrefix)); ok {
				mode := strings.ToLower(strings.TrimSpace(after))
				if mode != "" {
					opts.projectionMode = ProjectionTranslationMode(mode)
					opts.projectionModeSet = true
				}
				continue
			}
			if after, ok := strings.CutPrefix(token, string(contentListContentTypePrefix)); ok {
				if id, err := uuid.Parse(strings.TrimSpace(after)); err == nil {
					opts.contentTypeID = id
				}
				continue
			}
			if opts.envKey == "" {
				opts.envKey = token
			}
		}
	}
	return opts
}
