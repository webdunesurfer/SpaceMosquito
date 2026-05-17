package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Space struct {
	ID          uuid.UUID   `db:"id"`
	Key         string      `db:"key"`
	Name        string      `db:"name"`
	URL         string      `db:"url"`
	LastCrawled *time.Time  `db:"last_crawled"`
	CreatedAt   time.Time   `db:"created_at"`
}

type Page struct {
	ID                uuid.UUID     `db:"id"`
	SpaceID           uuid.UUID     `db:"space_id"`
	ConfluenceID      int           `db:"confluence_id"`
	Title             string        `db:"title"`
	ParentConfluenceID *int         `db:"parent_confluence_id"`
	Content           string        `db:"content"`
	HTMLPath          string        `db:"html_path"`
	RawHTMLPath       string        `db:"raw_html_path"`
	MetadataPath      string        `db:"metadata_path"`
	FileDir           string        `db:"file_dir"`
	CreatedAt         time.Time     `db:"created_at"`
	UpdatedAt         time.Time     `db:"updated_at"`
}

type PageEmbedding struct {
	ID        uuid.UUID   `db:"id"`
	PageID    uuid.UUID   `db:"page_id"`
	Embedding []float32   `db:"embedding"`
	CreatedAt time.Time   `db:"created_at"`
}

type SearchResult struct {
	PageID        uuid.UUID `db:"page_id"`
	SpaceKey      string    `db:"space_key"`
	Title         string    `db:"title"`
	Excerpt       string    `db:"excerpt"`
	Similarity    float64   `db:"similarity"`
	FilePath      string    `db:"file_path"`
}

func (d *DB) CreateSpace(ctx context.Context, key, name, url string) (uuid.UUID, error) {
	var id uuid.UUID
	err := d.pool.QueryRow(ctx,
		"INSERT INTO spaces (key, name, url) VALUES ($1, $2, $3) ON CONFLICT (key) DO UPDATE SET name=EXCLUDED.name, url=EXCLUDED.url RETURNING id",
		key, name, url,
	).Scan(&id)
	return id, err
}

func (d *DB) GetSpaceByKey(ctx context.Context, key string) (*Space, error) {
	var s Space
	err := d.pool.QueryRow(ctx,
		"SELECT id, key, name, url, last_crawled, created_at FROM spaces WHERE key = $1", key,
	).Scan(&s.ID, &s.Key, &s.Name, &s.URL, &s.LastCrawled, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d *DB) ListSpaces(ctx context.Context) ([]Space, error) {
	rows, err := d.pool.Query(ctx, "SELECT id, key, name, url, last_crawled, created_at FROM spaces ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spaces []Space
	for rows.Next() {
		var s Space
		if err := rows.Scan(&s.ID, &s.Key, &s.Name, &s.URL, &s.LastCrawled, &s.CreatedAt); err != nil {
			return nil, err
		}
		spaces = append(spaces, s)
	}
	return spaces, nil
}

func (d *DB) UpsertPage(ctx context.Context, page *Page) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO pages (space_id, confluence_id, title, parent_confluence_id, content, html_path, raw_html_path, metadata_path, file_dir, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		 ON CONFLICT (space_id, confluence_id) DO UPDATE SET
		   title=EXCLUDED.title,
		   parent_confluence_id=EXCLUDED.parent_confluence_id,
		   content=EXCLUDED.content,
		   html_path=EXCLUDED.html_path,
		   raw_html_path=EXCLUDED.raw_html_path,
		   metadata_path=EXCLUDED.metadata_path,
		   file_dir=EXCLUDED.file_dir,
		   updated_at=NOW()`,
		page.SpaceID, page.ConfluenceID, page.Title, page.ParentConfluenceID,
		page.Content, page.HTMLPath, page.RawHTMLPath, page.MetadataPath, page.FileDir,
	)
	return err
}

func (d *DB) GetPage(ctx context.Context, spaceKey string, pageID int) (*Page, error) {
	var p Page
	err := d.pool.QueryRow(ctx,
		`SELECT p.id, p.space_id, p.confluence_id, p.title, p.parent_confluence_id,
		        p.content, p.html_path, p.raw_html_path, p.metadata_path, p.file_dir,
		        p.created_at, p.updated_at
		 FROM pages p
		 JOIN spaces s ON s.id = p.space_id
		 WHERE s.key = $1 AND p.confluence_id = $2`,
		spaceKey, pageID,
	).Scan(&p.ID, &p.SpaceID, &p.ConfluenceID, &p.Title, &p.ParentConfluenceID,
		&p.Content, &p.HTMLPath, &p.RawHTMLPath, &p.MetadataPath, &p.FileDir,
		&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (d *DB) ListPages(ctx context.Context, spaceKey string, limit int) ([]Page, error) {
	if limit == 0 {
		limit = 100
	}
	rows, err := d.pool.Query(ctx,
		`SELECT p.id, p.space_id, p.confluence_id, p.title, p.parent_confluence_id,
		        p.content, p.html_path, p.raw_html_path, p.metadata_path, p.file_dir,
		        p.created_at, p.updated_at
		 FROM pages p
		 JOIN spaces s ON s.id = p.space_id
		 WHERE s.key = $1
		 ORDER BY p.confluence_id
		 LIMIT $2`,
		spaceKey, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		var p Page
		if err := rows.Scan(&p.ID, &p.SpaceID, &p.ConfluenceID, &p.Title, &p.ParentConfluenceID,
			&p.Content, &p.HTMLPath, &p.RawHTMLPath, &p.MetadataPath, &p.FileDir,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, nil
}

func (d *DB) CreateEmbedding(ctx context.Context, pageID uuid.UUID, embedding []float32) error {
	_, err := d.pool.Exec(ctx,
		"INSERT INTO page_embeddings (page_id, embedding) VALUES ($1, $2)",
		pageID, embedding,
	)
	return err
}

func (d *DB) UpsertEmbedding(ctx context.Context, pageID uuid.UUID, embedding []float32) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO page_embeddings (page_id, embedding) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		pageID, embedding,
	)
	if err != nil {
		return err
	}
	_, err = d.pool.Exec(ctx,
		`UPDATE page_embeddings SET embedding = $2 WHERE page_id = $1
		 AND EXISTS (SELECT 1 FROM page_embeddings WHERE page_id = $1)`,
		pageID, embedding,
	)
	return err
}

func (d *DB) SearchEmbeddings(ctx context.Context, queryEmbedding []float32, spaceKey string, limit int) ([]SearchResult, error) {
	if limit == 0 {
		limit = 10
	}

	query := `
		SELECT p.id AS page_id, s.key AS space_key, p.title,
		       p.content,
		       (pe.embedding <-> $1::vector) AS similarity,
		       p.html_path AS file_path
		FROM page_embeddings pe
		JOIN pages p ON p.id = pe.page_id
		JOIN spaces s ON s.id = p.space_id
	`
	args := []interface{}{queryEmbedding}
	argIdx := 2

	if spaceKey != "" {
		query += fmt.Sprintf(" WHERE s.key = $%d", argIdx)
		args = append(args, spaceKey)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY pe.embedding <-> $1::vector LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := d.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.PageID, &r.SpaceKey, &r.Title, &r.Excerpt, &r.Similarity, &r.FilePath); err != nil {
			return nil, err
		}
		if len(r.Excerpt) > 200 {
			r.Excerpt = r.Excerpt[:200] + "..."
		}
		results = append(results, r)
	}
	return results, nil
}
