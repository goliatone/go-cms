package shortcode

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Sanitizer is a conservative implementation that rejects inline script tags and enforces URL schemes.
type Sanitizer struct {
	allowedSchemes map[string]struct{}
}

// NewSanitizer returns a sanitizer allowing http/https URLs.
func NewSanitizer() *Sanitizer {
	return &Sanitizer{
		allowedSchemes: map[string]struct{}{
			"http":  {},
			"https": {},
			"":      {},
		},
	}
}

// Sanitize rejects obvious script injections while preserving safe markup.
func (s *Sanitizer) Sanitize(html string) (string, error) {
	lower := strings.ToLower(html)
	if strings.Contains(lower, "<script") {
		return "", fmt.Errorf("shortcode: script tags are not allowed")
	}
	return html, nil
}

// ValidateURL ensures the URL has an allowed scheme.
func (s *Sanitizer) ValidateURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}

	if _, ok := s.allowedSchemes[strings.ToLower(parsed.Scheme)]; !ok {
		return fmt.Errorf("shortcode: url scheme %q not permitted", parsed.Scheme)
	}
	return nil
}

// ValidateAttributes rejects inline event handlers like onload/onerror.
func (s *Sanitizer) ValidateAttributes(attrs map[string]any) error {
	for key := range attrs {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "on") {
			return fmt.Errorf("shortcode: attribute %q not permitted", key)
		}
	}
	return nil
}

var _ interfaces.ShortcodeSanitizer = (*Sanitizer)(nil)
