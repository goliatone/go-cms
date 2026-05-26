package content_test

import (
	"database/sql"
	"io/fs"
	"strings"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
)

func TestContentTypeSlugMigrationBackfill(t *testing.T) {
	db, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	applyMigrationFile(t, db, "20250102000000_initial_schema.up.sql")

	schema := `{"fields":[]}`
	id1 := uuid.NewString()
	id2 := uuid.NewString()

	if _, err := db.Exec(`INSERT INTO content_types (id, name, schema) VALUES (?, ?, ?)`, id1, "Landing Page", schema); err != nil {
		t.Fatalf("insert content type 1: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO content_types (id, name, schema) VALUES (?, ?, ?)`, id2, "Landing Page", schema); err != nil {
		t.Fatalf("insert content type 2: %v", err)
	}

	applyMigrationFile(t, db, "20260126000000_content_type_slug.up.sql")

	slug1 := fetchContentTypeSlug(t, db, id1)
	slug2 := fetchContentTypeSlug(t, db, id2)

	if !strings.HasPrefix(slug1, "landing-page") {
		t.Fatalf("expected slug1 to start with landing-page, got %q", slug1)
	}
	if !strings.HasPrefix(slug2, "landing-page") {
		t.Fatalf("expected slug2 to start with landing-page, got %q", slug2)
	}
	if slug1 == slug2 {
		t.Fatalf("expected unique slugs after backfill, got %q", slug1)
	}

	if _, err := db.Exec(`INSERT INTO content_types (id, name, slug, schema) VALUES (?, ?, ?, ?)`, uuid.NewString(), "Duplicate", slug1, schema); err == nil {
		t.Fatalf("expected unique slug constraint error")
	}
}

func TestContentTypeSlugMigrationBackfillUsesIDFallback(t *testing.T) {
	db, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	applyMigrationFile(t, db, "20250102000000_initial_schema.up.sql")

	id := uuid.NewString()
	if _, err := db.Exec(`INSERT INTO content_types (id, name, schema) VALUES (?, ?, ?)`, id, "", `{"fields":[]}`); err != nil {
		t.Fatalf("insert content type: %v", err)
	}

	applyMigrationFile(t, db, "20260126000000_content_type_slug.up.sql")

	slug := fetchContentTypeSlug(t, db, id)
	if wantPrefix := id[:8]; !strings.HasPrefix(slug, wantPrefix) {
		t.Fatalf("expected slug to start with %q, got %q", wantPrefix, slug)
	}
}

func TestContentTranslationMetadataHygieneMigrationRepairsJSONNull(t *testing.T) {
	db, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE content_translations (id TEXT PRIMARY KEY, metadata TEXT)`); err != nil {
		t.Fatalf("create content_translations: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO content_translations (id, metadata) VALUES (?, ?)`, "json-null", "null"); err != nil {
		t.Fatalf("insert json-null metadata: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO content_translations (id, metadata) VALUES (?, ?)`, "object", `{"workflow":true}`); err != nil {
		t.Fatalf("insert object metadata: %v", err)
	}

	applyMigrationFile(t, db, "20260526000000_content_translation_metadata_hygiene.up.sql")

	var repaired sql.NullString
	if err := db.QueryRow(`SELECT metadata FROM content_translations WHERE id = ?`, "json-null").Scan(&repaired); err != nil {
		t.Fatalf("select repaired metadata: %v", err)
	}
	if repaired.Valid {
		t.Fatalf("expected JSON null metadata to be repaired to SQL NULL, got %q", repaired.String)
	}

	var object string
	if err := db.QueryRow(`SELECT metadata FROM content_translations WHERE id = ?`, "object").Scan(&object); err != nil {
		t.Fatalf("select object metadata: %v", err)
	}
	if object != `{"workflow":true}` {
		t.Fatalf("expected object metadata to remain unchanged, got %q", object)
	}
}

func fetchContentTypeSlug(t *testing.T, db *sql.DB, id string) string {
	t.Helper()
	var slug string
	if err := db.QueryRow(`SELECT slug FROM content_types WHERE id = ?`, id).Scan(&slug); err != nil {
		t.Fatalf("select slug for %s: %v", id, err)
	}
	return slug
}

func applyMigrationFile(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	paths := []string{
		"data/sql/migrations/sqlite/" + name,
		"data/sql/migrations/" + name,
	}
	var raw []byte
	var err error
	for _, path := range paths {
		raw, err = fs.ReadFile(cms.GetMigrationsFS(), path)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	content := string(raw)
	// SQLite doesn't understand Postgres JSONB casts in defaults.
	content = strings.ReplaceAll(content, "::jsonb", "")
	content = strings.ReplaceAll(content, "::JSONB", "")
	for chunk := range strings.SplitSeq(content, "---bun:split") {
		statement := strings.TrimSpace(chunk)
		if statement == "" {
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec migration %s: %v", name, err)
		}
	}
}
