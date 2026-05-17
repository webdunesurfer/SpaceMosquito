-- +migrate Down
DROP INDEX IF EXISTS idx_embeddings_vector;
DROP INDEX IF EXISTS idx_pages_title;
DROP INDEX IF EXISTS idx_pages_parent;
DROP INDEX IF EXISTS idx_pages_space;

DROP TABLE IF EXISTS page_embeddings;
DROP TABLE IF EXISTS pages;
DROP TABLE IF EXISTS spaces;

DROP EXTENSION IF EXISTS vector;
