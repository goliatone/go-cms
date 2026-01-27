DROP INDEX IF EXISTS menus_location_idx;

-- SQLite does not support dropping columns via ALTER TABLE.
-- No-op for menus.location.
