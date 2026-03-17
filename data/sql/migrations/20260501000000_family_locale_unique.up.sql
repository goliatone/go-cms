-- Enforce one locale per translation group for active rows.
CREATE UNIQUE INDEX IF NOT EXISTS idx_content_translations_group_locale_unique
    ON content_translations(family_id, locale_id)
    WHERE family_id IS NOT NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_page_translations_group_locale_unique
    ON page_translations(family_id, locale_id)
    WHERE family_id IS NOT NULL AND deleted_at IS NULL;
