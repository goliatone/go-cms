UPDATE content_translations
SET metadata = NULL
WHERE trim(metadata) = 'null';

