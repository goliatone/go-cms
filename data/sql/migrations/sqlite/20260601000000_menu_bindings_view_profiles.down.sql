DROP TABLE IF EXISTS menu_location_bindings;
DROP TABLE IF EXISTS menu_view_profiles;

DROP INDEX IF EXISTS idx_menus_translation_group_id;
DROP INDEX IF EXISTS idx_menus_status;

-- No-op for menus.status/locale/translation_group_id/published_at columns on SQLite.
