CREATE VIRTUAL TABLE pages_fts USING fts5(
    page_id UNINDEXED,
    title,
    content,
    tokenize='porter'
);

CREATE TRIGGER pages_fts_insert AFTER INSERT ON pages BEGIN
    INSERT INTO pages_fts(page_id, title, content)
    VALUES (new.id, new.title, coalesce(new.content, ''));
END;

CREATE TRIGGER pages_fts_delete AFTER DELETE ON pages BEGIN
    DELETE FROM pages_fts WHERE page_id = old.id;
END;

CREATE TRIGGER pages_fts_update AFTER UPDATE OF title, content ON pages BEGIN
    DELETE FROM pages_fts WHERE page_id = old.id;
    INSERT INTO pages_fts(page_id, title, content)
    VALUES (new.id, new.title, coalesce(new.content, ''));
END;
