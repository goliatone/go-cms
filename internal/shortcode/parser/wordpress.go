package parser

import (
	"fmt"
	"regexp"
	"strings"
)

var wpTagPattern = regexp.MustCompile(`\[(\/?)([a-zA-Z0-9_\-]+)([^\]]*)\]`)

// WordPressPreprocessor converts WordPress-style shortcodes to Hugo syntax.
type WordPressPreprocessor struct{}

// NewWordPressPreprocessor constructs a preprocessor.
func NewWordPressPreprocessor() *WordPressPreprocessor {
	return &WordPressPreprocessor{}
}

// Process rewrites bracket shortcodes into Hugo-style equivalents.
func (p *WordPressPreprocessor) Process(content string) string {
	if !strings.Contains(content, "[") {
		return content
	}

	return wpTagPattern.ReplaceAllStringFunc(content, func(tag string) string {
		matches := wpTagPattern.FindStringSubmatch(tag)
		if len(matches) < 3 {
			return tag
		}

		isClosing := matches[1] == "/"
		name := matches[2]
		rawAttr := strings.TrimSpace(matches[3])

		if isClosing {
			return fmt.Sprintf("{{< /%s >}}", name)
		}

		selfClosing := strings.HasSuffix(rawAttr, "/")
		if selfClosing {
			rawAttr = strings.TrimSpace(strings.TrimSuffix(rawAttr, "/"))
		}

		formatted := formatAttributes(rawAttr)
		return fmt.Sprintf("{{< %s%s >}}", name, formatted)
	})
}

func formatAttributes(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return " " + raw
}
