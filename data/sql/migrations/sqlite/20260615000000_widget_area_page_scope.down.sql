PRAGMA foreign_keys = OFF;
PRAGMA legacy_alter_table = ON;

ALTER TABLE widget_area_definitions RENAME TO widget_area_definitions_old;

CREATE TABLE widget_area_definitions (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    scope TEXT NOT NULL DEFAULT 'global' CHECK (scope IN ('global', 'theme', 'template')),
    theme_id TEXT REFERENCES themes(id) ON DELETE SET NULL,
    template_id TEXT REFERENCES templates(id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO widget_area_definitions (id, code, name, description, scope, theme_id, template_id, created_at, updated_at)
SELECT id,
       code,
       name,
       description,
       CASE WHEN scope = 'page' THEN 'global' ELSE scope END,
       theme_id,
       template_id,
       created_at,
       updated_at
FROM widget_area_definitions_old;

DROP TABLE widget_area_definitions_old;

CREATE INDEX idx_widget_area_definitions_code ON widget_area_definitions(code);
CREATE INDEX idx_widget_area_definitions_scope ON widget_area_definitions(scope);
CREATE INDEX idx_widget_area_definitions_theme_id ON widget_area_definitions(theme_id);
CREATE INDEX idx_widget_area_definitions_template_id ON widget_area_definitions(template_id);

PRAGMA legacy_alter_table = OFF;
PRAGMA foreign_keys = ON;
