package generator

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type sitemapEntry struct {
	Location   string
	LastMod    time.Time
	Priority   string
	ChangeFreq string
}

func buildSitemap(baseURL string, pages []RenderedPage, fallback time.Time) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "http://localhost"
	}

	entries := make([]sitemapEntry, 0, len(pages))
	seen := map[string]struct{}{}
	for _, page := range pages {
		route := strings.TrimSpace(page.Route)
		if route == "" {
			route = "/"
		}
		if !strings.HasPrefix(route, "/") {
			route = "/" + route
		}
		location := base + route
		if _, ok := seen[location]; ok {
			continue
		}
		seen[location] = struct{}{}
		lastMod := page.Metadata.LastModified
		if lastMod.IsZero() {
			lastMod = fallback
		}
		entries = append(entries, sitemapEntry{
			Location: location,
			LastMod:  lastMod,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Location < entries[j].Location
	})

	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	builder.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, entry := range entries {
		builder.WriteString("  <url>\n")
		builder.WriteString(fmt.Sprintf("    <loc>%s</loc>\n", entry.Location))
		if !entry.LastMod.IsZero() {
			builder.WriteString(fmt.Sprintf("    <lastmod>%s</lastmod>\n", entry.LastMod.UTC().Format(time.RFC3339)))
		}
		builder.WriteString("  </url>\n")
	}
	builder.WriteString(`</urlset>` + "\n")
	return builder.String()
}

func buildRobots(baseURL string, includeSitemap bool) string {
	var builder strings.Builder
	builder.WriteString("User-agent: *\n")
	builder.WriteString("Allow: /\n")
	if includeSitemap {
		base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if base == "" {
			base = "http://localhost"
		}
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Sitemap: %s/sitemap.xml\n", base))
	}
	return builder.String()
}
