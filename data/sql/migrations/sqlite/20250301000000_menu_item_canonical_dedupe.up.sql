-- Add canonical key for menu item deduplication
ALTER TABLE menu_items
    ADD COLUMN canonical_key TEXT;

UPDATE menu_items
SET canonical_key = CASE
    WHEN json_extract(target, '$.type') = 'page' AND json_extract(target, '$.page_id') IS NOT NULL THEN 'page:id:' || json_extract(target, '$.page_id')
    WHEN json_extract(target, '$.type') = 'page' AND json_extract(target, '$.slug') IS NOT NULL THEN 'page:slug:' || json_extract(target, '$.slug')
    WHEN json_extract(target, '$.url') IS NOT NULL THEN 'url:' || json_extract(target, '$.url')
    WHEN json_extract(target, '$.path') IS NOT NULL THEN 'path:' || json_extract(target, '$.path')
    ELSE NULL
END
WHERE canonical_key IS NULL;

-- Drop duplicates before enforcing uniqueness.
DELETE FROM menu_items
WHERE id IN (
    SELECT id FROM (
        SELECT id,
               ROW_NUMBER() OVER (PARTITION BY menu_id, canonical_key ORDER BY created_at) AS rn
        FROM menu_items
        WHERE canonical_key IS NOT NULL
    ) AS dedupe
    WHERE rn > 1
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_menu_items_menu_canonical_key
    ON menu_items(menu_id, canonical_key);
