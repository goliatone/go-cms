ALTER TABLE menus ADD COLUMN status TEXT NOT NULL DEFAULT 'published';
ALTER TABLE menus ADD COLUMN locale TEXT;
ALTER TABLE menus ADD COLUMN translation_group_id TEXT;
ALTER TABLE menus ADD COLUMN published_at TIMESTAMP;

UPDATE menus
SET status = 'published'
WHERE status IS NULL OR trim(status) = '';

CREATE INDEX IF NOT EXISTS idx_menus_status ON menus(status);
CREATE INDEX IF NOT EXISTS idx_menus_translation_group_id ON menus(translation_group_id);

CREATE TABLE IF NOT EXISTS menu_view_profiles (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    mode TEXT NOT NULL DEFAULT 'full',
    max_top_level INTEGER,
    max_depth INTEGER,
    include_item_ids TEXT,
    exclude_item_ids TEXT,
    status TEXT NOT NULL DEFAULT 'published',
    published_at TIMESTAMP,
    environment_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001',
    created_by TEXT NOT NULL,
    updated_by TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_menu_view_profiles_env_code
    ON menu_view_profiles(environment_id, code);
CREATE INDEX IF NOT EXISTS idx_menu_view_profiles_status
    ON menu_view_profiles(status);

CREATE TABLE IF NOT EXISTS menu_location_bindings (
    id TEXT PRIMARY KEY,
    location TEXT NOT NULL,
    menu_code TEXT NOT NULL,
    view_profile_code TEXT,
    locale TEXT,
    priority INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'published',
    published_at TIMESTAMP,
    environment_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001',
    created_by TEXT NOT NULL,
    updated_by TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_menu_location_bindings_env_location
    ON menu_location_bindings(environment_id, location);
CREATE INDEX IF NOT EXISTS idx_menu_location_bindings_env_menu
    ON menu_location_bindings(environment_id, menu_code);
CREATE INDEX IF NOT EXISTS idx_menu_location_bindings_env_profile
    ON menu_location_bindings(environment_id, view_profile_code);
CREATE INDEX IF NOT EXISTS idx_menu_location_bindings_priority
    ON menu_location_bindings(environment_id, location, priority DESC);
CREATE INDEX IF NOT EXISTS idx_menu_location_bindings_locale
    ON menu_location_bindings(environment_id, location, locale);
