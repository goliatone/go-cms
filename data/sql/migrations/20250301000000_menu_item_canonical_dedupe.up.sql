-- Add canonical key for menu item deduplication
ALTER TABLE menu_items
    ADD COLUMN canonical_key TEXT;

UPDATE menu_items
SET canonical_key = CASE
    WHEN target ->> 'type' = 'page' AND target ->> 'page_id' IS NOT NULL THEN 'page:id:' || target ->> 'page_id'
    WHEN target ->> 'type' = 'page' AND target ->> 'slug' IS NOT NULL THEN 'page:slug:' || target ->> 'slug'
    WHEN target ->> 'url' IS NOT NULL THEN 'url:' || target ->> 'url'
    WHEN target ->> 'path' IS NOT NULL THEN 'path:' || target ->> 'path'
    ELSE NULL
END
WHERE canonical_key IS NULL;

-- Drop duplicates before enforcing uniqueness.
DELETE FROM menu_items mi
USING (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY menu_id, canonical_key ORDER BY created_at) AS rn
    FROM menu_items
    WHERE canonical_key IS NOT NULL
) dup
WHERE mi.id = dup.id
  AND dup.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS idx_menu_items_menu_canonical_key
    ON menu_items(menu_id, canonical_key)
    WHERE canonical_key IS NOT NULL;
