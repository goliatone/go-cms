ALTER TABLE content_translations
    ADD COLUMN IF NOT EXISTS metadata JSONB;
