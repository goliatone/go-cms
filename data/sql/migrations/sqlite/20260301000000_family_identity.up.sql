ALTER TABLE content_translations
    ADD COLUMN family_id TEXT;

CREATE INDEX idx_content_translations_group_id
    ON content_translations(family_id);

--bun:split

ALTER TABLE page_translations
    ADD COLUMN family_id TEXT;

CREATE INDEX idx_page_translations_group_id
    ON page_translations(family_id);
