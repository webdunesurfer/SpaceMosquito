package scraper

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/internal/session"
	"github.com/vkh/spacemosquito/internal/storage"
	"github.com/vkh/spacemosquito/pkg/logging"
)

// Page represents a discovered Confluence page in the tree.
type Page struct {
	ConfluenceID       int              `json:"confluence_id"`
	Title              string           `json:"title"`
	URL                string           `json:"url"`
	ParentID           *int             `json:"parent_id,omitempty"`
	Level              int              `json:"level"`
	Content            string           `json:"content,omitempty"`
	CleanHTML          string           `json:"clean_html,omitempty"`
	RawHTML            string           `json:"raw_html,omitempty"`
	Images             []storage.AssetRef
	Attachments        []storage.AssetRef
	FileDir            string           `json:"file_dir,omitempty"`
	HTMLPath           string           `json:"html_path,omitempty"`
	RawHTMLPath        string           `json:"raw_html_path,omitempty"`
	MetadataPath       string           `json:"metadata_path,omitempty"`
}

// CrawlStats tracks crawl progress.
type CrawlStats struct {
	TotalPages            int
	SkippedPages          int
	FailedPages           int
	ImagesDownloaded      int
	AttachmentsDownloaded int
}

// Scraper manages browser lifecycle and crawl operations.
type Scraper struct {
	browser *rod.Browser
	ctx     context.Context
	cancel  context.CancelFunc
	cfg     *config.Config
	db      *db.DB
	storage *storage.Writer
	assets  *storage.AssetDownloader
	log     logging.Sugar
	stats   CrawlStats
	mu      sync.Mutex
}

// New creates a new Scraper with the given config and dependencies.
func New(
	cfg *config.Config,
	database *db.DB,
	storageWriter *storage.Writer,
	assetDownloader *storage.AssetDownloader,
	log logging.Sugar,
) *Scraper {
	return &Scraper{
		cfg:     cfg,
		db:      database,
		storage: storageWriter,
		assets:  assetDownloader,
		log:     log,
	}
}

// LaunchBrowser creates a rod browser instance with Chromium headless.
func (s *Scraper) LaunchBrowser() error {
	if s.log.Enabled() {
		s.log.Info("initializing rod with Chromium")
	}

	url, err := launcher.New().
		Bin("/usr/bin/chromium").
		Headless(true).
		NoSandbox(true).
		Set("disable-gpu", "").
		Set("disable-dev-shm-usage", "").
		Set("disable-gpu-sandbox", "").
		Set("disable-setuid-sandbox", "").
		Set("disable-seccomp-filter-sandbox", "").
		Set("disable-features", "VizDisplayCompositor,TranslateUI,BlinkGenPropertyTrees").
		Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36").
		Launch()
	if err != nil {
		return fmt.Errorf("launch chromium: %w", err)
	}

	s.browser = rod.New().ControlURL(url).MustConnect()
	s.ctx, s.cancel = context.WithCancel(context.Background())

	if s.log.Enabled() {
		s.log.Info("rod browser created", "control_url", url)
	}

	return nil
}


// SetupContextWithSession injects cookies from a session into the browser.
func (s *Scraper) SetupContextWithSession(sess *session.Session) error {
	if len(sess.Cookies) == 0 {
		return nil
	}

	cookies := make([]*proto.NetworkCookie, 0, len(sess.Cookies))
	for _, c := range sess.Cookies {
		cookie := &proto.NetworkCookie{
			Name:     c.Name,
			Value:    strings.TrimSpace(c.Value),
			Domain:   strings.TrimSpace(c.Domain),
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
		}
		switch strings.TrimSpace(c.SameSite) {
		case "None":
			cookie.SameSite = proto.NetworkCookieSameSiteNone
		case "Lax":
			cookie.SameSite = proto.NetworkCookieSameSiteLax
		case "Strict":
			cookie.SameSite = proto.NetworkCookieSameSiteStrict
		}
		if c.Expires > 0 {
			cookie.Expires = proto.TimeSinceEpoch(time.Unix(int64(c.Expires), 0).Unix() * 1000)
		}
		cookies = append(cookies, cookie)
	}

	s.browser.MustSetCookies(cookies...)

	if s.log.Enabled() {
		s.log.Infow("rod context ready",
			"cookie_count", len(sess.Cookies),
			"confluence_url", sess.ConfluenceURL)
	}

	return nil
}

// CloseBrowser tears down the browser.
func (s *Scraper) CloseBrowser() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
		if s.log.Enabled() {
			s.log.Debug("rod context closed")
		}
	}
	if s.browser != nil {
		s.browser.Close()
		s.browser = nil
		if s.log.Enabled() {
			s.log.Debug("rod browser closed")
		}
	}
}

// CrawlSpace performs a full crawl of a Confluence space.
func (s *Scraper) CrawlSpace(spaceURL string, sess *session.Session) error {
	crawlStart := time.Now()

	s.log.Infow("crawl started",
		"space_url", spaceURL,
		"session_captured_at", sess.CapturedAt)

	if err := s.SetupContextWithSession(sess); err != nil {
		return fmt.Errorf("setup context: %w", err)
	}
	defer s.CloseBrowser()

	pageInfo, err := s.discoverSpace(spaceURL)
	if err != nil {
		return fmt.Errorf("discover space: %w", err)
	}

	if s.log.Enabled() {
		s.log.Infow("space discovery complete",
			"space_key", pageInfo.SpaceKey,
			"page_count", len(pageInfo.Pages),
			"duration_ms", time.Since(crawlStart).Milliseconds())
	}

	spaceID, err := s.db.CreateSpace(s.ctx, pageInfo.SpaceKey, pageInfo.SpaceName, spaceURL)
	if err != nil {
		s.log.Warnw("failed to create space record, continuing",
			"space_key", pageInfo.SpaceKey,
			"error", err)
	} else if s.log.Enabled() {
		s.log.Infow("space record created",
			"space_key", pageInfo.SpaceKey,
			"space_id", spaceID)
	}

	for i, pg := range pageInfo.Pages {
		s.log.Infow("crawling page",
			"space_key", pageInfo.SpaceKey,
			"page_index", i+1,
			"page_total", len(pageInfo.Pages),
			"page_id", pg.ConfluenceID,
			"title", pg.Title)

		if err := s.ScrapePage(pg, pageInfo.SpaceKey, pageInfo.SpaceURL); err != nil {
			s.log.Errorw("page scrape failed",
				"space_key", pageInfo.SpaceKey,
				"page_id", pg.ConfluenceID,
				"title", pg.Title,
				"error", err)
			s.mu.Lock()
			s.stats.FailedPages++
			s.mu.Unlock()
			continue
		}

		s.mu.Lock()
		s.stats.TotalPages++
		s.mu.Unlock()
	}

	duration := time.Since(crawlStart)
	if s.log.Enabled() {
		s.log.Infow("crawl complete",
			"space_key", pageInfo.SpaceKey,
			"pages_crawled", s.stats.TotalPages,
			"pages_skipped", s.stats.SkippedPages,
			"pages_failed", s.stats.FailedPages,
			"images_downloaded", s.stats.ImagesDownloaded,
			"attachments_downloaded", s.stats.AttachmentsDownloaded,
			"duration", duration.Round(time.Millisecond))
	}

	return nil
}

// ScrapePage scrapes a single page and saves it.
func (s *Scraper) ScrapePage(pg *Page, spaceKey, spaceURL string) error {
	baseURL := extractConfluenceBaseURL(spaceURL)

	page := s.browser.MustPage(pg.URL)
	page = page.MustWaitStable().MustWaitLoad()
	page.Timeout(30 * time.Second)

	html, err := page.HTML()
	if err != nil {
		return fmt.Errorf("get html: %w", err)
	}

	pg.RawHTML = html

	if s.log.Enabled() {
		s.log.Debugw("page content captured",
			"url", pg.URL,
			"html_size", len(html))
	}

	cleanHTML, images, attachments, err := s.extractContent(html, pg.Title, baseURL)
	if err != nil {
		return fmt.Errorf("extract content: %w", err)
	}

	pg.CleanHTML = cleanHTML
	pg.Images = images
	pg.Attachments = attachments

	dir, err := s.storage.MakePageDir(spaceKey, pg.Title)
	if err != nil {
		return fmt.Errorf("make page dir: %w", err)
	}
	pg.FileDir = dir

	if err := s.storage.SaveHTML(dir, cleanHTML); err != nil {
		return fmt.Errorf("save clean html: %w", err)
	}
	pg.HTMLPath = dir + "/index.html"

	if err := s.storage.SaveRawHTML(dir, html); err != nil {
		s.log.Warnw("save raw html failed",
			"page_id", pg.ConfluenceID,
			"title", pg.Title,
			"error", err)
	}
	pg.RawHTMLPath = dir + "/raw.html"

	meta := &storage.Metadata{
		Title:         pg.Title,
		ConfluenceURL: pg.URL,
		SpaceKey:      spaceKey,
		Author:        "",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Images:        images,
		Attachments:   attachments,
		SavedAt:       time.Now(),
	}
	if pg.ParentID != nil {
		meta.ParentTitle = ""
	}
	if err := s.storage.SaveMetadata(dir, meta); err != nil {
		return fmt.Errorf("save metadata: %w", err)
	}
	pg.MetadataPath = dir + "/metadata.json"

	var space *db.Space
	space, err = s.db.GetSpaceByKey(context.Background(), spaceKey)
	if err != nil {
		s.log.Infow("space not found, auto-creating", "space_key", spaceKey)
		spaceURL := spaceURL
		if spaceURL == "" {
			spaceURL = "https://example.atlassian.net/wiki/spaces/" + spaceKey
		}
		spaceID, err := s.db.CreateSpace(context.Background(), spaceKey, spaceKey, spaceURL)
		if err != nil {
			s.log.Warnw("failed to auto-create space, skipping db save",
				"space_key", spaceKey, "error", err)
			return nil
		}
		space = &db.Space{ID: spaceID, Key: spaceKey, Name: spaceKey, URL: spaceURL}
	}

	var parentID *int
	if pg.ParentID != nil && *pg.ParentID > 0 {
		parentID = pg.ParentID
	}

	dbPage := &db.Page{
		SpaceID:              space.ID,
		ConfluenceID:         pg.ConfluenceID,
		Title:                pg.Title,
		ParentConfluenceID:   parentID,
		Content:              extractTextFromHTML(cleanHTML),
		HTMLPath:             dir + "/index.html",
		RawHTMLPath:          dir + "/raw.html",
		MetadataPath:         dir + "/metadata.json",
		FileDir:              dir,
	}

	if err := s.db.UpsertPage(s.ctx, dbPage); err != nil {
		s.log.Warnw("save page to db failed",
			"page_id", pg.ConfluenceID,
			"title", pg.Title,
			"error", err)
	}

	if err := s.db.UpdateSpaceLastCrawled(s.ctx, spaceKey); err != nil {
		s.log.Warnw("failed to update space last crawled",
			"space_key", spaceKey,
			"error", err)
	}

	return nil
}
