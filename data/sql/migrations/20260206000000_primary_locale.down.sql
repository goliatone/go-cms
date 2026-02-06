ALTER TABLE contents
    DROP COLUMN IF EXISTS primary_locale;

ALTER TABLE pages
    DROP COLUMN IF EXISTS primary_locale;
