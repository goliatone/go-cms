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
	if len(record.Translations) != 1 {
		t.Fatalf("expected 1 translation, got %d", len(record.Translations))
	}
	md := record.Translations[0].Fields["markdown"].(map[string]any)
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
	md := record.Translations[0].Fields["markdown"].(map[string]any)
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
	records map[string]*interfaces.ContentRecord
}

func newStubContentService() *stubContentService {
	return &stubContentService{
		records: map[string]*interfaces.ContentRecord{},
	}
}

func (s *stubContentService) Create(_ context.Context, req interfaces.ContentCreateRequest) (*interfaces.ContentRecord, error) {
	id := uuid.New()
	record := &interfaces.ContentRecord{
		ID:           id,
		ContentType:  req.ContentTypeID,
		Slug:         req.Slug,
		Status:       req.Status,
		Translations: make([]interfaces.ContentTranslation, len(req.Translations)),
		Metadata:     cloneMapAny(req.Metadata),
	}
	for i, tr := range req.Translations {
		record.Translations[i] = interfaces.ContentTranslation{
			ID:      uuid.New(),
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: tr.Summary,
			Fields:  cloneMapAny(tr.Fields),
		}
	}
	s.records[req.Slug] = record
	return cloneContentRecord(record), nil
}

func (s *stubContentService) Update(_ context.Context, req interfaces.ContentUpdateRequest) (*interfaces.ContentRecord, error) {
	var record *interfaces.ContentRecord
	for slug, existing := range s.records {
		if existing.ID == req.ID {
			record = existing
			record.Status = req.Status
			record.Metadata = cloneMapAny(req.Metadata)
			record.Translations = make([]interfaces.ContentTranslation, len(req.Translations))
			for i, tr := range req.Translations {
				record.Translations[i] = interfaces.ContentTranslation{
					ID:      uuid.New(),
					Locale:  tr.Locale,
					Title:   tr.Title,
					Summary: tr.Summary,
					Fields:  cloneMapAny(tr.Fields),
				}
			}
			s.records[slug] = record
			break
		}
	}
	if record == nil {
		return nil, errors.New("record not found")
	}
	return cloneContentRecord(record), nil
}

func (s *stubContentService) GetBySlug(_ context.Context, slug string, _ ...string) (*interfaces.ContentRecord, error) {
	if record, ok := s.records[slug]; ok {
		return cloneContentRecord(record), nil
	}
	return nil, nil
}

func (s *stubContentService) List(context.Context, ...string) ([]*interfaces.ContentRecord, error) {
	result := make([]*interfaces.ContentRecord, 0, len(s.records))
	for _, record := range s.records {
		result = append(result, cloneContentRecord(record))
	}
	return result, nil
}

func (s *stubContentService) Delete(_ context.Context, req interfaces.ContentDeleteRequest) error {
	for slug, record := range s.records {
		if record.ID == req.ID {
			delete(s.records, slug)
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
		for idx, tr := range record.Translations {
			if strings.EqualFold(tr.Locale, req.Locale) {
				updated := interfaces.ContentTranslation{
					ID:      tr.ID,
					Locale:  req.Locale,
					Title:   req.Title,
					Summary: req.Summary,
					Fields:  cloneMapAny(req.Fields),
				}
				record.Translations[idx] = updated
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
		newTranslations := make([]interfaces.ContentTranslation, 0, len(record.Translations))
		for _, tr := range record.Translations {
			if strings.EqualFold(tr.Locale, req.Locale) {
				continue
			}
			newTranslations = append(newTranslations, tr)
		}
		if len(newTranslations) == len(record.Translations) {
			return errors.New("translation not found")
		}
		record.Translations = newTranslations
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
		Translations:    make([]interfaces.ContentTranslation, len(record.Translations)),
	}
	for i, tr := range record.Translations {
		out.Translations[i] = interfaces.ContentTranslation{
			ID:      tr.ID,
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: tr.Summary,
			Fields:  cloneMapAny(tr.Fields),
		}
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
