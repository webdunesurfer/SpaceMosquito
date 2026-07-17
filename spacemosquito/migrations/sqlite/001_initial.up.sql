PRAGMA foreign_keys = ON;

CREATE TABLE spaces (
    id TEXT PRIMARY KEY,
    key VARCHAR(10) NOT NULL UNIQUE,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    last_crawled TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE pages (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    confluence_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    parent_confluence_id INTEGER,
    content TEXT,
    html_path TEXT,
    raw_html_path TEXT,
    metadata_path TEXT,
    file_dir TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(space_id, confluence_id)
);

CREATE INDEX idx_pages_space ON pages(space_id);
CREATE INDEX idx_pages_parent ON pages(parent_confluence_id);
