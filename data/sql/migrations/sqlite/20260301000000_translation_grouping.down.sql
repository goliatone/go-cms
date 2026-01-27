DROP INDEX IF EXISTS idx_content_translations_group_id;
DROP INDEX IF EXISTS idx_page_translations_group_id;

-- SQLite does not support dropping columns via ALTER TABLE.
-- No-op for content_translations.translation_group_id and page_translations.translation_group_id.
