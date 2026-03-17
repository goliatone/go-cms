DROP INDEX IF EXISTS idx_content_translations_group_id;
ALTER TABLE content_translations
    DROP COLUMN IF EXISTS family_id;

--bun:split

DROP INDEX IF EXISTS idx_page_translations_group_id;
ALTER TABLE page_translations
    DROP COLUMN IF EXISTS family_id;
