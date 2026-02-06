package content

import "strings"

// ContentListOption configures content list behavior. It is an alias to string to
// preserve the existing List(ctx, env ...string) call pattern.
type ContentListOption = string

// ContentGetOption configures content get behavior. It reuses list option tokens.
type ContentGetOption = ContentListOption

const contentListWithTranslations ContentListOption = "content:list:with_translations"

// WithTranslations preloads translations when listing content records.
func WithTranslations() ContentListOption {
	return contentListWithTranslations
}

type contentListOptions struct {
	envKey              string
	includeTranslations bool
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
			if opts.envKey == "" {
				opts.envKey = token
			}
		}
	}
	return opts
}
