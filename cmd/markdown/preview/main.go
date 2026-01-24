package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/goliatone/go-cms/cmd/markdown/internal/bootstrap"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

var moduleBuilder = bootstrap.BuildModule

func main() {
	var (
		contentDir          = flag.String("content-dir", "content", "Path to the markdown content root")
		pattern             = flag.String("pattern", "*.md", "Glob pattern applied when discovering markdown files")
		locales             = flag.String("locales", "", "Comma separated list of locales (defaults to config locales)")
		defaultLocale       = flag.String("default-locale", "en", "Default locale for fallback documents")
		translationsEnabled = flag.Bool("translations-enabled", true, "Enable translations (set false for monolingual mode)")
		requireTranslations = flag.Bool("require-translations", true, "Require at least one translation when translations are enabled")
		filePath            = flag.String("file", "", "Markdown file to preview (relative to the content root)")
		renderHTML          = flag.Bool("render-html", true, "Render markdown body into HTML as part of the preview")
	)

	flag.Parse()

	if *filePath == "" {
		log.Fatalf("--file is required")
	}

	opts := bootstrap.Options{
		ContentDir:          *contentDir,
		Pattern:             *pattern,
		Recursive:           true,
		DefaultLocale:       *defaultLocale,
		Locales:             bootstrap.SplitLocales(*locales),
		TranslationsEnabled: translationsEnabled,
		RequireTranslations: requireTranslations,
	}

	module, err := moduleBuilder(opts)
	if err != nil {
		log.Fatalf("bootstrap module: %v", err)
	}

	if module == nil || module.Service == nil {
		log.Fatalf("markdown service not configured; ensure Features.Markdown is enabled")
	}

	ctx := context.Background()

	doc, err := module.Service.Load(ctx, *filePath, interfaces.LoadOptions{})
	if err != nil {
		log.Fatalf("load markdown document: %v", err)
	}

	if *renderHTML {
		if _, err := module.Service.RenderDocument(ctx, doc, interfaces.ParseOptions{}); err != nil {
			log.Fatalf("render markdown: %v", err)
		}
	}

	fmt.Fprintf(os.Stdout, "Path: %s\nLocale: %s\nChecksum: %x\n\n", doc.FilePath, doc.Locale, doc.Checksum)

	if doc.FrontMatter.Raw != nil {
		frontmatter, err := json.MarshalIndent(doc.FrontMatter.Raw, "", "  ")
		if err == nil {
			fmt.Fprintf(os.Stdout, "Frontmatter:\n%s\n\n", frontmatter)
		}
	}

	if *renderHTML {
		fmt.Fprintf(os.Stdout, "Rendered HTML:\n%s\n", string(doc.BodyHTML))
	} else {
		fmt.Fprintf(os.Stdout, "Markdown Body:\n%s\n", string(doc.Body))
	}
}
