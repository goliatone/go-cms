ALTER TABLE menus
  ADD COLUMN location text;

CREATE INDEX IF NOT EXISTS menus_location_idx ON menus (location);
