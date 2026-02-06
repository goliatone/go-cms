-- Add primary_locale to content and page records
ALTER TABLE contents
    ADD COLUMN primary_locale TEXT;

ALTER TABLE pages
    ADD COLUMN primary_locale TEXT;

-- Backfill primary_locale from earliest translation per record.
WITH ranked AS (
    SELECT ct.content_id,
           l.code AS locale,
           ROW_NUMBER() OVER (PARTITION BY ct.content_id ORDER BY ct.created_at ASC, ct.id ASC) AS rn
    FROM content_translations ct
    JOIN locales l ON l.id = ct.locale_id
    WHERE ct.deleted_at IS NULL
)
UPDATE contents AS c
SET primary_locale = ranked.locale
FROM ranked
WHERE ranked.content_id = c.id
  AND ranked.rn = 1
  AND (c.primary_locale IS NULL OR c.primary_locale = '');

WITH ranked AS (
    SELECT pt.page_id,
           l.code AS locale,
           ROW_NUMBER() OVER (PARTITION BY pt.page_id ORDER BY pt.created_at ASC, pt.id ASC) AS rn
    FROM page_translations pt
    JOIN locales l ON l.id = pt.locale_id
    WHERE pt.deleted_at IS NULL
)
UPDATE pages AS p
SET primary_locale = ranked.locale
FROM ranked
WHERE ranked.page_id = p.id
  AND ranked.rn = 1
  AND (p.primary_locale IS NULL OR p.primary_locale = '');
