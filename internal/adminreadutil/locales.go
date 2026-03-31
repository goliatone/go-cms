package adminreadutil

import (
	"context"
	"slices"
	"strings"

	sharedi18n "github.com/goliatone/go-i18n"
	"github.com/google/uuid"
)

// NormalizeLocale trims and canonicalizes locale codes.
func NormalizeLocale(code string) string {
	return sharedi18n.NormalizeLocale(code)
}

// DedupeLocalePreference clears the fallback when it matches the primary locale.
func DedupeLocalePreference(primaryID uuid.UUID, primaryCode string, fallbackID uuid.UUID, fallbackCode string) (uuid.UUID, string, uuid.UUID, string) {
	if primaryID != uuid.Nil && primaryID == fallbackID {
		fallbackID = uuid.Nil
		fallbackCode = ""
	}
	if NormalizeLocale(primaryCode) != "" && NormalizeLocale(primaryCode) == NormalizeLocale(fallbackCode) {
		fallbackID = uuid.Nil
		fallbackCode = ""
	}
	return primaryID, NormalizeLocale(primaryCode), fallbackID, NormalizeLocale(fallbackCode)
}

// LocaleCodeByID resolves a locale code lazily using the supplied callback.
func LocaleCodeByID(ctx context.Context, id uuid.UUID, resolver func(context.Context, uuid.UUID) (string, error)) string {
	if id == uuid.Nil || resolver == nil {
		return ""
	}
	code, err := resolver(ctx, id)
	if err != nil {
		return ""
	}
	return NormalizeLocale(code)
}

// CollectLocaleCodes normalizes, deduplicates, and preserves locale order.
func CollectLocaleCodes(ctx context.Context, ids []uuid.UUID, directCodes []string, resolver func(context.Context, uuid.UUID) (string, error)) []string {
	if len(ids) == 0 && len(directCodes) == 0 {
		return nil
	}
	out := make([]string, 0, max(len(ids), len(directCodes)))
	seen := map[string]struct{}{}
	for idx, code := range directCodes {
		normalized := NormalizeLocale(code)
		if normalized == "" && idx < len(ids) {
			normalized = LocaleCodeByID(ctx, ids[idx], resolver)
		}
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	for _, id := range ids {
		normalized := LocaleCodeByID(ctx, id, resolver)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// MissingRequestedLocale reports whether a requested locale is absent from the available set.
func MissingRequestedLocale(requested string, available []string) bool {
	normalized := NormalizeLocale(requested)
	if normalized == "" || len(available) == 0 {
		return false
	}
	return !slices.ContainsFunc(available, func(candidate string) bool {
		return strings.EqualFold(NormalizeLocale(candidate), normalized)
	})
}
