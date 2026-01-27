DROP INDEX IF EXISTS idx_content_types_slug;

-- SQLite does not support dropping columns via ALTER TABLE.
-- No-op for content_types.slug.
