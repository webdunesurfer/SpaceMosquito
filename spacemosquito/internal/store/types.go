package store

import (
	"time"

	"github.com/google/uuid"
)

type Space struct {
	ID          uuid.UUID  `db:"id"`
	Key         string     `db:"key"`
	Name        string     `db:"name"`
	URL         string     `db:"url"`
	LastCrawled *time.Time `db:"last_crawled"`
	CreatedAt   time.Time  `db:"created_at"`
}

type Page struct {
	ID                 uuid.UUID `db:"id"`
	SpaceID            uuid.UUID `db:"space_id"`
	ConfluenceID       int       `db:"confluence_id"`
	Version            int       `db:"version"`
	Title              string    `db:"title"`
	ParentConfluenceID *int      `db:"parent_confluence_id"`
	Content            string    `db:"content"`
	HTMLPath           string    `db:"html_path"`
	RawHTMLPath        string    `db:"raw_html_path"`
	MetadataPath       string    `db:"metadata_path"`
	FileDir            string    `db:"file_dir"`
	CreatedAt          time.Time `db:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"`
}

// PageSummary is a lightweight page row for list APIs (no content or paths).
type PageSummary struct {
	ID                 uuid.UUID `db:"id"`
	ConfluenceID       int       `db:"confluence_id"`
	Version            int       `db:"version"`
	Title              string    `db:"title"`
	ParentConfluenceID *int      `db:"parent_confluence_id"`
	CreatedAt          time.Time `db:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"`
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
	TotalPages     int       `db:"total_pages"`
	TotalSpaces    int       `db:"total_spaces"`
	LastCrawled    time.Time `db:"last_crawled"`
	LastCrawledStr string    `db:"-"`
	ContentLang    string    `db:"content_lang"`
}
