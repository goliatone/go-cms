DROP INDEX IF EXISTS idx_menu_items_menu_canonical_key;

-- SQLite does not support dropping columns via ALTER TABLE.
-- No-op for menu_items.canonical_key.
