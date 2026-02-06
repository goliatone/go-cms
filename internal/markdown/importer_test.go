package markdown

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

func TestImportCreatesContent(t *testing.T) {
	contentStub := newStubContentService()

	svc := newImportService(t, contentStub)

	doc, err := svc.Load(context.Background(), "en/about.md", interfaces.LoadOptions{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := interfaces.ImportOptions{
		ContentTypeID: uuid.New(),
		AuthorID:      uuid.New(),
	}

	result, err := svc.Import(context.Background(), doc, opts)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(result.CreatedContentIDs) != 1 {
		t.Fatalf("expected created content, got %#v", result)
	}

	record := contentStub.records["about"]
	if record == nil {
		t.Fatalf("content not stored")
	}
	if record.Translation.Requested == nil {
		t.Fatalf("expected requested translation")
	}
	md := record.Translation.Requested.Fields["markdown"].(map[string]any)
	if md["checksum"] == "" {
		t.Fatalf("expected checksum stored")
	}

}

func TestImportUpdatesExistingTranslations(t *testing.T) {
	contentStub := newStubContentService()
	svc := newImportService(t, contentStub)

	doc, err := svc.Load(context.Background(), "en/about.md", interfaces.LoadOptions{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	opts := interfaces.ImportOptions{
		ContentTypeID: uuid.New(),
		AuthorID:      uuid.New(),
	}

	if _, err := svc.Import(context.Background(), doc, opts); err != nil {
		t.Fatalf("initial import: %v", err)
	}

	// Modify document body and checksum.
	clone := cloneDocument(doc)
	clone.Body = []byte("# Updated\n\nNew body")
	clone.BodyHTML = []byte("<h1>Updated</h1>\n<p>New body</p>\n")
	sum := sha256.Sum256(clone.Body)
	clone.Checksum = sum[:]

	result, err := svc.Import(context.Background(), clone, opts)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if len(result.UpdatedContentIDs) != 1 {
		t.Fatalf("expected updated content, got %#v", result)
	}

	record := contentStub.records["about"]
	if record == nil {
		t.Fatalf("content missing after update")
	}
	if record.Translation.Requested == nil {
		t.Fatalf("expected requested translation after update")
	}
	md := record.Translation.Requested.Fields["markdown"].(map[string]any)
	if md["checksum"] != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum not updated")
	}
}

func TestSyncDeletesOrphans(t *testing.T) {
	contentStub := newStubContentService()
	svc := newImportService(t, contentStub)

	opts := interfaces.ImportOptions{
		ContentTypeID: uuid.New(),
		AuthorID:      uuid.New(),
	}

	if _, err := svc.ImportDirectory(context.Background(), ".", opts); err != nil {
		t.Fatalf("initial import: %v", err)
	}

	// Seed orphan content/page.
	orphanID := uuid.New()
	contentStub.records["orphan"] = &interfaces.ContentRecord{
		ID:       orphanID,
		Slug:     "orphan",
		Status:   "draft",
		Metadata: map[string]any{},
	}
	syncRes, err := svc.Sync(context.Background(), ".", interfaces.SyncOptions{
		ImportOptions: interfaces.ImportOptions{
			ContentTypeID: opts.ContentTypeID,
			AuthorID:      opts.AuthorID,
		},
		DeleteOrphaned: true,
		UpdateExisting: true,
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if _, ok := contentStub.records["orphan"]; ok {
		t.Fatalf("expected orphan content removed")
	}
	if syncRes.Deleted == 0 {
		t.Fatalf("expected deleted count > 0")
	}
}

// Helper constructors --------------------------------------------------------

func newImportService(tb testing.TB, contentSvc *stubContentService, opts ...ServiceOption) *Service {
	tb.Helper()

	cfg := Config{
		BasePath:      filepath.Join("testdata", "site"),
		DefaultLocale: "en",
		Locales:       []string{"en", "es"},
		Pattern:       "*.md",
		Recursive:     true,
	}

	serviceOpts := []ServiceOption{
		WithContentService(contentSvc),
	}
	serviceOpts = append(serviceOpts, opts...)

	svc, err := NewService(cfg, nil, serviceOpts...)
	if err != nil {
		tb.Fatalf("NewService: %v", err)
	}
	return svc
}

func cloneDocument(doc *interfaces.Document) *interfaces.Document {
	if doc == nil {
		return nil
	}
	body := make([]byte, len(doc.Body))
	copy(body, doc.Body)
	html := make([]byte, len(doc.BodyHTML))
	copy(html, doc.BodyHTML)
	checksum := make([]byte, len(doc.Checksum))
	copy(checksum, doc.Checksum)
	return &interfaces.Document{
		FilePath:     doc.FilePath,
		Locale:       doc.Locale,
		FrontMatter:  doc.FrontMatter,
		Body:         body,
		BodyHTML:     html,
		LastModified: time.Now(),
		Checksum:     checksum,
	}
}

// Stub implementations -------------------------------------------------------

type stubContentService struct {
	records      map[string]*interfaces.ContentRecord
	translations map[uuid.UUID][]interfaces.ContentTranslation
}

func newStubContentService() *stubContentService {
	return &stubContentService{
		records:      map[string]*interfaces.ContentRecord{},
		translations: map[uuid.UUID][]interfaces.ContentTranslation{},
	}
}

func (s *stubContentService) Create(_ context.Context, req interfaces.ContentCreateRequest) (*interfaces.ContentRecord, error) {
	id := uuid.New()
	translations := make([]interfaces.ContentTranslation, len(req.Translations))
	for i, tr := range req.Translations {
		translations[i] = interfaces.ContentTranslation{
			ID:      uuid.New(),
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: tr.Summary,
			Fields:  cloneMapAny(tr.Fields),
		}
	}
	record := &interfaces.ContentRecord{
		ID:           id,
		ContentType:  req.ContentTypeID,
		Slug:         req.Slug,
		Status:       req.Status,
		Metadata:     cloneMapAny(req.Metadata),
	}
	record.Translation = buildStubTranslationBundle(translations, interfaces.ContentReadOptions{
		Locale:                   firstLocale(translations),
		IncludeAvailableLocales:  true,
		AllowMissingTranslations: true,
	})
	s.records[req.Slug] = record
	s.translations[id] = translations
	return cloneContentRecord(record), nil
}

func (s *stubContentService) Update(_ context.Context, req interfaces.ContentUpdateRequest) (*interfaces.ContentRecord, error) {
	var record *interfaces.ContentRecord
	translations := make([]interfaces.ContentTranslation, len(req.Translations))
	for i, tr := range req.Translations {
		translations[i] = interfaces.ContentTranslation{
			ID:      uuid.New(),
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: tr.Summary,
			Fields:  cloneMapAny(tr.Fields),
		}
	}
	for slug, existing := range s.records {
		if existing.ID == req.ID {
			record = existing
			record.Status = req.Status
			record.Metadata = cloneMapAny(req.Metadata)
			record.Translation = buildStubTranslationBundle(translations, interfaces.ContentReadOptions{
				Locale:                   firstLocale(translations),
				IncludeAvailableLocales:  true,
				AllowMissingTranslations: true,
			})
			s.records[slug] = record
			s.translations[record.ID] = translations
			break
		}
	}
	if record == nil {
		return nil, errors.New("record not found")
	}
	return cloneContentRecord(record), nil
}

func (s *stubContentService) GetBySlug(_ context.Context, slug string, opts interfaces.ContentReadOptions) (*interfaces.ContentRecord, error) {
	if record, ok := s.records[slug]; ok {
		cloned := cloneContentRecord(record)
		cloned.Translation = buildStubTranslationBundle(s.translations[record.ID], opts)
		return cloned, nil
	}
	return nil, nil
}

func (s *stubContentService) List(_ context.Context, opts interfaces.ContentReadOptions) ([]*interfaces.ContentRecord, error) {
	result := make([]*interfaces.ContentRecord, 0, len(s.records))
	for _, record := range s.records {
		cloned := cloneContentRecord(record)
		cloned.Translation = buildStubTranslationBundle(s.translations[record.ID], opts)
		result = append(result, cloned)
	}
	return result, nil
}

func (s *stubContentService) CheckTranslations(context.Context, uuid.UUID, []string, interfaces.TranslationCheckOptions) ([]string, error) {
	return nil, nil
}

func (s *stubContentService) AvailableLocales(_ context.Context, id uuid.UUID, _ interfaces.TranslationCheckOptions) ([]string, error) {
	return collectTranslationLocales(s.translations[id]), nil
}

func (s *stubContentService) Delete(_ context.Context, req interfaces.ContentDeleteRequest) error {
	for slug, record := range s.records {
		if record.ID == req.ID {
			delete(s.records, slug)
			delete(s.translations, record.ID)
			return nil
		}
	}
	return nil
}

func (s *stubContentService) UpdateTranslation(_ context.Context, req interfaces.ContentUpdateTranslationRequest) (*interfaces.ContentTranslation, error) {
	for _, record := range s.records {
		if record.ID != req.ContentID {
			continue
		}
		translations := s.translations[record.ID]
		for idx, tr := range translations {
			if strings.EqualFold(tr.Locale, req.Locale) {
				updated := interfaces.ContentTranslation{
					ID:      tr.ID,
					Locale:  req.Locale,
					Title:   req.Title,
					Summary: req.Summary,
					Fields:  cloneMapAny(req.Fields),
				}
				translations[idx] = updated
				s.translations[record.ID] = translations
				record.Translation = buildStubTranslationBundle(translations, interfaces.ContentReadOptions{
					Locale:                   req.Locale,
					IncludeAvailableLocales:  true,
					AllowMissingTranslations: true,
				})
				return cloneContentTranslation(updated), nil
			}
		}
		return nil, errors.New("translation not found")
	}
	return nil, errors.New("content not found")
}

func (s *stubContentService) DeleteTranslation(_ context.Context, req interfaces.ContentDeleteTranslationRequest) error {
	for _, record := range s.records {
		if record.ID != req.ContentID {
			continue
		}
		current := s.translations[record.ID]
		newTranslations := make([]interfaces.ContentTranslation, 0, len(current))
		for _, tr := range current {
			if strings.EqualFold(tr.Locale, req.Locale) {
				continue
			}
			newTranslations = append(newTranslations, tr)
		}
		if len(newTranslations) == len(current) {
			return errors.New("translation not found")
		}
		s.translations[record.ID] = newTranslations
		record.Translation = buildStubTranslationBundle(newTranslations, interfaces.ContentReadOptions{
			Locale:                   firstLocale(newTranslations),
			IncludeAvailableLocales:  true,
			AllowMissingTranslations: true,
		})
		return nil
	}
	return errors.New("content not found")
}

// Helper cloning functions ---------------------------------------------------

func cloneContentRecord(record *interfaces.ContentRecord) *interfaces.ContentRecord {
	if record == nil {
		return nil
	}
	out := &interfaces.ContentRecord{
		ID:              record.ID,
		ContentType:     record.ContentType,
		ContentTypeSlug: record.ContentTypeSlug,
		Slug:            record.Slug,
		Status:          record.Status,
		Metadata:        cloneMapAny(record.Metadata),
		Translation:     cloneTranslationBundle(record.Translation),
	}
	return out
}

func cloneContentTranslation(tr interfaces.ContentTranslation) *interfaces.ContentTranslation {
	return &interfaces.ContentTranslation{
		ID:      tr.ID,
		Locale:  tr.Locale,
		Title:   tr.Title,
		Summary: tr.Summary,
		Fields:  cloneMapAny(tr.Fields),
	}
}

func cloneTranslationBundle(bundle interfaces.TranslationBundle[interfaces.ContentTranslation]) interfaces.TranslationBundle[interfaces.ContentTranslation] {
	out := interfaces.TranslationBundle[interfaces.ContentTranslation]{
		Meta: bundle.Meta,
	}
	if bundle.Requested != nil {
		out.Requested = cloneContentTranslation(*bundle.Requested)
	}
	if bundle.Resolved != nil {
		out.Resolved = cloneContentTranslation(*bundle.Resolved)
	}
	return out
}

func buildStubTranslationBundle(translations []interfaces.ContentTranslation, opts interfaces.ContentReadOptions) interfaces.TranslationBundle[interfaces.ContentTranslation] {
	requested := strings.TrimSpace(opts.Locale)
	fallback := strings.TrimSpace(opts.FallbackLocale)
	meta := interfaces.TranslationMeta{
		RequestedLocale: requested,
	}
	if opts.IncludeAvailableLocales {
		meta.AvailableLocales = collectTranslationLocales(translations)
	}

	var requestedTr *interfaces.ContentTranslation
	var resolvedTr *interfaces.ContentTranslation
	resolvedLocale := ""

	if requested != "" {
		if tr := findTranslation(translations, requested); tr != nil {
			requestedTr = tr
			resolvedTr = tr
			resolvedLocale = tr.Locale
		}
	}
	if requestedTr == nil && requested != "" && fallback != "" {
		if tr := findTranslation(translations, fallback); tr != nil {
			resolvedTr = tr
			resolvedLocale = tr.Locale
			meta.FallbackUsed = true
		}
	}

	meta.ResolvedLocale = strings.TrimSpace(resolvedLocale)
	meta.MissingRequestedLocale = requested != "" && requestedTr == nil

	return interfaces.TranslationBundle[interfaces.ContentTranslation]{
		Meta:      meta,
		Requested: requestedTr,
		Resolved:  resolvedTr,
	}
}

func findTranslation(translations []interfaces.ContentTranslation, locale string) *interfaces.ContentTranslation {
	if locale == "" {
		return nil
	}
	for _, tr := range translations {
		if strings.EqualFold(strings.TrimSpace(tr.Locale), locale) {
			return cloneContentTranslation(tr)
		}
	}
	return nil
}

func collectTranslationLocales(translations []interfaces.ContentTranslation) []string {
	if len(translations) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	locales := make([]string, 0, len(translations))
	for _, tr := range translations {
		locale := strings.TrimSpace(tr.Locale)
		if locale == "" {
			continue
		}
		key := strings.ToLower(locale)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		locales = append(locales, locale)
	}
	if len(locales) == 0 {
		return nil
	}
	return locales
}

func firstLocale(translations []interfaces.ContentTranslation) string {
	for _, tr := range translations {
		if locale := strings.TrimSpace(tr.Locale); locale != "" {
			return locale
		}
	}
	return ""
}

func cloneMapAny(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
