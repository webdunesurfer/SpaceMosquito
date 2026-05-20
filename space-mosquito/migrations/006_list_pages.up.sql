-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_pages_space_id ON pages(space_id);

-- +migrate Down
