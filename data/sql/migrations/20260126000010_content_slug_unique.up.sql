-- Enforce global uniqueness for content slugs.
CREATE UNIQUE INDEX IF NOT EXISTS idx_contents_slug_unique ON contents(slug);
