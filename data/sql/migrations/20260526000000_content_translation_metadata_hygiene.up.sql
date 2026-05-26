UPDATE content_translations
SET metadata = NULL
WHERE metadata = 'null'::jsonb;

ALTER TABLE content_translations
    DROP CONSTRAINT IF EXISTS content_translations_metadata_object_check;

ALTER TABLE content_translations
    ADD CONSTRAINT content_translations_metadata_object_check
    CHECK (metadata IS NULL OR jsonb_typeof(metadata) = 'object');

