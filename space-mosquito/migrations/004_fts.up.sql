-- fts search: add tsvector column and GIN index for BM25-like lexical search
-- phx: 004_fts

ALTER TABLE pages ADD COLUMN IF NOT EXISTS content_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(content, '')), 'B')
) STORED;

CREATE INDEX IF NOT EXISTS idx_pages_content_vector ON pages USING gin (content_vector);
