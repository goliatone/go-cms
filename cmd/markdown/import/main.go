package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/goliatone/go-cms/cmd/markdown/internal/bootstrap"
	markdowncmd "github.com/goliatone/go-cms/internal/commands/markdown"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

var moduleBuilder = bootstrap.BuildModule

func main() {
	if err := runImport(os.Args[1:]); err != nil {
		log.Fatalf("markdown import: %v", err)
	}
}

func runImport(args []string) error {
	fs := flag.NewFlagSet("markdown-import", flag.ExitOnError)
	contentDir := fs.String("content-dir", "content", "Path to the markdown content root")
	pattern := fs.String("pattern", "*.md", "Glob pattern applied when discovering markdown files")
	locales := fs.String("locales", "", "Comma separated list of locales (defaults to config locales)")
	defaultLocale := fs.String("default-locale", "en", "Default locale for fallback documents")
	translationsEnabled := fs.Bool("translations-enabled", true, "Enable translations (set false for monolingual mode)")
	requireTranslations := fs.Bool("require-translations", true, "Require at least one translation when translations are enabled")
	directory := fs.String("directory", ".", "Directory to import, relative to the content root")
	contentType := fs.String("content-type", "", "Content type ID to associate with imported documents")
	author := fs.String("author", "", "Author ID recorded on imported content")
	dryRun := fs.Bool("dry-run", false, "Preview changes without persisting content")

	if err := fs.Parse(args); err != nil {
		return err
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
		return fmt.Errorf("bootstrap module: %w", err)
	}
	if module == nil || module.Service == nil {
		return fmt.Errorf("markdown service not configured; ensure Features.Markdown is enabled")
	}

	ctx := context.Background()

	importOpts := interfaces.ImportOptions{
		DryRun: *dryRun,
	}

	if id, err := bootstrap.ParseUUID(*contentType); err != nil {
		return fmt.Errorf("parse content-type: %w", err)
	} else {
		importOpts.ContentTypeID = id
	}
	if importOpts.ContentTypeID == uuid.Nil {
		return fmt.Errorf("content-type is required")
	}

	if id, err := bootstrap.ParseUUID(*author); err != nil {
		return fmt.Errorf("parse author: %w", err)
	} else {
		importOpts.AuthorID = id
	}

	handler := markdowncmd.NewImportDirectoryHandler(module.Service, module.Logger, markdowncmd.FeatureGates{
		MarkdownEnabled: func() bool { return true },
	})
	cmd := markdowncmd.ImportDirectoryCommand{
		Directory:     *directory,
		ContentTypeID: importOpts.ContentTypeID,
		AuthorID:      importOpts.AuthorID,
		DryRun:        importOpts.DryRun,
	}
	if err := handler.Execute(ctx, cmd); err != nil {
		return fmt.Errorf("execute import command: %w", err)
	}
	fmt.Fprintln(os.Stdout, "markdown import command executed successfully")

	return nil
}
