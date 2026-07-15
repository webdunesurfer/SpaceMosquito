package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Store is the database abstraction used by API, MCP, scraper, and cron.
type Store interface {
	CreateSpace(ctx context.Context, key, name, url string) (uuid.UUID, error)
	GetSpaceByKey(ctx context.Context, key string) (*Space, error)
	ListSpaces(ctx context.Context) ([]Space, error)
	UpdateSpaceLastCrawled(ctx context.Context, spaceKey string) error
	CountPagesBySpaceID(ctx context.Context, spaceID uuid.UUID) (int, error)
	DeleteSpace(ctx context.Context, spaceKey string) error

	UpsertPage(ctx context.Context, page *Page) error
	GetPage(ctx context.Context, spaceKey string, confluenceID int) (*Page, error)
	GetPageByConfluenceID(ctx context.Context, confluenceID int, spaceKey string) (*Page, string, error)
	ListPages(ctx context.Context, spaceKey string, limit int, afterConfluenceID *int) ([]Page, error)
	ListPageSummaries(ctx context.Context, spaceKey string, limit int, afterConfluenceID *int) ([]PageSummary, error)
	DeleteStalePages(ctx context.Context, spaceID uuid.UUID, crawlStart time.Time) (int64, error)

	SearchPages(ctx context.Context, query, spaceKey string, limit int) ([]SearchResult, error)
	IndexPageContent(ctx context.Context, spaceKey string, confluenceID int) error
	IndexAllPageContents(ctx context.Context) error
	GetPageStats(ctx context.Context) (*PageStats, error)

	Close()
}
