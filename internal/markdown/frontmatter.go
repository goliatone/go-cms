package markdown

import (
	"bytes"
	"fmt"
	"time"

	"github.com/adrg/frontmatter"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// ParseFrontMatter extracts metadata and Markdown body content from the
// provided source bytes. It returns the structured frontmatter, the Markdown
// body without delimiters, and any error encountered.
func ParseFrontMatter(source []byte) (interfaces.FrontMatter, []byte, error) {
	var meta frontMatterEnvelope

	reader := bytes.NewReader(source)
	body, err := frontmatter.Parse(reader, &meta)
	if err != nil {
		return interfaces.FrontMatter{}, nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	return envelopeToFrontMatter(meta), body, nil
}

// BuildDocument assembles an interfaces.Document from the supplied file path,
// locale, raw content, and modification time. BodyHTML is intentionally left
// empty so callers can render lazily.
func BuildDocument(path string, locale string, source []byte, modified time.Time) (*interfaces.Document, error) {
	frontmatter, body, err := ParseFrontMatter(source)
	if err != nil {
		return nil, err
	}

	return &interfaces.Document{
		FilePath:     path,
		Locale:       locale,
		FrontMatter:  frontmatter,
		Body:         body,
		LastModified: modified,
	}, nil
}

type frontMatterEnvelope struct {
	Title    string         `yaml:"title"`
	Slug     string         `yaml:"slug"`
	Summary  string         `yaml:"summary"`
	Status   string         `yaml:"status"`
	Template string         `yaml:"template"`
	Tags     []string       `yaml:"tags"`
	Author   string         `yaml:"author"`
	Date     time.Time      `yaml:"date"`
	Draft    bool           `yaml:"draft"`
	Custom   map[string]any `yaml:",inline"`
}

func envelopeToFrontMatter(env frontMatterEnvelope) interfaces.FrontMatter {
	if env.Custom == nil {
		env.Custom = map[string]any{}
	}

	raw := make(map[string]any, len(env.Custom)+8)
	for key, value := range env.Custom {
		raw[key] = value
	}

	if env.Title != "" {
		raw["title"] = env.Title
	}
	if env.Slug != "" {
		raw["slug"] = env.Slug
	}
	if env.Summary != "" {
		raw["summary"] = env.Summary
	}
	if env.Status != "" {
		raw["status"] = env.Status
	}
	if env.Template != "" {
		raw["template"] = env.Template
	}
	if len(env.Tags) > 0 {
		raw["tags"] = append([]string(nil), env.Tags...)
	}
	if env.Author != "" {
		raw["author"] = env.Author
	}
	if !env.Date.IsZero() {
		raw["date"] = env.Date
	}
	raw["draft"] = env.Draft

	return interfaces.FrontMatter{
		Title:    env.Title,
		Slug:     env.Slug,
		Summary:  env.Summary,
		Status:   env.Status,
		Template: env.Template,
		Tags:     append([]string(nil), env.Tags...),
		Author:   env.Author,
		Date:     env.Date,
		Draft:    env.Draft,
		Custom:   cloneMap(env.Custom),
		Raw:      raw,
	}
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}

	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
