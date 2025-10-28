package markdown

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

var (
	ErrContentServiceRequired = errors.New("markdown importer: content service is required")
	ErrPageServiceRequired    = errors.New("markdown importer: page service required when CreatePages is enabled")
	ErrSlugMissing            = errors.New("markdown importer: frontmatter slug is required")
	ErrLocaleMissing          = errors.New("markdown importer: locale could not be determined")
	ErrTemplateMissing        = errors.New("markdown importer: template id required when CreatePages is enabled")
)

// ImporterConfig encapsulates dependencies required to persist markdown documents.
type ImporterConfig struct {
	Content interfaces.ContentService
	Pages   interfaces.PageService
	Logger  interfaces.Logger
}

// Importer orchestrates conversion of markdown documents into content and pages.
type Importer struct {
	content interfaces.ContentService
	pages   interfaces.PageService
	logger  interfaces.Logger
}

// NewImporter builds an Importer from the supplied configuration.
func NewImporter(cfg ImporterConfig) *Importer {
	return &Importer{
		content: cfg.Content,
		pages:   cfg.Pages,
		logger:  cfg.Logger,
	}
}

// ImportDocument imports a single markdown document.
func (i *Importer) ImportDocument(ctx context.Context, doc *interfaces.Document, opts interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	if i.content == nil {
		return nil, ErrContentServiceRequired
	}
	group := []*interfaces.Document{doc}
	acc := newImportAccumulator()
	if err := i.applyGroup(ctx, groupKey(doc), group, opts, acc); err != nil {
		acc.addError(err)
	}
	return acc.result(), firstError(errSlice(acc.errors))
}

// ImportDocuments imports an arbitrary slice of documents, grouping them by slug.
func (i *Importer) ImportDocuments(ctx context.Context, docs []*interfaces.Document, opts interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	if i.content == nil {
		return nil, ErrContentServiceRequired
	}

	grouped := groupBySlug(docs)
	acc := newImportAccumulator()
	for slug, group := range grouped {
		group = sortDocuments(group)
		if err := i.applyGroup(ctx, slug, group, opts, acc); err != nil {
			acc.addError(err)
		}
	}
	return acc.result(), firstError(errSlice(acc.errors))
}

// SyncDocuments imports all provided documents and optionally deletes orphaned content.
func (i *Importer) SyncDocuments(ctx context.Context, docs []*interfaces.Document, opts interfaces.SyncOptions) (*interfaces.SyncResult, error) {
	if i.content == nil {
		return nil, ErrContentServiceRequired
	}

	grouped := groupBySlug(docs)
	acc := newSyncAccumulator()

	for slug, group := range grouped {
		group = sortDocuments(group)
		res := newImportAccumulator()
		if err := i.applyGroup(ctx, slug, group, opts.ImportOptions, res); err != nil {
			res.addError(err)
		}
		acc.merge(res.result())
	}

	if opts.DeleteOrphaned {
		if err := i.deleteOrphaned(ctx, grouped, opts, acc); err != nil {
			acc.addError(err)
		}
	}

	return acc.result(), firstError(errSlice(acc.errors))
}

func (i *Importer) applyGroup(ctx context.Context, slug string, docs []*interfaces.Document, opts interfaces.ImportOptions, acc *importAccumulator) error {
	if slug == "" {
		return ErrSlugMissing
	}

	contentTranslations := make([]interfaces.ContentTranslationInput, 0, len(docs))
	titleFallback := fallbackTitle(slug)
	status := selectStatus(docs)

	for _, doc := range docs {
		if err := validateDocument(doc); err != nil {
			return err
		}

		title := strings.TrimSpace(doc.FrontMatter.Title)
		if title == "" {
			title = titleFallback
		}

		fields := buildContentFields(doc)
		contentTranslations = append(contentTranslations, interfaces.ContentTranslationInput{
			Locale:  doc.Locale,
			Title:   title,
			Summary: optionalString(doc.FrontMatter.Summary),
			Fields:  fields,
		})
	}

	existing, err := i.content.GetBySlug(ctx, slug)
	if err != nil && existing != nil {
		return fmt.Errorf("markdown importer: content lookup %s: %w", slug, err)
	}

	if existing == nil {
		if opts.DryRun {
			acc.skip(uuid.Nil)
			return nil
		}

		createReq := interfaces.ContentCreateRequest{
			ContentTypeID: opts.ContentTypeID,
			Slug:          slug,
			Status:        status,
			CreatedBy:     opts.AuthorID,
			UpdatedBy:     opts.AuthorID,
			Translations:  contentTranslations,
			Metadata: map[string]any{
				"source":    "markdown",
				"documents": documentMetadata(docs),
			},
		}

		record, createErr := i.content.Create(ctx, createReq)
		if createErr != nil {
			return fmt.Errorf("markdown importer: create content %s: %w", slug, createErr)
		}
		acc.created(record.ID)
		return i.ensurePage(ctx, record, docs, opts, acc)
	}

	changedTranslations := diffTranslations(existing.Translations, contentTranslations)
	if !changedTranslations {
		acc.skip(existing.ID)
		return nil
	}

	if opts.DryRun {
		acc.skip(existing.ID)
		return nil
	}

	updateReq := interfaces.ContentUpdateRequest{
		ID:           existing.ID,
		Status:       status,
		UpdatedBy:    opts.AuthorID,
		Translations: contentTranslations,
		Metadata: map[string]any{
			"source":    "markdown",
			"documents": documentMetadata(docs),
		},
	}

	updated, updateErr := i.content.Update(ctx, updateReq)
	if updateErr != nil {
		return fmt.Errorf("markdown importer: update content %s: %w", slug, updateErr)
	}
	acc.updated(updated.ID)
	return i.ensurePage(ctx, updated, docs, opts, acc)
}

func (i *Importer) ensurePage(ctx context.Context, content *interfaces.ContentRecord, docs []*interfaces.Document, opts interfaces.ImportOptions, acc *importAccumulator) error {
	if !opts.CreatePages {
		return nil
	}
	if i.pages == nil {
		return ErrPageServiceRequired
	}
	if opts.TemplateID == nil || *opts.TemplateID == uuid.Nil {
		for _, doc := range docs {
			if strings.TrimSpace(doc.FrontMatter.Template) != "" {
				break
			}
		}
		if opts.TemplateID == nil || *opts.TemplateID == uuid.Nil {
			return ErrTemplateMissing
		}
	}

	slug := content.Slug
	existing, err := i.pages.GetBySlug(ctx, slug)
	if err != nil && existing != nil {
		return fmt.Errorf("markdown importer: page lookup %s: %w", slug, err)
	}

	translations := make([]interfaces.PageTranslationInput, 0, len(docs))
	for _, doc := range docs {
		title := strings.TrimSpace(doc.FrontMatter.Title)
		if title == "" {
			title = fallbackTitle(slug)
		}
		path := canonicalPath(doc, slug)
		translations = append(translations, interfaces.PageTranslationInput{
			Locale:  doc.Locale,
			Title:   title,
			Path:    path,
			Summary: optionalString(doc.FrontMatter.Summary),
			Fields: map[string]any{
				"source":      "markdown",
				"frontmatter": doc.FrontMatter.Raw,
			},
		})
	}

	if existing == nil {
		if opts.DryRun {
			return nil
		}

		createReq := interfaces.PageCreateRequest{
			ContentID:    content.ID,
			TemplateID:   templateFor(opts, docs),
			Slug:         slug,
			Status:       content.Status,
			CreatedBy:    opts.AuthorID,
			UpdatedBy:    opts.AuthorID,
			Translations: translations,
			Metadata: map[string]any{
				"source": "markdown",
			},
		}
		record, createErr := i.pages.Create(ctx, createReq)
		if createErr != nil {
			return fmt.Errorf("markdown importer: create page %s: %w", slug, createErr)
		}
		acc.createdPage(record.ID)
		return nil
	}

	if !pageNeedsUpdate(existing.Translations, translations, content.Status) {
		acc.skipPage(existing.ID)
		return nil
	}

	if opts.DryRun {
		acc.skipPage(existing.ID)
		return nil
	}

	updateReq := interfaces.PageUpdateRequest{
		ID:           existing.ID,
		TemplateID:   pointerTemplate(templateFor(opts, docs)),
		Status:       content.Status,
		UpdatedBy:    opts.AuthorID,
		Translations: translations,
		Metadata: map[string]any{
			"source": "markdown",
		},
	}
	record, updateErr := i.pages.Update(ctx, updateReq)
	if updateErr != nil {
		return fmt.Errorf("markdown importer: update page %s: %w", slug, updateErr)
	}
	acc.updatedPage(record.ID)
	return nil
}

func (i *Importer) deleteOrphaned(ctx context.Context, docs map[string][]*interfaces.Document, opts interfaces.SyncOptions, acc *syncAccumulator) error {
	existing, err := i.content.List(ctx)
	if err != nil {
		return fmt.Errorf("markdown importer: list content: %w", err)
	}

	docSlugs := make(map[string]struct{}, len(docs))
	for slug := range docs {
		docSlugs[slug] = struct{}{}
	}

	for _, record := range existing {
		if _, ok := docSlugs[record.Slug]; ok {
			continue
		}
		if opts.DryRun {
			acc.deleted++
			continue
		}
		if err := i.content.Delete(ctx, record.ID); err != nil {
			return fmt.Errorf("markdown importer: delete content %s: %w", record.Slug, err)
		}
		acc.deleted++
	}

	if !opts.ImportOptions.CreatePages || i.pages == nil {
		return nil
	}

	pages, err := i.pages.List(ctx)
	if err != nil {
		return fmt.Errorf("markdown importer: list pages: %w", err)
	}
	for _, page := range pages {
		if _, ok := docSlugs[page.Slug]; ok {
			continue
		}
		if opts.DryRun {
			acc.deleted++
			continue
		}
		if err := i.pages.Delete(ctx, page.ID); err != nil {
			return fmt.Errorf("markdown importer: delete page %s: %w", page.Slug, err)
		}
		acc.deleted++
	}
	return nil
}

func validateDocument(doc *interfaces.Document) error {
	if doc == nil {
		return errors.New("markdown importer: nil document")
	}
	if strings.TrimSpace(doc.FrontMatter.Slug) == "" {
		return ErrSlugMissing
	}
	if strings.TrimSpace(doc.Locale) == "" {
		return ErrLocaleMissing
	}
	return nil
}

func groupKey(doc *interfaces.Document) string {
	if doc == nil {
		return ""
	}
	return strings.TrimSpace(doc.FrontMatter.Slug)
}

func groupBySlug(docs []*interfaces.Document) map[string][]*interfaces.Document {
	result := map[string][]*interfaces.Document{}
	for _, doc := range docs {
		key := groupKey(doc)
		result[key] = append(result[key], doc)
	}
	return result
}

func sortDocuments(docs []*interfaces.Document) []*interfaces.Document {
	slices.SortFunc(docs, func(a, b *interfaces.Document) int {
		if a == nil || b == nil {
			return 0
		}
		if strings.Compare(a.Locale, b.Locale) != 0 {
			return strings.Compare(a.Locale, b.Locale)
		}
		return strings.Compare(a.FilePath, b.FilePath)
	})
	return docs
}

func fallbackTitle(slug string) string {
	if slug == "" {
		return "Untitled"
	}
	return strings.ReplaceAll(strings.Title(strings.ReplaceAll(slug, "-", " ")), "_", " ")
}

func selectStatus(docs []*interfaces.Document) string {
	for _, doc := range docs {
		if doc != nil && strings.TrimSpace(doc.FrontMatter.Status) != "" {
			return doc.FrontMatter.Status
		}
	}
	return "draft"
}

func buildContentFields(doc *interfaces.Document) map[string]any {
	return map[string]any{
		"markdown": map[string]any{
			"body":        string(doc.Body),
			"body_html":   string(doc.BodyHTML),
			"checksum":    hex.EncodeToString(doc.Checksum),
			"frontmatter": doc.FrontMatter.Raw,
			"custom":      doc.FrontMatter.Custom,
		},
		"locale": doc.Locale,
	}
}

func documentMetadata(docs []*interfaces.Document) []map[string]any {
	out := make([]map[string]any, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		out = append(out, map[string]any{
			"path":      doc.FilePath,
			"locale":    doc.Locale,
			"checksum":  hex.EncodeToString(doc.Checksum),
			"template":  doc.FrontMatter.Template,
			"tags":      doc.FrontMatter.Tags,
			"title":     doc.FrontMatter.Title,
			"timestamp": doc.LastModified,
		})
	}
	return out
}

func diffTranslations(existing []interfaces.ContentTranslation, inputs []interfaces.ContentTranslationInput) bool {
	current := map[string]interfaces.ContentTranslation{}
	for _, tr := range existing {
		current[strings.ToLower(tr.Locale)] = tr
	}

	seen := map[string]struct{}{}

	for _, in := range inputs {
		localeKey := strings.ToLower(in.Locale)
		seen[localeKey] = struct{}{}
		currentTr, ok := current[localeKey]
		if !ok {
			return true
		}

		if strings.TrimSpace(in.Title) != strings.TrimSpace(currentTr.Title) {
			return true
		}
		if stringValue(in.Summary) != stringValue(currentTr.Summary) {
			return true
		}
		if checksumFromFields(in.Fields) != checksumFromFields(currentTr.Fields) {
			return true
		}
	}

	return len(current) != len(seen)
}

func pageNeedsUpdate(existing []interfaces.PageTranslation, inputs []interfaces.PageTranslationInput, status string) bool {
	current := map[string]interfaces.PageTranslation{}
	for _, tr := range existing {
		current[strings.ToLower(tr.Locale)] = tr
	}
	seen := map[string]struct{}{}

	for _, in := range inputs {
		locale := strings.ToLower(in.Locale)
		seen[locale] = struct{}{}
		cur, ok := current[locale]
		if !ok {
			return true
		}
		if strings.TrimSpace(cur.Title) != strings.TrimSpace(in.Title) {
			return true
		}
		if strings.TrimSpace(cur.Path) != strings.TrimSpace(in.Path) {
			return true
		}
		if stringValue(in.Summary) != stringValue(cur.Summary) {
			return true
		}
		if !mapsEqual(cur.Fields, in.Fields) {
			return true
		}
	}
	return len(current) != len(seen)
}

func checksumFromFields(fields map[string]any) string {
	markdown, ok := fields["markdown"].(map[string]any)
	if !ok {
		return ""
	}
	checksum, _ := markdown["checksum"].(string)
	return checksum
}

func stringValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func canonicalPath(doc *interfaces.Document, slug string) string {
	if doc != nil {
		if path, ok := doc.FrontMatter.Raw["path"].(string); ok && strings.TrimSpace(path) != "" {
			return path
		}
	}
	if slug == "" {
		return "/"
	}
	if strings.HasPrefix(slug, "/") {
		return slug
	}
	return "/" + slug
}

func templateFor(opts interfaces.ImportOptions, docs []*interfaces.Document) uuid.UUID {
	if opts.TemplateID != nil && *opts.TemplateID != uuid.Nil {
		return *opts.TemplateID
	}
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if tmpl, ok := doc.FrontMatter.Raw["template_id"].(string); ok {
			if parsed, err := uuid.Parse(tmpl); err == nil {
				return parsed
			}
		}
	}
	return uuid.Nil
}

func pointerTemplate(value uuid.UUID) *uuid.UUID {
	if value == uuid.Nil {
		return nil
	}
	out := value
	return &out
}

func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if bVal, ok := b[key]; !ok || fmt.Sprint(bVal) != fmt.Sprint(value) {
			return false
		}
	}
	return true
}

type importAccumulator struct {
	createdIDs  []uuid.UUID
	updatedIDs  []uuid.UUID
	skippedIDs  []uuid.UUID
	pageCreated []uuid.UUID
	pageUpdated []uuid.UUID
	pageSkipped []uuid.UUID
	errors      []error
}

func newImportAccumulator() *importAccumulator {
	return &importAccumulator{
		createdIDs:  []uuid.UUID{},
		updatedIDs:  []uuid.UUID{},
		skippedIDs:  []uuid.UUID{},
		pageCreated: []uuid.UUID{},
		pageUpdated: []uuid.UUID{},
		pageSkipped: []uuid.UUID{},
		errors:      []error{},
	}
}

func (a *importAccumulator) created(id uuid.UUID) {
	if id != uuid.Nil {
		a.createdIDs = append(a.createdIDs, id)
	}
}

func (a *importAccumulator) updated(id uuid.UUID) {
	if id != uuid.Nil {
		a.updatedIDs = append(a.updatedIDs, id)
	}
}

func (a *importAccumulator) skip(id uuid.UUID) {
	if id != uuid.Nil {
		a.skippedIDs = append(a.skippedIDs, id)
	}
}

func (a *importAccumulator) createdPage(id uuid.UUID) {
	if id != uuid.Nil {
		a.pageCreated = append(a.pageCreated, id)
	}
}

func (a *importAccumulator) updatedPage(id uuid.UUID) {
	if id != uuid.Nil {
		a.pageUpdated = append(a.pageUpdated, id)
	}
}

func (a *importAccumulator) skipPage(id uuid.UUID) {
	if id != uuid.Nil {
		a.pageSkipped = append(a.pageSkipped, id)
	}
}

func (a *importAccumulator) addError(err error) {
	if err != nil {
		a.errors = append(a.errors, err)
	}
}

func (a *importAccumulator) result() *interfaces.ImportResult {
	return &interfaces.ImportResult{
		CreatedContentIDs: a.createdIDs,
		UpdatedContentIDs: a.updatedIDs,
		SkippedContentIDs: a.skippedIDs,
		Errors:            a.errors,
	}
}

type syncAccumulator struct {
	created int
	updated int
	deleted int
	skipped int
	errors  []error
}

func newSyncAccumulator() *syncAccumulator {
	return &syncAccumulator{
		errors: []error{},
	}
}

func (s *syncAccumulator) merge(res *interfaces.ImportResult) {
	if res == nil {
		return
	}
	s.created += len(res.CreatedContentIDs)
	s.updated += len(res.UpdatedContentIDs)
	s.skipped += len(res.SkippedContentIDs)
	s.errors = append(s.errors, res.Errors...)
}

func (s *syncAccumulator) addError(err error) {
	if err != nil {
		s.errors = append(s.errors, err)
	}
}

func (s *syncAccumulator) result() *interfaces.SyncResult {
	return &interfaces.SyncResult{
		Created: s.created,
		Updated: s.updated,
		Skipped: s.skipped,
		Errors:  s.errors,
		Deleted: s.deleted,
	}
}

func errSlice(errs []error) []error {
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return filtered
}

func firstError(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errs[0]
}
