package generator

import (
	"context"
	"fmt"
	"html"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/content"
)

const maxFeedItems = 100

type feedItem struct {
	Title       string
	Summary     string
	Link        string
	GUID        string
	PublishedAt time.Time
	UpdatedAt   time.Time
}

type feedDocument struct {
	Locale LocaleSpec
	Items  []feedItem
}

func (s *service) buildFeedDocuments(buildCtx *BuildContext) []feedDocument {
	if buildCtx == nil || len(buildCtx.Pages) == 0 {
		return nil
	}

	byLocale := make(map[string]*feedDocument)
	seen := make(map[string]map[string]struct{})

	for _, data := range buildCtx.Pages {
		if data == nil || data.Page == nil || data.Translation == nil {
			continue
		}
		route := strings.TrimSpace(safeTranslationPath(data.Translation))
		if route == "" {
			continue
		}

		localeCode := data.Locale.Code
		doc := byLocale[localeCode]
		if doc == nil {
			doc = &feedDocument{Locale: data.Locale}
			byLocale[localeCode] = doc
			seen[localeCode] = map[string]struct{}{}
		}

		guid := fmt.Sprintf("%s:%s", data.Page.ID.String(), localeCode)
		if _, ok := seen[localeCode][guid]; ok {
			continue
		}
		seen[localeCode][guid] = struct{}{}

		title := strings.TrimSpace(data.Translation.Title)
		if title == "" && data.ContentTranslation != nil {
			title = strings.TrimSpace(data.ContentTranslation.Title)
		}
		if title == "" {
			title = route
		}
		summary := feedSummaryForPage(data)
		link := absoluteURL(s.cfg.BaseURL, route)

		publishedAt := firstNonZeroTime(
			timePtrOrZero(data.Page.PublishedAt),
			contentPublishedAt(data.Content),
			data.Metadata.LastModified,
			data.Page.CreatedAt,
		)
		if publishedAt.IsZero() {
			publishedAt = buildCtx.GeneratedAt
		}

		updatedAt := firstNonZeroTime(
			data.Metadata.LastModified,
			data.Page.UpdatedAt,
			contentUpdatedAt(data.Content),
			publishedAt,
		)

		doc.Items = append(doc.Items, feedItem{
			Title:       title,
			Summary:     summary,
			Link:        link,
			GUID:        guid,
			PublishedAt: publishedAt,
			UpdatedAt:   updatedAt,
		})
	}

	docs := make([]feedDocument, 0, len(byLocale))
	for _, doc := range byLocale {
		if len(doc.Items) == 0 {
			continue
		}
		sort.Slice(doc.Items, func(i, j int) bool {
			left := doc.Items[i].PublishedAt
			if left.IsZero() {
				left = doc.Items[i].UpdatedAt
			}
			right := doc.Items[j].PublishedAt
			if right.IsZero() {
				right = doc.Items[j].UpdatedAt
			}
			if left.Equal(right) {
				return doc.Items[i].GUID < doc.Items[j].GUID
			}
			return left.After(right)
		})
		if len(doc.Items) > maxFeedItems {
			doc.Items = append([]feedItem(nil), doc.Items[:maxFeedItems]...)
		}
		docs = append(docs, *doc)
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Locale.Code < docs[j].Locale.Code
	})
	return docs
}

func (s *service) writeFeeds(
	ctx context.Context,
	writer artifactWriter,
	siteMeta SiteMetadata,
	buildCtx *BuildContext,
	docs []feedDocument,
) (int, error) {
	if len(docs) == 0 {
		return 0, nil
	}
	baseDir := strings.Trim(strings.TrimSpace(s.cfg.OutputDir), "/")
	dirCache := map[string]struct{}{}
	if baseDir != "" {
		dirCache[baseDir] = struct{}{}
		if err := writer.EnsureDir(ctx, baseDir); err != nil {
			return 0, err
		}
	}

	total := 0
	defaultRSSWritten := false
	defaultAtomWritten := false
	for _, doc := range docs {
		if len(doc.Items) == 0 {
			continue
		}
		rssContent := buildRSSFeed(siteMeta, doc, buildCtx.GeneratedAt)
		rssPath := joinOutputPath(baseDir, path.Join("feeds", fmt.Sprintf("%s.rss.xml", doc.Locale.Code)))
		if err := ensureDir(ctx, writer, dirCache, path.Dir(rssPath)); err != nil {
			return total, err
		}
		if err := writer.WriteFile(ctx, writeFileRequest{
			Path:        rssPath,
			Content:     strings.NewReader(rssContent),
			Size:        int64(len(rssContent)),
			Locale:      doc.Locale.Code,
			Category:    categoryFeed,
			ContentType: "application/rss+xml",
			Checksum:    computeHashFromString(rssContent),
			Metadata:    feedMetadata(doc.Locale.Code, "rss", buildCtx.GeneratedAt, false),
		}); err != nil {
			return total, err
		}
		total++

		atomContent := buildAtomFeed(siteMeta, doc, buildCtx.GeneratedAt)
		atomPath := joinOutputPath(baseDir, path.Join("feeds", fmt.Sprintf("%s.atom.xml", doc.Locale.Code)))
		if err := ensureDir(ctx, writer, dirCache, path.Dir(atomPath)); err != nil {
			return total, err
		}
		if err := writer.WriteFile(ctx, writeFileRequest{
			Path:        atomPath,
			Content:     strings.NewReader(atomContent),
			Size:        int64(len(atomContent)),
			Locale:      doc.Locale.Code,
			Category:    categoryFeed,
			ContentType: "application/atom+xml",
			Checksum:    computeHashFromString(atomContent),
			Metadata:    feedMetadata(doc.Locale.Code, "atom", buildCtx.GeneratedAt, false),
		}); err != nil {
			return total, err
		}
		total++

		if doc.Locale.IsDefault {
			if !defaultRSSWritten {
				defaultRSSWritten = true
				defaultRSSPath := joinOutputPath(baseDir, "feed.xml")
				if err := ensureDir(ctx, writer, dirCache, path.Dir(defaultRSSPath)); err != nil {
					return total, err
				}
				if err := writer.WriteFile(ctx, writeFileRequest{
					Path:        defaultRSSPath,
					Content:     strings.NewReader(rssContent),
					Size:        int64(len(rssContent)),
					Locale:      doc.Locale.Code,
					Category:    categoryFeed,
					ContentType: "application/rss+xml",
					Checksum:    computeHashFromString(rssContent),
					Metadata:    feedMetadata(doc.Locale.Code, "rss", buildCtx.GeneratedAt, true),
				}); err != nil {
					return total, err
				}
				total++
			}
			if !defaultAtomWritten {
				defaultAtomWritten = true
				defaultAtomPath := joinOutputPath(baseDir, "feed.atom.xml")
				if err := ensureDir(ctx, writer, dirCache, path.Dir(defaultAtomPath)); err != nil {
					return total, err
				}
				if err := writer.WriteFile(ctx, writeFileRequest{
					Path:        defaultAtomPath,
					Content:     strings.NewReader(atomContent),
					Size:        int64(len(atomContent)),
					Locale:      doc.Locale.Code,
					Category:    categoryFeed,
					ContentType: "application/atom+xml",
					Checksum:    computeHashFromString(atomContent),
					Metadata:    feedMetadata(doc.Locale.Code, "atom", buildCtx.GeneratedAt, true),
				}); err != nil {
					return total, err
				}
				total++
			}
		}
	}
	return total, nil
}

func buildRSSFeed(site SiteMetadata, doc feedDocument, generatedAt time.Time) string {
	baseLink := baseURLWithFallback(site.BaseURL)
	title := feedTitleForLocale(site, doc.Locale)
	description := feedDescriptionForLocale(site, doc.Locale)

	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	builder.WriteString(`<rss version="2.0">` + "\n")
	builder.WriteString("  <channel>\n")
	builder.WriteString(fmt.Sprintf("    <title>%s</title>\n", escapeXML(title)))
	builder.WriteString(fmt.Sprintf("    <link>%s</link>\n", escapeXML(baseLink)))
	builder.WriteString(fmt.Sprintf("    <description>%s</description>\n", escapeXML(description)))
	builder.WriteString(fmt.Sprintf("    <language>%s</language>\n", escapeXML(doc.Locale.Code)))
	builder.WriteString(fmt.Sprintf("    <lastBuildDate>%s</lastBuildDate>\n", generatedAt.UTC().Format(time.RFC1123Z)))
	for _, item := range doc.Items {
		pub := item.PublishedAt
		if pub.IsZero() {
			pub = generatedAt
		}
		builder.WriteString("    <item>\n")
		builder.WriteString(fmt.Sprintf("      <title>%s</title>\n", escapeXML(item.Title)))
		builder.WriteString(fmt.Sprintf("      <link>%s</link>\n", escapeXML(item.Link)))
		builder.WriteString(fmt.Sprintf("      <guid>%s</guid>\n", escapeXML(item.GUID)))
		builder.WriteString(fmt.Sprintf("      <pubDate>%s</pubDate>\n", pub.UTC().Format(time.RFC1123Z)))
		if item.Summary != "" {
			builder.WriteString(fmt.Sprintf("      <description>%s</description>\n", escapeXML(item.Summary)))
		}
		builder.WriteString("    </item>\n")
	}
	builder.WriteString("  </channel>\n")
	builder.WriteString(`</rss>` + "\n")
	return builder.String()
}

func buildAtomFeed(site SiteMetadata, doc feedDocument, generatedAt time.Time) string {
	baseLink := baseURLWithFallback(site.BaseURL)
	feedID := fmt.Sprintf("%s/feeds/%s.atom.xml", baseLink, doc.Locale.Code)
	title := feedTitleForLocale(site, doc.Locale)

	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	builder.WriteString(fmt.Sprintf(`<feed xmlns="http://www.w3.org/2005/Atom" xml:lang="%s">`+"\n", escapeXMLAttr(doc.Locale.Code)))
	builder.WriteString(fmt.Sprintf("  <id>%s</id>\n", escapeXML(feedID)))
	builder.WriteString(fmt.Sprintf("  <title>%s</title>\n", escapeXML(title)))
	builder.WriteString(fmt.Sprintf("  <updated>%s</updated>\n", generatedAt.UTC().Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf(`  <link rel="alternate" href="%s" />`+"\n", escapeXMLAttr(baseLink)))
	builder.WriteString(fmt.Sprintf(`  <link rel="self" href="%s" />`+"\n", escapeXMLAttr(feedID)))
	for _, item := range doc.Items {
		updated := item.UpdatedAt
		if updated.IsZero() {
			updated = item.PublishedAt
		}
		if updated.IsZero() {
			updated = generatedAt
		}
		builder.WriteString("  <entry>\n")
		builder.WriteString(fmt.Sprintf("    <id>%s</id>\n", escapeXML(item.GUID)))
		builder.WriteString(fmt.Sprintf("    <title>%s</title>\n", escapeXML(item.Title)))
		builder.WriteString(fmt.Sprintf(`    <link href="%s" />`+"\n", escapeXMLAttr(item.Link)))
		builder.WriteString(fmt.Sprintf("    <updated>%s</updated>\n", updated.UTC().Format(time.RFC3339)))
		if !item.PublishedAt.IsZero() {
			builder.WriteString(fmt.Sprintf("    <published>%s</published>\n", item.PublishedAt.UTC().Format(time.RFC3339)))
		}
		if item.Summary != "" {
			builder.WriteString(fmt.Sprintf("    <summary>%s</summary>\n", escapeXML(item.Summary)))
		}
		builder.WriteString("  </entry>\n")
	}
	builder.WriteString(`</feed>` + "\n")
	return builder.String()
}

func feedMetadata(locale, feedType string, generatedAt time.Time, alias bool) map[string]string {
	meta := map[string]string{
		"generated_at": generatedAt.UTC().Format(time.RFC3339),
		"feed_type":    feedType,
	}
	if strings.TrimSpace(locale) != "" {
		meta["locale"] = locale
	}
	if alias {
		meta["alias"] = "true"
	}
	return meta
}

func feedTitleForLocale(site SiteMetadata, locale LocaleSpec) string {
	base := siteTitle(site)
	if locale.IsDefault || strings.TrimSpace(locale.Code) == "" {
		return base
	}
	return fmt.Sprintf("%s (%s)", base, strings.ToUpper(locale.Code))
}

func feedDescriptionForLocale(site SiteMetadata, locale LocaleSpec) string {
	if site.Metadata != nil {
		if desc, ok := site.Metadata["description"].(string); ok && strings.TrimSpace(desc) != "" {
			return strings.TrimSpace(desc)
		}
	}
	if locale.IsDefault {
		return "Latest updates"
	}
	return fmt.Sprintf("Latest updates for %s", strings.ToUpper(locale.Code))
}

func siteTitle(site SiteMetadata) string {
	if site.Metadata != nil {
		if title, ok := site.Metadata["title"].(string); ok && strings.TrimSpace(title) != "" {
			return strings.TrimSpace(title)
		}
	}
	base := strings.TrimSpace(site.BaseURL)
	if base != "" {
		return base
	}
	return "CMS Feed"
}

func baseURLWithFallback(base string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	if trimmed == "" {
		return "http://localhost"
	}
	return trimmed
}

func absoluteURL(base, route string) string {
	targetBase := baseURLWithFallback(base)
	normalized := strings.TrimSpace(route)
	if normalized == "" {
		return targetBase
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	return targetBase + normalized
}

func timePtrOrZero(ts *time.Time) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.UTC()
}

func contentPublishedAt(record *content.Content) time.Time {
	if record == nil {
		return time.Time{}
	}
	return timePtrOrZero(record.PublishedAt)
}

func contentUpdatedAt(record *content.Content) time.Time {
	if record == nil {
		return time.Time{}
	}
	return record.UpdatedAt
}

func firstNonZeroTime(instants ...time.Time) time.Time {
	for _, ts := range instants {
		if !ts.IsZero() {
			return ts
		}
	}
	return time.Time{}
}

func feedSummaryForPage(data *PageData) string {
	if data == nil {
		return ""
	}
	if data.Translation != nil && data.Translation.Summary != nil {
		if summary := strings.TrimSpace(*data.Translation.Summary); summary != "" {
			return normalizeWhitespace(summary)
		}
	}
	if data.ContentTranslation != nil && data.ContentTranslation.Summary != nil {
		if summary := strings.TrimSpace(*data.ContentTranslation.Summary); summary != "" {
			return normalizeWhitespace(summary)
		}
	}
	return ""
}

func normalizeWhitespace(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	return strings.Join(strings.Fields(input), " ")
}

func escapeXML(value string) string {
	return html.EscapeString(value)
}

func escapeXMLAttr(value string) string {
	return html.EscapeString(value)
}
