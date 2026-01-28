-- Remove backfilled _schema entries from content translations
UPDATE content_translations
SET content = content - '_schema'
WHERE content ? '_schema';
