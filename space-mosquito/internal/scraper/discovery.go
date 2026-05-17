package scraper

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
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
	page := s.browser.MustPage(spaceURL)
	page.Timeout(60 * time.Second)

	// Navigate and wait for page to load
	page.MustWaitLoad()
	page.MustWaitStable()

	// Capture the full page HTML
	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("get space html: %w", err)
	}

	if s.log.Enabled() {
		s.log.Debugw("space root page captured", "html_size", len(html))
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

	spaceInfo.Pages, err = s.parseSidebar(doc, spaceInfo.SpaceKey, 0)
	if err != nil {
		return nil, fmt.Errorf("parse sidebar: %w", err)
	}

	seen := make(map[int]bool)
	var unique []*Page
	duplicates := 0
	for _, p := range spaceInfo.Pages {
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

	// Collect all sidebar page links using multiple selectors
	sidebarSelectors := []string{
		"#sidebar #page-tree li.page-tree-item a",
		"#sidebar .page-tree a",
		"[data-testid=sidebar.navigation] a",
		"#sidebar a[href*='/spaces/']",
		".aui-sidebar .aui-nav a[href*='/spaces/']",
		"[id^=sidebar] a[href*='/page']",
		".sidebar a[href*='/page']",
	}

	type linkInfo struct {
		href string
		text string
	}

	var allLinks []linkInfo

	for _, sel := range sidebarSelectors {
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			text := strings.TrimSpace(s.Text())
			if len(href) > 0 && strings.Contains(href, "/spaces/") && strings.Contains(href, "/page/") {
				allLinks = append(allLinks, linkInfo{href: href, text: text})
			}
		})
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
	// Confluence page URLs contain the page ID: /spaces/key/page/NNNNNNN
	re := regexp.MustCompile(`/page/(\d+)`)
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
