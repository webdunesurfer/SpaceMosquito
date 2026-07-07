package db

import (
	"context"
	"fmt"
	"strings"
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
	Version           int           `db:"version"`
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

// PageSummary is a lightweight page row for list-space APIs (no content or paths).
type PageSummary struct {
	ID                 uuid.UUID `db:"id"`
	ConfluenceID       int       `db:"confluence_id"`
	Version            int       `db:"version"`
	Title              string    `db:"title"`
	ParentConfluenceID *int      `db:"parent_confluence_id"`
	CreatedAt          time.Time `db:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"`
}

type PageEmbedding struct {
	ID        uuid.UUID   `db:"id"`
	PageID    uuid.UUID   `db:"page_id"`
	Embedding []float32   `db:"embedding"`
	CreatedAt time.Time   `db:"created_at"`
}

type SearchResult struct {
	ConfluenceID int       `db:"confluence_id"`
	SpaceKey     string    `db:"space_key"`
	Title        string    `db:"title"`
	Excerpt      string    `db:"excerpt"`
	Similarity   float64   `db:"similarity"`
	FilePath     string    `db:"file_path"`
	InternalID   uuid.UUID `db:"internal_id"`
}

type PageStats struct {
	TotalPages      int       `db:"total_pages"`
	TotalSpaces     int       `db:"total_spaces"`
	LastCrawled     time.Time `db:"last_crawled"`
	LastCrawledStr  string    `db:"-"`
	ContentLang     string    `db:"content_lang"`
}

func (d *DB) CreateSpace(ctx context.Context, key, name, url string) (uuid.UUID, error) {
	var id uuid.UUID
	err := d.pool.QueryRow(ctx,
		"INSERT INTO spaces (key, name, url) VALUES ($1, $2, $3) ON CONFLICT (key) DO UPDATE SET name=EXCLUDED.name, url=EXCLUDED.url RETURNING id",
		key, name, url,
	).Scan(&id)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("create space failed",
				"key", key,
				"name", name,
				"error", err)
		}
		return uuid.Nil, err
	}
	if d.log.Enabled() {
		d.log.Infow("space created", "id", id, "key", key, "name", name)
	}
	return id, nil
}

func (d *DB) GetSpaceByKey(ctx context.Context, key string) (*Space, error) {
	var s Space
	err := d.pool.QueryRow(ctx,
		"SELECT id, key, name, url, last_crawled, created_at FROM spaces WHERE key = $1", key,
	).Scan(&s.ID, &s.Key, &s.Name, &s.URL, &s.LastCrawled, &s.CreatedAt)
	if err != nil {
		if d.log.Enabled() {
			d.log.Debugw("get space by key: not found", "key", key, "error", err)
		}
		return nil, err
	}
	if d.log.Enabled() {
		d.log.Debugw("space retrieved", "id", s.ID, "key", s.Key, "name", s.Name)
	}
	return &s, nil
}

func (d *DB) ListSpaces(ctx context.Context) ([]Space, error) {
	rows, err := d.pool.Query(ctx, "SELECT id, key, name, url, last_crawled, created_at FROM spaces ORDER BY name")
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("list spaces failed", "error", err)
		}
		return nil, err
	}
	defer rows.Close()

	var spaces []Space
	for rows.Next() {
		var s Space
		if err := rows.Scan(&s.ID, &s.Key, &s.Name, &s.URL, &s.LastCrawled, &s.CreatedAt); err != nil {
			if d.log.Enabled() {
				d.log.Errorw("list spaces: scan error", "error", err)
			}
			return nil, err
		}
		spaces = append(spaces, s)
	}
	if d.log.Enabled() {
		d.log.Infow("spaces listed", "count", len(spaces))
	}
	return spaces, nil
}

func (d *DB) UpsertPage(ctx context.Context, page *Page) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO pages (space_id, confluence_id, version, title, parent_confluence_id, content, html_path, raw_html_path, metadata_path, file_dir, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		 ON CONFLICT (space_id, confluence_id) DO UPDATE SET
		   version=EXCLUDED.version,
		   title=EXCLUDED.title,
		   parent_confluence_id=EXCLUDED.parent_confluence_id,
		   content=EXCLUDED.content,
		   html_path=EXCLUDED.html_path,
		   raw_html_path=EXCLUDED.raw_html_path,
		   metadata_path=EXCLUDED.metadata_path,
		   file_dir=EXCLUDED.file_dir,
		   updated_at=NOW()`,
		page.SpaceID, page.ConfluenceID, page.Version, page.Title, page.ParentConfluenceID,
		page.Content, page.HTMLPath, page.RawHTMLPath, page.MetadataPath, page.FileDir,
	)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("upsert page failed",
				"space_id", page.SpaceID,
				"confluence_id", page.ConfluenceID,
				"title", page.Title,
				"error", err)
		}
		return err
	}
	if d.log.Enabled() {
		d.log.Infow("page saved",
			"space_id", page.SpaceID,
			"confluence_id", page.ConfluenceID,
			"title", page.Title,
			"html_path", page.HTMLPath)
	}
	return nil
}

func (d *DB) GetPage(ctx context.Context, spaceKey string, pageID int) (*Page, error) {
	var p Page
	err := d.pool.QueryRow(ctx,
		`SELECT p.id, p.space_id, p.confluence_id, p.version, p.title, p.parent_confluence_id,
		        p.content, p.html_path, p.raw_html_path, p.metadata_path, p.file_dir,
		        p.created_at, p.updated_at
		 FROM pages p
		 JOIN spaces s ON s.id = p.space_id
		 WHERE s.key = $1 AND p.confluence_id = $2`,
		spaceKey, pageID,
	).Scan(&p.ID, &p.SpaceID, &p.ConfluenceID, &p.Version, &p.Title, &p.ParentConfluenceID,
		&p.Content, &p.HTMLPath, &p.RawHTMLPath, &p.MetadataPath, &p.FileDir,
		&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if d.log.Enabled() {
			d.log.Debugw("get page: not found",
				"space_key", spaceKey,
				"confluence_id", pageID,
				"error", err)
		}
		return nil, err
	}
	if d.log.Enabled() {
		d.log.Debugw("page retrieved",
			"id", p.ID,
			"space_key", spaceKey,
			"title", p.Title)
	}
	return &p, nil
}

func (d *DB) ListPages(ctx context.Context, spaceKey string, limit int, afterConfluenceID *int) ([]Page, error) {
	if limit == 0 {
		limit = 100
	}
	query := `SELECT p.id, p.space_id, p.confluence_id, p.version, p.title, p.parent_confluence_id,
		        p.content, p.html_path, p.raw_html_path, p.metadata_path, p.file_dir,
		        p.created_at, p.updated_at
		 FROM pages p
		 JOIN spaces s ON s.id = p.space_id
		 WHERE s.key = $1`
	args := []interface{}{spaceKey}
	argNum := 2
	if afterConfluenceID != nil {
		query += fmt.Sprintf(" AND p.confluence_id > $%d", argNum)
		args = append(args, *afterConfluenceID)
		argNum++
	}
	query += fmt.Sprintf(" ORDER BY p.confluence_id ASC LIMIT $%d", argNum)
	args = append(args, limit)

	rows, err := d.pool.Query(ctx, query, args...)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("list pages failed", "space_key", spaceKey, "error", err)
		}
		return nil, err
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		var p Page
		if err := rows.Scan(&p.ID, &p.SpaceID, &p.ConfluenceID, &p.Version, &p.Title, &p.ParentConfluenceID,
			&p.Content, &p.HTMLPath, &p.RawHTMLPath, &p.MetadataPath, &p.FileDir,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			if d.log.Enabled() {
				d.log.Errorw("list pages: scan error", "error", err)
			}
			return nil, err
		}
		pages = append(pages, p)
	}
	if d.log.Enabled() {
		d.log.Infow("pages listed", "space_key", spaceKey, "count", len(pages))
	}
	return pages, nil
}

func (d *DB) ListPageSummaries(ctx context.Context, spaceKey string, limit int, afterConfluenceID *int) ([]PageSummary, error) {
	if limit == 0 {
		limit = 100
	}
	query := `SELECT p.id, p.confluence_id, p.version, p.title, p.parent_confluence_id,
		        p.created_at, p.updated_at
		 FROM pages p
		 JOIN spaces s ON s.id = p.space_id
		 WHERE s.key = $1`
	args := []interface{}{spaceKey}
	argNum := 2
	if afterConfluenceID != nil {
		query += fmt.Sprintf(" AND p.confluence_id > $%d", argNum)
		args = append(args, *afterConfluenceID)
		argNum++
	}
	query += fmt.Sprintf(" ORDER BY p.confluence_id ASC LIMIT $%d", argNum)
	args = append(args, limit)

	rows, err := d.pool.Query(ctx, query, args...)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("list page summaries failed", "space_key", spaceKey, "error", err)
		}
		return nil, err
	}
	defer rows.Close()

	var summaries []PageSummary
	for rows.Next() {
		var s PageSummary
		if err := rows.Scan(&s.ID, &s.ConfluenceID, &s.Version, &s.Title, &s.ParentConfluenceID,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			if d.log.Enabled() {
				d.log.Errorw("list page summaries: scan error", "error", err)
			}
			return nil, err
		}
		summaries = append(summaries, s)
	}
	if d.log.Enabled() {
		d.log.Infow("page summaries listed", "space_key", spaceKey, "count", len(summaries))
	}
	return summaries, nil
}

func (d *DB) CreateEmbedding(ctx context.Context, pageID uuid.UUID, embedding []float32) error {
	_, err := d.pool.Exec(ctx,
		"INSERT INTO page_embeddings (page_id, embedding) VALUES ($1, $2)",
		pageID, embedding,
	)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("create embedding failed",
				"page_id", pageID,
				"dimension", len(embedding),
				"error", err)
		}
		return err
	}
	if d.log.Enabled() {
		d.log.Infow("embedding created",
			"page_id", pageID,
			"dimension", len(embedding))
	}
	return nil
}

func (d *DB) UpsertEmbedding(ctx context.Context, pageID uuid.UUID, embedding []float32) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO page_embeddings (page_id, embedding) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		pageID, embedding,
	)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("upsert embedding failed: insert",
				"page_id", pageID,
				"error", err)
		}
		return err
	}
	_, err = d.pool.Exec(ctx,
		`UPDATE page_embeddings SET embedding = $2 WHERE page_id = $1
		 AND EXISTS (SELECT 1 FROM page_embeddings WHERE page_id = $1)`,
		pageID, embedding,
	)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("upsert embedding failed: update",
				"page_id", pageID,
				"error", err)
		}
		return err
	}
	if d.log.Enabled() {
		d.log.Infow("embedding upserted", "page_id", pageID, "dimension", len(embedding))
	}
	return nil
}

func (d *DB) SearchEmbeddings(ctx context.Context, queryEmbedding []float32, spaceKey string, limit int) ([]SearchResult, error) {
	if limit == 0 {
		limit = 10
	}

	query := `
		SELECT p.confluence_id, s.key AS space_key, p.title,
		       p.content,
		       (pe.embedding <-> $1::vector) AS similarity,
		       p.html_path AS file_path,
		       p.id AS internal_id
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

	if d.log.Enabled() {
		d.log.Debugw("searching embeddings",
			"space_key", spaceKey,
			"limit", limit,
			"dimension", len(queryEmbedding))
	}

	rows, err := d.pool.Query(ctx, query, args...)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("search embeddings failed", "error", err)
		}
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ConfluenceID, &r.SpaceKey, &r.Title, &r.Excerpt, &r.Similarity, &r.FilePath, &r.InternalID); err != nil {
			if d.log.Enabled() {
				d.log.Errorw("search embeddings: scan error", "error", err)
			}
			return nil, err
		}
		if len(r.Excerpt) > 200 {
			r.Excerpt = r.Excerpt[:200] + "..."
		}
		results = append(results, r)
	}
	if d.log.Enabled() {
		d.log.Infow("search results", "space_key", spaceKey, "count", len(results))
	}
	return results, nil
}

func (d *DB) IndexPageContent(ctx context.Context, spaceKey string, pageID int) error {
	var pageID64 uuid.UUID
	err := d.pool.QueryRow(ctx,
		`SELECT id FROM pages WHERE space_id = (SELECT id FROM spaces WHERE key = $1) AND confluence_id = $2`,
		spaceKey, pageID,
	).Scan(&pageID64)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("index page content: page not found",
				"space_key", spaceKey,
				"confluence_id", pageID,
				"error", err)
		}
		return fmt.Errorf("page not found: %w", err)
	}

	_, err = d.pool.Exec(ctx,
		`UPDATE pages SET content_vector = to_tsvector('english', coalesce(title, '')) || to_tsvector('english', coalesce(content, ''))
		 WHERE id = $1`,
		pageID64,
	)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("index page content: update failed",
				"page_id", pageID64,
				"error", err)
		}
		return fmt.Errorf("index page content: %w", err)
	}
	if d.log.Enabled() {
		d.log.Infow("page content indexed",
			"page_id", pageID64,
			"space_key", spaceKey,
			"confluence_id", pageID)
	}
	return nil
}

func (d *DB) IndexAllPageContents(ctx context.Context) error {
	rows, err := d.pool.Query(ctx,
		`SELECT p.id, s.key, p.confluence_id FROM pages p
		 JOIN spaces s ON s.id = p.space_id
		 WHERE p.content_vector IS NULL OR p.updated_at > pg_last_xact_replay_timestamp()`,
	)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("index all page contents: query failed", "error", err)
		}
		return fmt.Errorf("index all page contents: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var pageID64 uuid.UUID
		var spaceKey string
		var confluenceID int
		if err := rows.Scan(&pageID64, &spaceKey, &confluenceID); err != nil {
			if d.log.Enabled() {
				d.log.Errorw("index all page contents: scan error", "error", err)
			}
			continue
		}
		_, err := d.pool.Exec(ctx,
			`UPDATE pages SET content_vector = to_tsvector('english', coalesce(title, '')) || to_tsvector('english', coalesce(content, ''))
			 WHERE id = $1`,
			pageID64,
		)
		if err != nil {
			if d.log.Enabled() {
				d.log.Warnw("index page content: update failed, skipping",
					"page_id", pageID64,
					"error", err)
			}
			continue
		}
		count++
	}
	if d.log.Enabled() {
		d.log.Infow("all page contents indexed", "indexed", count)
	}
	return nil
}

func (d *DB) DeleteStalePages(ctx context.Context, spaceID uuid.UUID, crawlStart time.Time) (int64, error) {
	result, err := d.pool.Exec(ctx,
		`DELETE FROM pages WHERE space_id = $1 AND updated_at < $2`,
		spaceID, crawlStart,
	)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("delete stale pages failed", "space_id", spaceID, "error", err)
		}
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (d *DB) SearchPages(ctx context.Context, query string, spaceKey string, limit int) ([]SearchResult, error) {
	if limit == 0 {
		limit = 10
	}

	query = strings.TrimSpace(query)
	if query == "" {
		if d.log.Enabled() {
			d.log.Warn("search pages: empty query")
		}
		return nil, nil
	}

	baseQuery := `
		SELECT p.confluence_id, s.key AS space_key, p.title,
		       ts_headline(
		         'english',
		         coalesce(p.title, '') || E'\n\n' || coalesce(p.content, ''),
		         plainto_tsquery('english', $1),
		         'MaxFragments=1, MaxWords=60, MinWords=20, ShortWord=3'
		       ) AS excerpt,
		       ts_rank(p.content_vector, plainto_tsquery('english', $1)) AS similarity,
		       p.html_path AS file_path,
		       p.id AS internal_id
		FROM pages p
		JOIN spaces s ON s.id = p.space_id
		WHERE p.content_vector @@ plainto_tsquery('english', $1)
	`
	args := []interface{}{query}
	argIdx := 2

	if spaceKey != "" {
		baseQuery += fmt.Sprintf(" AND s.key = $%d", argIdx)
		args = append(args, spaceKey)
		argIdx++
	}

	baseQuery += fmt.Sprintf(" ORDER BY similarity DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	if d.log.Enabled() {
		d.log.Debugw("searching pages",
			"query", query,
			"space_key", spaceKey,
			"limit", limit)
	}

	rows, err := d.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("search pages failed", "error", err)
		}
		return nil, fmt.Errorf("search pages: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ConfluenceID, &r.SpaceKey, &r.Title, &r.Excerpt, &r.Similarity, &r.FilePath, &r.InternalID); err != nil {
			if d.log.Enabled() {
				d.log.Errorw("search pages: scan error", "error", err)
			}
			return nil, fmt.Errorf("search pages: scan error: %w", err)
		}
		results = append(results, r)
	}
	if d.log.Enabled() {
		d.log.Infow("search results", "query", query, "space_key", spaceKey, "count", len(results))
	}
	return results, nil
}

func (d *DB) GetPageStats(ctx context.Context) (*PageStats, error) {
	stats := &PageStats{}
	var lastCrawled *time.Time
	err := d.pool.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM pages),
		       (SELECT COUNT(*) FROM spaces),
		       (SELECT MAX(last_crawled) FROM spaces),
		       CASE WHEN EXISTS (SELECT 1 FROM pages WHERE content_vector IS NOT NULL)
		            THEN 'ft_vector' ELSE 'none' END AS content_lang
	`).Scan(&stats.TotalPages, &stats.TotalSpaces, &lastCrawled, &stats.ContentLang)
	if err != nil {
		if d.log.Enabled() {
			d.log.Errorw("get page stats failed", "error", err)
		}
		return nil, fmt.Errorf("get page stats: %w", err)
	}
	if lastCrawled != nil {
		stats.LastCrawled = *lastCrawled
		stats.LastCrawledStr = lastCrawled.Format(time.RFC3339)
	}
	if d.log.Enabled() {
		d.log.Infow("page stats retrieved",
			"total_pages", stats.TotalPages,
			"total_spaces", stats.TotalSpaces,
			"content_indexing", stats.ContentLang)
	}
	return stats, nil
}
