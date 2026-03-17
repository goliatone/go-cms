DROP TABLE IF EXISTS menu_location_bindings;
DROP TABLE IF EXISTS menu_view_profiles;

DROP INDEX IF EXISTS idx_menus_family_id;
DROP INDEX IF EXISTS idx_menus_status;

ALTER TABLE menus
  DROP COLUMN IF EXISTS published_at,
  DROP COLUMN IF EXISTS family_id,
  DROP COLUMN IF EXISTS locale,
  DROP COLUMN IF EXISTS status;
