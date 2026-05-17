package scraper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
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
	pw      *playwright.Playwright
	browser playwright.Browser
	context playwright.BrowserContext
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

// LaunchBrowser initializes Playwright and launches a headless Firefox browser.
func (s *Scraper) LaunchBrowser() error {
	if s.log.Enabled() {
		s.log.Info("initializing playwright")
	}

	pw, err := playwright.Run()
	if err != nil {
		s.log.Errorw("failed to launch playwright", "error", err)
		return fmt.Errorf("launch playwright: %w", err)
	}
	s.pw = pw

	browser, err := s.pw.Firefox.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		s.pw.Stop()
		s.pw = nil
		s.log.Errorw("failed to launch firefox", "error", err)
		return fmt.Errorf("launch firefox: %w", err)
	}
	s.browser = browser

	if s.log.Enabled() {
		s.log.Infow("firefox launched",
			"headless", true)
	}

	return nil
}

// SetupContextWithSession creates a browser context with cookies from a session.
func (s *Scraper) SetupContextWithSession(sess *session.Session) error {
	context, err := s.browser.NewContext(playwright.BrowserNewContextOptions{
		IgnoreHttpsErrors: playwright.Bool(true),
		UserAgent:         playwright.String("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		Viewport:          &playwright.Size{Width: 1920, Height: 1080},
	})
	if err != nil {
		s.log.Errorw("failed to create browser context", "error", err)
		return fmt.Errorf("create context: %w", err)
	}
	s.context = context

	var cookies []playwright.OptionalCookie
	for _, c := range sess.Cookies {
		var expires *float64
		if c.Expires > 0 {
			e := float64(c.Expires)
			expires = &e
		}
		cookies = append(cookies, playwright.OptionalCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   ptrString(c.Domain),
			Path:     ptrString(c.Path),
			Expires:  expires,
			Secure:   ptrBool(c.Secure),
			HttpOnly: ptrBool(c.HTTPOnly),
			SameSite: (*playwright.SameSiteAttribute)(&c.SameSite),
		})
	}

	if len(cookies) > 0 {
		if err := context.AddCookies(cookies); err != nil {
			s.log.Warnw("failed to add cookies to context", "error", err)
		}
	}

	if s.log.Enabled() {
		s.log.Infow("browser context created with cookies",
			"cookie_count", len(sess.Cookies))
	}

	return nil
}

// CloseBrowser tears down the browser and playwright instance.
func (s *Scraper) CloseBrowser() {
	if s.context != nil {
		s.context.Close()
		s.context = nil
		if s.log.Enabled() {
			s.log.Debug("browser context closed")
		}
	}
	if s.browser != nil {
		s.browser.Close()
		s.browser = nil
		if s.log.Enabled() {
			s.log.Debug("browser closed")
		}
	}
	if s.pw != nil {
		s.pw.Stop()
		s.pw = nil
		if s.log.Enabled() {
			s.log.Debug("playwright closed")
		}
	}
}

// CrawlSpace performs a full crawl of a Confluence space.
func (s *Scraper) CrawlSpace(spaceURL string, sess *session.Session) error {
	crawlStart := time.Now()
	ctx := context.Background()

	s.log.Infow("crawl started",
		"space_url", spaceURL,
		"session_captured_at", sess.CapturedAt)

	if err := s.SetupContextWithSession(sess); err != nil {
		return fmt.Errorf("setup context: %w", err)
	}
	defer s.CloseBrowser()

	pageInfo, err := s.discoverSpace(spaceURL, ctx)
	if err != nil {
		return fmt.Errorf("discover space: %w", err)
	}

	if s.log.Enabled() {
		s.log.Infow("space discovery complete",
			"space_key", pageInfo.SpaceKey,
			"page_count", len(pageInfo.Pages),
			"duration_ms", time.Since(crawlStart).Milliseconds())
	}

	spaceID, err := s.db.CreateSpace(ctx, pageInfo.SpaceKey, pageInfo.SpaceName, spaceURL)
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

		if err := s.scrapePage(ctx, pg, pageInfo.SpaceKey, pageInfo.SpaceURL); err != nil {
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

func (s *Scraper) scrapePage(ctx context.Context, pg *Page, spaceKey, spaceURL string) error {
	page, err := s.context.NewPage()
	if err != nil {
		return fmt.Errorf("new page: %w", err)
	}
	defer page.Close()

	rawHTML, err := s.navigateAndWait(page, pg.URL)
	if err != nil {
		return fmt.Errorf("navigate: %w", err)
	}

	pg.RawHTML = rawHTML

	cleanHTML, images, attachments, err := s.extractContent(rawHTML, pg.Title)
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

	if err := s.storage.SaveRawHTML(dir, rawHTML); err != nil {
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

	space, err := s.db.GetSpaceByKey(ctx, spaceKey)
	if err != nil {
		s.log.Warnw("space not found for page save",
			"space_key", spaceKey,
			"error", err)
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

	if err := s.db.UpsertPage(ctx, dbPage); err != nil {
		s.log.Warnw("save page to db failed",
			"page_id", pg.ConfluenceID,
			"title", pg.Title,
			"error", err)
	}

	return nil
}

func (s *Scraper) navigateAndWait(page playwright.Page, pageURL string) (string, error) {
	resp, err := page.Goto(pageURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateCommit,
		Timeout:   playwright.Float(60000),
	})
	if err != nil {
		return "", fmt.Errorf("goto: %w", err)
	}

	if s.log.Enabled() {
		status := "unknown"
		if resp != nil {
			status = fmt.Sprintf("%d", resp.Status())
		}
		s.log.Infow("page navigated",
			"url", pageURL,
			"status", status)
	}

	waitStart := time.Now()
	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(30000),
	}); err != nil {
		if s.log.Enabled() {
			s.log.Warnw("network idle timeout", "waited_ms", time.Since(waitStart).Milliseconds())
		}
	} else if s.log.Enabled() {
		s.log.Debugw("network idle reached", "waited_ms", time.Since(waitStart).Milliseconds())
	}

	html, err := page.Content()
	if err != nil {
		return "", fmt.Errorf("get content: %w", err)
	}

	if s.log.Enabled() {
		s.log.Debugw("page content captured",
			"url", pageURL,
			"html_size", len(html))
	}

	return html, nil
}

func ptrBool(b bool) *bool {
	return &b
}

func ptrString(s string) *string {
	return &s
}
