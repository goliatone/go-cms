ALTER TABLE contents
    ADD COLUMN primary_locale TEXT;

ALTER TABLE pages
    ADD COLUMN primary_locale TEXT;

UPDATE contents
SET primary_locale = (
    SELECT l.code
    FROM content_translations ct
    JOIN locales l ON l.id = ct.locale_id
    WHERE ct.content_id = contents.id
      AND ct.deleted_at IS NULL
    ORDER BY ct.created_at ASC, ct.id ASC
    LIMIT 1
)
WHERE primary_locale IS NULL OR primary_locale = '';

UPDATE pages
SET primary_locale = (
    SELECT l.code
    FROM page_translations pt
    JOIN locales l ON l.id = pt.locale_id
    WHERE pt.page_id = pages.id
      AND pt.deleted_at IS NULL
    ORDER BY pt.created_at ASC, pt.id ASC
    LIMIT 1
)
WHERE primary_locale IS NULL OR primary_locale = '';
