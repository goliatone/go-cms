ALTER TABLE content_translations
    ADD COLUMN translation_group_id TEXT;

CREATE INDEX idx_content_translations_group_id
    ON content_translations(translation_group_id);

--bun:split

ALTER TABLE page_translations
    ADD COLUMN translation_group_id TEXT;

CREATE INDEX idx_page_translations_group_id
    ON page_translations(translation_group_id);
