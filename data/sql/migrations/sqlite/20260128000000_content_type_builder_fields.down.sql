DROP INDEX IF EXISTS idx_content_types_slug;
CREATE UNIQUE INDEX idx_content_types_slug ON content_types(slug);

-- SQLite does not support dropping columns via ALTER TABLE.
-- No-op for content_types ui_schema/schema_version/status/deleted_at.
