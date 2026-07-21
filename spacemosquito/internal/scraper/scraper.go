package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/contentmd"
	"github.com/vkh/spacemosquito/internal/contentmd/csf"
	"github.com/vkh/spacemosquito/internal/session"
	"github.com/vkh/spacemosquito/internal/storage"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logging"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Page represents a discovered Confluence page in the tree.
type Page struct {
	ConfluenceID int    `json:"confluence_id"`
	Version      int    `json:"version"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	ParentID     *int   `json:"parent_id,omitempty"`
	Level        int    `json:"level"`
	Content      string `json:"content,omitempty"`
	CleanHTML    string `json:"clean_html,omitempty"`
	RawHTML      string `json:"raw_html,omitempty"`
	Images       []storage.AssetRef
	Attachments  []storage.AssetRef
	FileDir      string `json:"file_dir,omitempty"`
	HTMLPath     string `json:"html_path,omitempty"`
	RawHTMLPath  string `json:"raw_html_path,omitempty"`
	MetadataPath string `json:"metadata_path,omitempty"`
}

// CrawlStats tracks crawl progress.
type CrawlStats struct {
	TotalPages            int
	SkippedPages          int
	SkippedUnchanged      int
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
	db      store.Store
	storage *storage.Writer
	assets  *storage.AssetDownloader
	log     logging.Sugar
	stats   CrawlStats
	mu      sync.Mutex
}

// New creates a new Scraper with the given config and dependencies.
func New(
	cfg *config.Config,
	database store.Store,
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

// LaunchBrowser creates a rod browser instance with Chromium headless (lazy, idempotent).
func (s *Scraper) LaunchBrowser() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.browser != nil {
		return nil
	}

	bin, source, err := resolveChromiumBin(s.cfg, s.log)
	if err != nil {
		return err
	}
	if s.log.Enabled() {
		s.log.Infow("launching Chromium", "source", source, "bin", bin)
	}

	url, err := buildLauncher(bin).Launch()
	if err != nil {
		return fmt.Errorf("launch chromium: %w", err)
	}

	s.browser = rod.New().ControlURL(url).MustConnect()
	if s.ctx == nil || s.cancel == nil {
		s.ctx, s.cancel = context.WithCancel(context.Background())
	}

	if s.log.Enabled() {
		s.log.Info("rod browser created", "control_url", url)
	}
	return nil
}

func (s *Scraper) Browser() *rod.Browser {
	return s.browser
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
		"flavor", sess.Flavor,
		"session_captured_at", sess.CapturedAt)

	// Context is needed for DB operations
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	pageInfo, err := s.discoverSpace(spaceURL, sess)
	if err != nil {
		return fmt.Errorf("discover space: %w", err)
	}

	// Close browser if it was opened for fallback
	defer s.CloseBrowser()

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
			"version", pg.Version,
			"title", pg.Title)

		// Check if page needs scraping
		if pg.Version > 0 {
			existingPage, err := s.db.GetPage(s.ctx, pageInfo.SpaceKey, pg.ConfluenceID)
			if err == nil && existingPage.Version >= pg.Version {
				if s.log.Enabled() {
					s.log.Infow("skipping unchanged page", "page_id", pg.ConfluenceID, "version", pg.Version)
				}
				s.mu.Lock()
				s.stats.SkippedUnchanged++
				s.stats.TotalPages++ // Count as processed
				s.mu.Unlock()
				continue
			}
		}

		// Try API scraping first
		err := s.ScrapePageAPI(pg, pageInfo.SpaceKey, pageInfo.SpaceURL, sess)
		if err != nil {
			if s.log.Enabled() {
				s.log.Warnw("API page content extraction failed, FALLING BACK to browser",
					"page_id", pg.ConfluenceID, "title", pg.Title, "error", err)
			}

			// Fallback to browser
			if s.browser == nil {
				if err := s.LaunchBrowser(); err != nil {
					s.log.Errorw("failed to launch browser for fallback", "error", err)
					s.mu.Lock()
					s.stats.FailedPages++
					s.mu.Unlock()
					continue
				}
				if err := s.SetupContextWithSession(sess); err != nil {
					s.log.Errorw("failed to setup browser context", "error", err)
					s.mu.Lock()
					s.stats.FailedPages++
					s.mu.Unlock()
					continue
				}
			}

			if err := s.ScrapePage(pg, pageInfo.SpaceKey, pageInfo.SpaceURL); err != nil {
				s.log.Errorw("browser page scrape failed",
					"space_key", pageInfo.SpaceKey,
					"page_id", pg.ConfluenceID,
					"title", pg.Title,
					"error", err)
				s.mu.Lock()
				s.stats.FailedPages++
				s.mu.Unlock()
				continue
			}
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
			"pages_skipped_unchanged", s.stats.SkippedUnchanged,
			"pages_failed", s.stats.FailedPages,
			"images_downloaded", s.stats.ImagesDownloaded,
			"attachments_downloaded", s.stats.AttachmentsDownloaded,
			"duration", duration.Round(time.Millisecond))
	}

	// Stale page sweep logic (Disabled for now, implement later as a manual action)
	/*
		if s.log.Enabled() {
			s.log.Infow("sweeping stale pages deleted from Confluence", "space_key", pageInfo.SpaceKey)
		}
		deletedCount, err := s.db.DeleteStalePages(s.ctx, spaceID, crawlStart)
		if err != nil {
			s.log.Warnw("failed to sweep stale pages", "space_key", pageInfo.SpaceKey, "error", err)
		} else if deletedCount > 0 && s.log.Enabled() {
			s.log.Infow("stale pages swept", "space_key", pageInfo.SpaceKey, "deleted_count", deletedCount)
		}
	*/

	return nil
}

// ScrapePageAPI fetches page content using the REST API.
func (s *Scraper) ScrapePageAPI(pg *Page, spaceKey, spaceURL string, sess *session.Session) error {
	baseURL := extractConfluenceBaseURL(spaceURL)

	var apiURL string
	if sess.Flavor == session.FlavorCloud {
		apiURL = fmt.Sprintf("%s/wiki/rest/api/content/%d?expand=body.storage,version,ancestors", baseURL, pg.ConfluenceID)
	} else {
		apiURL = fmt.Sprintf("%s/rest/api/content/%d?expand=body.storage,version,ancestors", baseURL, pg.ConfluenceID)
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}

	for k, v := range sess.AsHeaders() {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var result struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Body  struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	storageHTML := result.Body.Storage.Value
	if storageHTML == "" {
		return fmt.Errorf("empty storage body")
	}

	pg.RawHTML = storageHTML

	if s.log.Enabled() {
		s.log.Debugw("page content captured via API",
			"url", pg.URL,
			"html_size", len(storageHTML))
	}

	// Process content (asset downloads etc still use the scraper's logic)
	cleanHTML, images, attachments, err := s.extractContent(storageHTML, pg.Title, baseURL)
	if err != nil {
		return fmt.Errorf("extract content: %w", err)
	}

	pg.CleanHTML = cleanHTML
	pg.Images = images
	pg.Attachments = attachments

	// Save to disk and DB (shared logic with browser scraping)
	return s.savePageMetadata(pg, spaceKey, spaceURL, sess.Flavor == session.FlavorCloud)
}

// savePageMetadata saves the scraped page to disk and database.
func (s *Scraper) savePageMetadata(pg *Page, spaceKey, spaceURL string, cloud bool) error {
	dir, err := s.storage.MakePageDir(spaceKey, pg.Title)
	if err != nil {
		return fmt.Errorf("make page dir: %w", err)
	}
	pg.FileDir = dir

	if err := s.storage.SaveHTML(dir, pg.CleanHTML); err != nil {
		return fmt.Errorf("save clean html: %w", err)
	}
	pg.HTMLPath = dir + "/index.html"

	contentMD, assets, err := s.renderMarkdown(pg, spaceURL, cloud)
	if err != nil {
		return fmt.Errorf("convert content markdown: %w", err)
	}
	if err := s.storage.SaveMarkdown(dir, contentMD); err != nil {
		return fmt.Errorf("save content markdown: %w", err)
	}

	// CSF conversion schedules asset downloads (images, diagrams); fetch them
	// into the page dir and record them alongside any HTML-path images.
	csfImages, csfDiagrams := s.downloadCSFAssets(dir, assets)
	pg.Images = append(pg.Images, csfImages...)

	if err := s.storage.SaveRawHTML(dir, pg.RawHTML); err != nil {
		s.log.Warnw("save raw html failed",
			"page_id", pg.ConfluenceID,
			"title", pg.Title,
			"error", err)
	}
	pg.RawHTMLPath = dir + "/raw.html"

	bodyFormat := "rendered"
	if csf.IsStorageFormat(pg.RawHTML) {
		bodyFormat = "storage"
	}
	meta := &storage.Metadata{
		Title:         pg.Title,
		ConfluenceURL: pg.URL,
		SpaceKey:      spaceKey,
		Author:        "",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Images:        pg.Images,
		Diagrams:      csfDiagrams,
		Attachments:   pg.Attachments,
		BodyFormat:    bodyFormat,
		SavedAt:       time.Now(),
	}
	if pg.ParentID != nil {
		meta.ParentTitle = ""
	}
	if err := s.storage.SaveMetadata(dir, meta); err != nil {
		return fmt.Errorf("save metadata: %w", err)
	}
	pg.MetadataPath = dir + "/metadata.json"

	var space *store.Space
	space, err = s.db.GetSpaceByKey(context.Background(), spaceKey)
	if err != nil {
		s.log.Infow("space not found, auto-creating", "space_key", spaceKey)
		sURL := spaceURL
		if sURL == "" {
			sURL = "https://example.atlassian.net/wiki/spaces/" + spaceKey
		}
		spaceID, err := s.db.CreateSpace(context.Background(), spaceKey, spaceKey, sURL)
		if err != nil {
			s.log.Warnw("failed to auto-create space, skipping db save",
				"space_key", spaceKey, "error", err)
			return nil
		}
		space = &store.Space{ID: spaceID, Key: spaceKey, Name: spaceKey, URL: sURL}
	}

	var parentID *int
	if pg.ParentID != nil && *pg.ParentID > 0 {
		parentID = pg.ParentID
	}

	dbPage := &store.Page{
		SpaceID:            space.ID,
		ConfluenceID:       pg.ConfluenceID,
		Version:            pg.Version,
		Title:              pg.Title,
		ParentConfluenceID: parentID,
		Content:            contentMD,
		HTMLPath:           dir + "/index.html",
		RawHTMLPath:        dir + "/raw.html",
		MetadataPath:       dir + "/metadata.json",
		FileDir:            dir,
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

// renderMarkdown produces the page's Markdown, routing by input format: pages
// whose raw body is Confluence Storage Format go through the rule-based CSF
// converter; browser-fallback rendered HTML uses the generic converter.
func (s *Scraper) renderMarkdown(pg *Page, spaceURL string, cloud bool) (string, []csf.AssetRequest, error) {
	if csf.IsStorageFormat(pg.RawHTML) {
		ctx := &csf.RenderContext{
			PageID:  pg.ConfluenceID,
			BaseURL: extractConfluenceBaseURL(spaceURL),
			Cloud:   cloud,
		}
		md, assets, err := csf.CSFToMarkdown(pg.RawHTML, ctx)
		if err != nil {
			return "", nil, fmt.Errorf("csf to markdown: %w", err)
		}
		return md, assets, nil
	}
	md, err := contentmd.HTMLToMarkdown(pg.CleanHTML)
	return md, nil, err
}

// downloadCSFAssets fetches the assets a CSF conversion requested into the
// page directory and returns their metadata refs, split by kind (images vs
// draw.io diagrams). Failures are logged and skipped (best-effort, like the
// HTML image path); the emitted Markdown alt text still degrades gracefully.
func (s *Scraper) downloadCSFAssets(dir string, reqs []csf.AssetRequest) (images, diagrams []storage.AssetRef) {
	for _, r := range reqs {
		if r.URL == "" || r.Filename == "" {
			continue
		}
		sub := "images"
		if r.Kind == "diagram" {
			sub = "diagrams"
		}
		rel := filepath.Join("assets", sub, r.Filename)
		if err := s.assets.DownloadAs(filepath.Join(dir, rel), r.URL); err != nil {
			s.log.Warnw("csf asset download failed", "url", r.URL, "kind", r.Kind, "error", err)
			continue
		}
		ref := storage.AssetRef{OriginalURL: r.URL, LocalPath: rel}
		if r.Kind == "diagram" {
			diagrams = append(diagrams, ref)
		} else {
			images = append(images, ref)
		}
	}
	return images, diagrams
}

// ScrapePage scrapes a single page using a headless browser (legacy fallback).
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
		s.log.Debugw("page content captured via browser",
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

	// Browser fallback yields rendered HTML (not CSF); flavor is irrelevant.
	return s.savePageMetadata(pg, spaceKey, spaceURL, false)
}
