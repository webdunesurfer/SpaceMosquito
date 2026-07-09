-- +migrate Up
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE spaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(10) NOT NULL UNIQUE,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    last_crawled TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE pages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    confluence_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    parent_confluence_id INTEGER,
    content TEXT,
    html_path TEXT,
    raw_html_path TEXT,
    metadata_path TEXT,
    file_dir TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(space_id, confluence_id)
);

CREATE TABLE page_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    page_id UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    embedding vector(768) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_pages_space ON pages(space_id);
CREATE INDEX idx_pages_parent ON pages(parent_confluence_id);
CREATE INDEX idx_pages_title ON pages USING gin(to_tsvector('simple', title));
CREATE INDEX idx_embeddings_vector ON page_embeddings USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
