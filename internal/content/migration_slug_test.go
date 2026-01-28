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
	for _, chunk := range strings.Split(content, "---bun:split") {
		statement := strings.TrimSpace(chunk)
		if statement == "" {
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec migration %s: %v", name, err)
		}
	}
}
