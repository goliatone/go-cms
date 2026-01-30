package content_test

import (
	"database/sql"
	"io/fs"
	"strings"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/pkg/testsupport"
)

const defaultEnvID = "00000000-0000-0000-0000-000000000001"

func TestEnvironmentsMigrationBackfill(t *testing.T) {
	db, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	applyMigrationFileEnv(t, db, "20250102000000_initial_schema.up.sql")
	applyMigrationFileEnv(t, db, "20250209000000_menu_navigation_enhancements.up.sql")
	applyMigrationFileEnv(t, db, "20260126000000_content_type_slug.up.sql")
	applyMigrationFileEnv(t, db, "20260126000010_content_slug_unique.up.sql")
	applyMigrationFileEnv(t, db, "20260128000000_content_type_builder_fields.up.sql")
	applyMigrationFileEnv(t, db, "20260301000001_menu_locations.up.sql")
	applyMigrationFileEnv(t, db, "20260401000000_block_definition_versions.up.sql")

	seedBlockDefinition(t, db, "block-1", "Hero Block")

	applyMigrationFileEnv(t, db, "20260415000000_environments.up.sql")

	defaultID := fetchDefaultEnvironmentID(t, db)
	if defaultID == "" {
		t.Fatalf("expected default environment to exist")
	}

	required := []struct {
		table  string
		column string
	}{
		{"content_types", "environment_id"},
		{"contents", "environment_id"},
		{"pages", "environment_id"},
		{"menus", "environment_id"},
		{"menu_items", "environment_id"},
		{"block_definitions", "environment_id"},
		{"block_definitions", "slug"},
	}
	for _, item := range required {
		if !columnExists(t, db, item.table, item.column) {
			t.Fatalf("expected column %s.%s", item.table, item.column)
		}
	}

	if !indexExists(t, db, "content_types", "idx_content_types_env_slug") {
		t.Fatalf("expected idx_content_types_env_slug")
	}
	if !indexExists(t, db, "contents", "idx_contents_env_type_slug") {
		t.Fatalf("expected idx_contents_env_type_slug")
	}
	if !indexExists(t, db, "menus", "idx_menus_env_code") {
		t.Fatalf("expected idx_menus_env_code")
	}
	if !indexExists(t, db, "block_definitions", "idx_block_definitions_env_slug") {
		t.Fatalf("expected idx_block_definitions_env_slug")
	}

	slug := fetchBlockDefinitionSlug(t, db, "block-1")
	if slug != "hero-block" {
		t.Fatalf("expected block slug backfill, got %q", slug)
	}

	// Unique per environment for menus.
	insertEnvironment(t, db, "00000000-0000-0000-0000-000000000002", "secondary")
	insertMenu(t, db, "menu-1", "main", defaultID)
	insertMenu(t, db, "menu-2", "main", "00000000-0000-0000-0000-000000000002")
	if _, err := db.Exec(`INSERT INTO menus (id, code, description, location, environment_id, created_by, updated_by) VALUES ('menu-3', 'main', 'dup', NULL, ?, 'u', 'u')`, defaultID); err == nil {
		t.Fatalf("expected env-scoped menu code uniqueness")
	}
}

func TestEnvironmentsMigrationBackfillsRecords(t *testing.T) {
	db, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	applyMigrationFileEnv(t, db, "20250102000000_initial_schema.up.sql")
	applyMigrationFileEnv(t, db, "20250209000000_menu_navigation_enhancements.up.sql")
	applyMigrationFileEnv(t, db, "20260126000000_content_type_slug.up.sql")
	applyMigrationFileEnv(t, db, "20260126000010_content_slug_unique.up.sql")
	applyMigrationFileEnv(t, db, "20260128000000_content_type_builder_fields.up.sql")
	applyMigrationFileEnv(t, db, "20260301000001_menu_locations.up.sql")
	applyMigrationFileEnv(t, db, "20260401000000_block_definition_versions.up.sql")

	insertContentType(t, db, "ct-1", "Article", "article")
	insertContent(t, db, "c-1", "ct-1", "welcome")
	seedThemeAndTemplate(t, db)
	insertPage(t, db, "p-1", "c-1", "tpl-1", "home")
	insertMenuLegacy(t, db, "menu-1", "main")
	insertMenuItemLegacy(t, db, "item-1", "menu-1")
	seedBlockDefinition(t, db, "block-1", "Hero Block")

	applyMigrationFileEnv(t, db, "20260415000000_environments.up.sql")

	defaultID := fetchDefaultEnvironmentID(t, db)
	if defaultID == "" {
		t.Fatalf("expected default environment to exist")
	}

	if got := fetchEnvironmentID(t, db, "content_types", "ct-1"); got != defaultID {
		t.Fatalf("expected content type env %s, got %s", defaultID, got)
	}
	if got := fetchEnvironmentID(t, db, "contents", "c-1"); got != defaultID {
		t.Fatalf("expected content env %s, got %s", defaultID, got)
	}
	if got := fetchEnvironmentID(t, db, "pages", "p-1"); got != defaultID {
		t.Fatalf("expected page env %s, got %s", defaultID, got)
	}
	menuEnv := fetchEnvironmentID(t, db, "menus", "menu-1")
	if menuEnv != defaultID {
		t.Fatalf("expected menu env %s, got %s", defaultID, menuEnv)
	}
	itemEnv := fetchEnvironmentID(t, db, "menu_items", "item-1")
	if itemEnv != menuEnv {
		t.Fatalf("expected menu item env %s, got %s", menuEnv, itemEnv)
	}
	if got := fetchEnvironmentID(t, db, "block_definitions", "block-1"); got != defaultID {
		t.Fatalf("expected block env %s, got %s", defaultID, got)
	}
}

func TestEnvironmentsMigrationConstraints(t *testing.T) {
	db, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()

	applyMigrationFileEnv(t, db, "20250102000000_initial_schema.up.sql")
	applyMigrationFileEnv(t, db, "20250209000000_menu_navigation_enhancements.up.sql")
	applyMigrationFileEnv(t, db, "20260126000000_content_type_slug.up.sql")
	applyMigrationFileEnv(t, db, "20260126000010_content_slug_unique.up.sql")
	applyMigrationFileEnv(t, db, "20260128000000_content_type_builder_fields.up.sql")
	applyMigrationFileEnv(t, db, "20260301000001_menu_locations.up.sql")
	applyMigrationFileEnv(t, db, "20260401000000_block_definition_versions.up.sql")
	applyMigrationFileEnv(t, db, "20260415000000_environments.up.sql")

	insertEnvironment(t, db, "00000000-0000-0000-0000-000000000002", "secondary")

	if _, err := db.Exec(`INSERT INTO content_types (id, name, slug, schema, environment_id) VALUES ('ct-1', 'Article', 'article', '{}', ?)`, defaultEnvID); err != nil {
		t.Fatalf("insert content type: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO contents (id, content_type_id, status, slug, created_by, updated_by, environment_id) VALUES ('c-1', 'ct-1', 'draft', 'post', 'u', 'u', ?)`, defaultEnvID); err != nil {
		t.Fatalf("insert content: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO contents (id, content_type_id, status, slug, created_by, updated_by, environment_id) VALUES ('c-2', 'ct-1', 'draft', 'post-2', 'u', 'u', ?)`, "00000000-0000-0000-0000-000000000002"); err == nil {
		t.Fatalf("expected content env mismatch to fail")
	}

	seedThemeAndTemplate(t, db)
	if _, err := db.Exec(`INSERT INTO pages (id, content_id, template_id, slug, status, created_by, updated_by, environment_id) VALUES ('p-1', 'c-1', 'tpl-1', 'home', 'draft', 'u', 'u', ?)`, defaultEnvID); err != nil {
		t.Fatalf("insert page: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO pages (id, content_id, template_id, slug, status, created_by, updated_by, environment_id) VALUES ('p-2', 'c-1', 'tpl-1', 'about', 'draft', 'u', 'u', ?)`, "00000000-0000-0000-0000-000000000002"); err == nil {
		t.Fatalf("expected page env mismatch to fail")
	}
}

func seedBlockDefinition(t *testing.T, db *sql.DB, id, name string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO block_definitions (id, name, schema, defaults, editor_style_url, frontend_style_url) VALUES (?, ?, '{}', '{}', '', '')`, id, name); err != nil {
		t.Fatalf("insert block definition: %v", err)
	}
}

func fetchBlockDefinitionSlug(t *testing.T, db *sql.DB, id string) string {
	t.Helper()
	var slug string
	if err := db.QueryRow(`SELECT slug FROM block_definitions WHERE id = ?`, id).Scan(&slug); err != nil {
		t.Fatalf("select block slug: %v", err)
	}
	return slug
}

func fetchDefaultEnvironmentID(t *testing.T, db *sql.DB) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`SELECT id FROM environments WHERE key = 'default'`).Scan(&id); err != nil {
		t.Fatalf("select default environment: %v", err)
	}
	return id
}

func insertEnvironment(t *testing.T, db *sql.DB, id, key string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO environments (id, key, name, description, is_active, is_default) VALUES (?, ?, ?, '', 1, 0)`, id, key, key); err != nil {
		t.Fatalf("insert environment: %v", err)
	}
}

func insertMenu(t *testing.T, db *sql.DB, id, code, envID string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO menus (id, code, description, location, environment_id, created_by, updated_by) VALUES (?, ?, '', NULL, ?, 'u', 'u')`, id, code, envID); err != nil {
		t.Fatalf("insert menu: %v", err)
	}
}

func insertMenuLegacy(t *testing.T, db *sql.DB, id, code string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO menus (id, code, description, location, created_by, updated_by) VALUES (?, ?, '', NULL, 'u', 'u')`, id, code); err != nil {
		t.Fatalf("insert menu: %v", err)
	}
}

func insertMenuItemLegacy(t *testing.T, db *sql.DB, id, menuID string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO menu_items (id, menu_id, parent_id, position, target, created_by, updated_by) VALUES (?, ?, NULL, 0, '{}', 'u', 'u')`, id, menuID); err != nil {
		t.Fatalf("insert menu item: %v", err)
	}
}

func insertContentType(t *testing.T, db *sql.DB, id, name, slug string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO content_types (id, name, slug, schema) VALUES (?, ?, ?, '{}')`, id, name, slug); err != nil {
		t.Fatalf("insert content type: %v", err)
	}
}

func insertContent(t *testing.T, db *sql.DB, id, typeID, slug string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO contents (id, content_type_id, status, slug, created_by, updated_by) VALUES (?, ?, 'draft', ?, 'u', 'u')`, id, typeID, slug); err != nil {
		t.Fatalf("insert content: %v", err)
	}
}

func insertPage(t *testing.T, db *sql.DB, id, contentID, templateID, slug string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO pages (id, content_id, template_id, slug, status, created_by, updated_by) VALUES (?, ?, ?, ?, 'draft', 'u', 'u')`, id, contentID, templateID, slug); err != nil {
		t.Fatalf("insert page: %v", err)
	}
}

func seedThemeAndTemplate(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO themes (id, name, description, version, author, is_active, theme_path, config) VALUES ('theme-1', 'Theme', '', '1.0.0', 'author', 1, 'themes/theme-1', '{}')`); err != nil {
		t.Fatalf("insert theme: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO templates (id, theme_id, name, slug, description, template_path, regions, metadata) VALUES ('tpl-1', 'theme-1', 'Template', 'base', '', 'templates/base.html', '[]', '{}')`); err != nil {
		t.Fatalf("insert template: %v", err)
	}
}

func fetchEnvironmentID(t *testing.T, db *sql.DB, table, id string) string {
	t.Helper()
	var envID string
	query := `SELECT environment_id FROM ` + table + ` WHERE id = ?`
	if err := db.QueryRow(query, id).Scan(&envID); err != nil {
		t.Fatalf("select environment_id from %s: %v", table, err)
	}
	return envID
}

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("pragma table_info %s: %v", table, err)
	}
	defer rows.Close()
	var (
		cid     int
		name    string
		typeVal string
		notnull int
		dflt    sql.NullString
		pk      int
	)
	for rows.Next() {
		if err := rows.Scan(&cid, &name, &typeVal, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		if name == column {
			return true
		}
	}
	return false
}

func indexExists(t *testing.T, db *sql.DB, table, index string) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA index_list(` + table + `)`)
	if err != nil {
		t.Fatalf("pragma index_list %s: %v", table, err)
	}
	defer rows.Close()
	var (
		seq     int
		name    string
		unique  int
		origin  string
		partial int
	)
	for rows.Next() {
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan index_list: %v", err)
		}
		if name == index {
			return true
		}
	}
	return false
}

func applyMigrationFileEnv(t *testing.T, db *sql.DB, name string) {
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
