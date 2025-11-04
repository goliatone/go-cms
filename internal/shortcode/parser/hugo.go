package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

var (
	startTagPattern = regexp.MustCompile(`{{<\s*([^\s/>]+)([^>]*)>}}`)
	endTagPattern   = regexp.MustCompile(`{{<\s*/\s*([^\s>]+)\s*>}}`)
)

// HugoParser parses Hugo-style shortcodes ({{< name param >}}).
type HugoParser struct {
}

// NewHugoParser creates a parser instance.
func NewHugoParser() *HugoParser {
	return &HugoParser{}
}

// Parse returns the list of parsed shortcodes in the content.
func (p *HugoParser) Parse(content string) ([]interfaces.ParsedShortcode, error) {
	_, shortcodes, err := p.Extract(content)
	return shortcodes, err
}

// Extract replaces shortcodes with placeholders and returns both the transformed content and extracted definitions.
func (p *HugoParser) Extract(content string) (string, []interfaces.ParsedShortcode, error) {
	type stackEntry struct {
		name       string
		startIndex int
		params     map[string]any
	}

	var (
		result     []rune
		shortcodes []interfaces.ParsedShortcode
		stack      []stackEntry
		position   int
	)

	appendString := func(s string) {
		result = append(result, []rune(s)...)
	}

	for position < len(content) {
		loc := startTagPattern.FindStringIndex(content[position:])
		endLoc := endTagPattern.FindStringIndex(content[position:])

		if loc == nil && endLoc == nil {
			appendString(content[position:])
			break
		}

		startPos := -1
		if loc != nil {
			startPos = position + loc[0]
		}

		endPos := -1
		if endLoc != nil {
			endPos = position + endLoc[0]
		}

		if startPos >= 0 && (endPos == -1 || startPos < endPos) {
			// append text preceding tag
			appendString(content[position:startPos])

			matches := startTagPattern.FindStringSubmatch(content[startPos:])
			if len(matches) < 2 {
				return "", nil, fmt.Errorf("invalid shortcode start tag at position %d", startPos)
			}
			name := matches[1]
			rawParams := strings.TrimSpace(matches[2])
			params := parseParams(rawParams)

			// Determine if this shortcode is self-closing (no corresponding end tag).
			remainder := content[startPos+len(matches[0]):]
			endMatcher := regexp.MustCompile(fmt.Sprintf(`{{<\s*/\s*%s\s*>}}`, regexp.QuoteMeta(name)))
			if loc := endMatcher.FindStringIndex(remainder); loc == nil {
				placeholder := fmt.Sprintf("<!-- shortcode:%d -->", len(shortcodes))
				appendString(placeholder)
				shortcodes = append(shortcodes, interfaces.ParsedShortcode{
					Name:   name,
					Params: params,
				})
				position = startPos + len(matches[0])
				continue
			}

			stack = append(stack, stackEntry{
				name:       name,
				startIndex: len(result),
				params:     params,
			})

			position = startPos + len(matches[0])
			continue
		}

		if endPos >= 0 {
			appendString(content[position:endPos])

			matches := endTagPattern.FindStringSubmatch(content[endPos:])
			if len(matches) < 2 {
				return "", nil, fmt.Errorf("invalid shortcode end tag at position %d", endPos)
			}
			name := matches[1]
			if len(stack) == 0 {
				return "", nil, fmt.Errorf("unexpected closing shortcode %s at position %d", name, endPos)
			}

			entry := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if entry.name != name {
				return "", nil, fmt.Errorf("mismatched shortcode end tag %s, expected %s", name, entry.name)
			}

			inner := string(result[entry.startIndex:])
			result = result[:entry.startIndex]

			placeholder := fmt.Sprintf("<!-- shortcode:%d -->", len(shortcodes))
			appendString(placeholder)

			shortcodes = append(shortcodes, interfaces.ParsedShortcode{
				Name:   name,
				Params: entry.params,
				Inner:  inner,
			})

			position = endPos + len(matches[0])
			continue
		}
	}

	if len(stack) > 0 {
		return "", nil, fmt.Errorf("unterminated shortcode %s", stack[len(stack)-1].name)
	}

	return string(result), shortcodes, nil
}

func parseParams(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	parts := strings.Fields(raw)
	params := make(map[string]any, len(parts))
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.Trim(kv[1], `"`)
			params[key] = value
		} else {
			params[fmt.Sprintf("param%d", len(params)+1)] = strings.Trim(part, `"`)
		}
	}
	return params
}
