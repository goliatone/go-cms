DROP INDEX IF EXISTS menus_location_idx;

ALTER TABLE menus
  DROP COLUMN IF EXISTS location;
