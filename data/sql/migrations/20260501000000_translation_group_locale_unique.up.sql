-- Enforce one locale per translation group for active rows.
CREATE UNIQUE INDEX IF NOT EXISTS idx_content_translations_group_locale_unique
    ON content_translations(translation_group_id, locale_id)
    WHERE translation_group_id IS NOT NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_page_translations_group_locale_unique
    ON page_translations(translation_group_id, locale_id)
    WHERE translation_group_id IS NOT NULL AND deleted_at IS NULL;
