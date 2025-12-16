-- i18n settings: runtime translation enforcement toggles
CREATE TABLE i18n_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    translations_enabled BOOLEAN NOT NULL DEFAULT true,
    require_translations BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO i18n_settings (id, translations_enabled, require_translations)
VALUES (1, true, true);

