package content

import "github.com/goliatone/go-slug"

// SlugNormalizer exposes the slug normalizer interface.
type SlugNormalizer = slug.Normalizer

// DefaultSlugNormalizer returns the default slug normalizer.
func DefaultSlugNormalizer() SlugNormalizer {
	return slug.Default()
}

// NormalizeSlug applies the default slug normalization rules.
func NormalizeSlug(value string) (string, error) {
	return slug.Normalize(value)
}

// IsValidSlug reports whether the slug matches the default rules.
func IsValidSlug(value string) bool {
	return slug.IsValid(value)
}
