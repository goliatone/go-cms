-- i18n settings: runtime translation enforcement toggles
CREATE TABLE i18n_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    translations_enabled INTEGER NOT NULL DEFAULT 1,
    require_translations INTEGER NOT NULL DEFAULT 1,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO i18n_settings (id, translations_enabled, require_translations)
VALUES (1, 1, 1);
