package generator

import (
	"path"
	"strings"
)

func buildOutputPath(route string, locale string, defaultLocale string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		route = "/"
	}
	clean := strings.Trim(route, " \t\r\n/")
	locale = strings.TrimSpace(locale)
	defaultLocale = strings.TrimSpace(defaultLocale)

	if locale == "" && defaultLocale != "" {
		locale = defaultLocale
	}

	if locale == "" || strings.EqualFold(locale, defaultLocale) {
		if clean == "" {
			return "index.html"
		}
		return path.Join(clean, "index.html")
	}

	segments := []string{}
	if clean != "" {
		segments = strings.Split(clean, "/")
		if len(segments) > 0 && strings.EqualFold(segments[0], locale) {
			segments = segments[1:]
		}
	}

	if len(segments) == 0 {
		return path.Join(locale, "index.html")
	}

	routePart := path.Join(segments...)
	if routePart == "" || routePart == "." {
		return path.Join(locale, "index.html")
	}
	return path.Join(locale, routePart, "index.html")
}
