package scraper

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
)

// SpaceInfo holds discovered metadata about a Confluence space.
type SpaceInfo struct {
	SpaceKey  string
	SpaceName string
	SpaceURL  string
	Pages     []*Page
}

// discoverSpace navigates to the space root and discovers all pages via sidebar parsing.
func (s *Scraper) discoverSpace(spaceURL string) (*SpaceInfo, error) {
	if s.log.Enabled() {
		s.log.Infow("navigating to space", "url", spaceURL)
	}

	page := s.browser.MustPage(spaceURL)
	page.Timeout(90 * time.Second)

	// Wait for the page to load with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		page.MustWaitLoad()
		close(done)
	}()

	select {
	case <-done:
		if s.log.Enabled() {
			s.log.Info("page loaded successfully")
		}
	case <-ctx.Done():
		if s.log.Enabled() {
			s.log.Errorw("page load timeout, taking screenshot for debug", "timeout", 60*time.Second)
		}
		// Take a screenshot for debugging
		page.MustScreenshot("/tmp/confluence_timeout.png")
		return nil, fmt.Errorf("page load timeout after 60s")
	}

	// Wait for dynamic content to render (Confluence is a SPA)
	// Use a timeout to prevent hanging
	stableCtx, stableCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stableCancel()

	stableDone := make(chan struct{})
	go func() {
		page.MustWaitStable()
		close(stableDone)
	}()

	select {
	case <-stableDone:
		if s.log.Enabled() {
			s.log.Info("page is stable")
		}
	case <-stableCtx.Done():
		if s.log.Enabled() {
			s.log.Warn("page stable timeout, continuing anyway")
		}
	}

	// Wait for sidebar to fully render
	if s.log.Enabled() {
		s.log.Info("waiting for sidebar rendering")
	}
	time.Sleep(10 * time.Second)

	// Print current URL to verify we're on the right page
	currentURL := page.MustInfo().URL
	if s.log.Enabled() {
		s.log.Infow("current page URL", "url", currentURL)
	}

	// Capture the full page HTML
	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("get space html: %w", err)
	}

	if s.log.Enabled() {
		s.log.Debugw("space root page captured", "html_size", len(html), "url", currentURL)
	}

	// Save HTML for debugging
	if err := os.WriteFile("/tmp/confluence_debug.html", []byte(html), 0644); err == nil {
		if s.log.Enabled() {
			s.log.Debugw("saved debug html", "path", "/tmp/confluence_debug.html", "size", len(html))
		}
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse space html: %w", err)
	}

	spaceInfo := &SpaceInfo{
		SpaceURL: spaceURL,
	}

	spaceInfo.SpaceKey = s.extractSpaceKey(doc, spaceURL)
	spaceInfo.SpaceName = s.extractSpaceName(doc)

	if s.log.Enabled() {
		s.log.Infow("space info extracted",
			"space_key", spaceInfo.SpaceKey,
			"space_name", spaceInfo.SpaceName)
	}

	// Try rod's Element API first for dynamic content
	pages := s.findPagesByRod(page, spaceURL)

	// Fall back to goquery if rod didn't find any
	if len(pages) == 0 {
		pages, err = s.parseSidebar(doc, spaceInfo.SpaceKey, 0)
		if err != nil {
			return nil, fmt.Errorf("parse sidebar: %w", err)
		}
	}

	seen := make(map[int]bool)
	var unique []*Page
	duplicates := 0
	for _, p := range pages {
		if seen[p.ConfluenceID] {
			duplicates++
			continue
		}
		seen[p.ConfluenceID] = true
		unique = append(unique, p)
	}
	spaceInfo.Pages = unique

	if s.log.Enabled() {
		s.log.Infow("page discovery complete",
			"space_key", spaceInfo.SpaceKey,
			"total_found", len(unique)+duplicates,
			"duplicates_skipped", duplicates,
			"unique_pages", len(unique))
	}

	return spaceInfo, nil
}

// findPagesByRod uses rod's Element API to find page links in the rendered DOM.
func (s *Scraper) findPagesByRod(page *rod.Page, spaceURL string) []*Page {
	// Normalize space URL to ensure trailing slash for URL joining
	if !strings.HasSuffix(spaceURL, "/") {
		spaceURL += "/"
	}
	var pages []*Page

	// Try to find all links with /pages/ in the href (Confluence uses /pages/ not /page/)
	if s.log.Enabled() {
		s.log.Debugw("searching for page links with /pages/")
	}

	elements, err := page.Elements("a[href*='/pages/']")
	if err != nil {
		if s.log.Enabled() {
			s.log.Debugw("rod Elements failed", "error", err)
		}
		return nil
	}

	if s.log.Enabled() {
		s.log.Debugw("found raw elements", "count", len(elements))
	}

	for _, el := range elements {
		href := el.MustAttribute("href")
		if href == nil || *href == "" || !strings.Contains(*href, "/pages/") {
			continue
		}

		text := el.MustText()
		text = strings.TrimSpace(text)

		confluenceID := s.parseConfluenceID(*href)
		if confluenceID == 0 {
			continue
		}

		title := text
		if title == "" {
			title = fmt.Sprintf("page-%d", confluenceID)
		}

		pages = append(pages, &Page{
			ConfluenceID: confluenceID,
			Title:        title,
			URL:          s.resolveURL(*href, spaceURL),
			Level:        0,
		})
	}

	if s.log.Enabled() && len(pages) > 0 {
		s.log.Debugw("found pages via rod", "count", len(pages))
	}

	return pages
}

func (s *Scraper) extractSpaceKey(doc *goquery.Document, spaceURL string) string {
	// Try extracting from space key element
	doc.Find("#space-heading, .aui-navgroup-label, [data-testid="+
		"sidebar.navigation.space-name]").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if len(text) > 0 {
			// Space key is usually the first part of the display name or a separate element
		}
	})

	// Try extracting from URL pattern: /spaces/<KEY>/...
	re := regexp.MustCompile(`/spaces/([A-Z0-9]+(?:-[A-Z0-9]+)*)/?`)
	matches := re.FindStringSubmatch(spaceURL)
	if len(matches) >= 2 {
		return matches[1]
	}

	// Fallback: try data-space-key attribute
	doc.Find("[data-space-key]").Each(func(i int, s *goquery.Selection) {
		if key, exists := s.Attr("data-space-key"); exists && len(key) > 0 {
			// Use first found
		}
	})

	return "unknown"
}

func (s *Scraper) extractSpaceName(doc *goquery.Document) string {
	// Try various selectors for space name
	selectors := []string{
		"#space-heading .aui-navgroup-label",
		".toolbar-group .active-page a",
		"[data-testid=sidebar.navigation.space-name]",
		".aui-layout .aui-nav .active a",
	}

	for _, sel := range selectors {
		text := doc.Find(sel).Text()
		text = strings.TrimSpace(text)
		if len(text) > 0 {
			return text
		}
	}

	// Try the page title as space name fallback
	title := doc.Find("#title-text, #page-title, h1:first-of-type").Text()
	title = strings.TrimSpace(title)
	return title
}

func (s *Scraper) parseSidebar(doc *goquery.Document, spaceKey string, level int) ([]*Page, error) {
	var pages []*Page

	type linkInfo struct {
		href string
		text string
	}

	var allLinks []linkInfo

	// Collect all sidebar page links using multiple selectors
	sidebarSelectors := []string{
		"[data-testid='page-tree-item'] a[href*='/pages/']",
		"[data-testid*='sidebar'] a[href*='/pages/']",
		"[data-testid*='sidebar'] [data-testid*='tree'] a[href*='/pages/']",
		"[data-testid*='navigation'] a[href*='/pages/']",
		"[data-testid*='page-tree'] a[href*='/pages/']",
		"nav a[href*='/pages/']",
		"a[href*='/spaces/'][href*='/pages/']",
	}

	for _, sel := range sidebarSelectors {
		count := 0
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			text := strings.TrimSpace(s.Text())
			if len(href) > 0 && strings.Contains(href, "/pages/") {
				allLinks = append(allLinks, linkInfo{href: href, text: text})
				count++
			}
		})
		if s.log.Enabled() && count > 0 {
			s.log.Debugw("found links with selector", "selector", sel, "count", count)
		}
	}

	// Deduplicate links by href
	seen := make(map[string]bool)
	for _, link := range allLinks {
		if seen[link.href] {
			continue
		}
		seen[link.href] = true

		// Parse confluence page ID from URL
		confluenceID := s.parseConfluenceID(link.href)
		title := link.text
		if title == "" {
			title = fmt.Sprintf("page-%d", confluenceID)
		}

		pages = append(pages, &Page{
			ConfluenceID: confluenceID,
			Title:        title,
			URL:          s.resolveURL(link.href, spaceKey),
			Level:        level,
		})
	}

	// Try to determine parent-child relationships from sidebar structure
	s.assignParentIDs(doc)

	return pages, nil
}

func (s *Scraper) parseConfluenceID(href string) int {
	// Confluence page URLs contain the page ID: /spaces/key/pages/NNNNNNN
	re := regexp.MustCompile(`/pages/(\d+)`)
	matches := re.FindStringSubmatch(href)
	if len(matches) >= 2 {
		var id int
		fmt.Sscanf(matches[1], "%d", &id)
		return id
	}
	return 0
}

func (s *Scraper) resolveURL(href, baseSpaceURL string) string {
	// Make sure URL is absolute
	if strings.HasPrefix(href, "http") {
		return href
	}

	// Build absolute URL from base space URL
	base := strings.TrimRight(baseSpaceURL, "/")
	return base + href
}

func (s *Scraper) assignParentIDs(doc *goquery.Document) {
	// Confluence sidebar typically uses nested <ul> elements for hierarchy
	// We attempt to infer parent-child from the DOM structure
	// This is a best-effort approach since Confluence uses dynamic rendering
}
