package scraper

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/vkh/spacemosquito/internal/storage"
)



// extractContent parses raw HTML, strips chrome, rewrites URLs, and downloads assets.
func (s *Scraper) extractContent(rawHTML, pageTitle, baseURL string) (string, []storage.AssetRef, []storage.AssetRef, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return "", nil, nil, fmt.Errorf("parse html: %w", err)
	}

	// Strip chrome elements
	stripped := s.stripChrome(doc)
	if s.log.Enabled() {
		s.log.Debugw("chrome elements stripped", "page_title", pageTitle, "stripped_count", stripped)
	}

	// Find and process images
	images, err := s.processImages(doc, baseURL)
	if err != nil {
		if s.log.Enabled() {
			s.log.Warnw("image processing failed", "error", err)
		}
	}

	// Find and process attachments
	attachments, err := s.processAttachments(doc, baseURL)
	if err != nil {
		if s.log.Enabled() {
			s.log.Warnw("attachment processing failed", "error", err)
		}
	}

	// Rewrite remaining relative URLs
	s.rewriteInternalLinks(doc)

	// Clean up empty elements
	s.cleanupEmptyElements(doc)

	// Serialize cleaned HTML
	cleanHTML, err := doc.Html()
	if err != nil {
		return "", nil, nil, fmt.Errorf("serialize html: %w", err)
	}

	if s.log.Enabled() {
		s.log.Infow("content extraction complete",
			"page_title", pageTitle,
			"clean_size", len(cleanHTML),
			"raw_size", len(rawHTML),
			"images", len(images),
			"attachments", len(attachments))
	}

	return cleanHTML, images, attachments, nil
}

func (s *Scraper) stripChrome(doc *goquery.Document) int {
	var removed int

	// Remove header, footer, navigation chrome
	chromeSelectors := []string{
		"#banner-wrapper, #banner, #header",
		"#footer, .footer",
		"#sidebar, .sidebar",
		".breadcrumbs, #breadcrumb",
		".toolbar, .toolbar-group",
		"#quick-edit, #edit-page-link",
		".aui-header, #aui-header2",
		".page-actions, #page-actions",
		".quick-edit",
		".space-label",
		".aui-nav",
		".aui-badge",
		"[data-testid=sidebar]",
		"[data-testid='page-header']",
		"[data-testid='page-nav']",
		".admin-menu, .quick-edit",
		".aui-page-header-wrapper",
		".aui-header .aui-navgroup",
		".aui-page-header .aui-avatar",
	}

	for _, sel := range chromeSelectors {
		doc.Find(sel).Each(func(i int, sel *goquery.Selection) {
			sel.Remove()
			removed++
		})
	}

	// Remove script and style tags (keep pre/code for code blocks)
	doc.Find("script").Remove()

	// Remove inline styles that are specific to Confluence chrome
	doc.Find("style").Remove()

	return removed
}

func (s *Scraper) processImages(doc *goquery.Document, baseURL string) ([]storage.AssetRef, error) {
	var assets []storage.AssetRef
	basePath := "assets/images"

	doc.Find("img").Each(func(i int, img *goquery.Selection) {
		src, exists := img.Attr("src")
		if !exists || src == "" {
			return
		}

		// Skip data URIs and relative URLs that aren't Confluence attachments
		if strings.HasPrefix(src, "data:") || (strings.HasPrefix(src, "/") && !strings.Contains(src, "/download/attachments/")) {
			return
		}

		// Check if this is a Confluence attachment URL
		if strings.Contains(src, "/download/attachments/") ||
			strings.Contains(src, "confluence-attachments") ||
			strings.Contains(src, "/plugins/attachments") {

			localPath, _ := s.downloadAsset(src, basePath, baseURL)

			assets = append(assets, storage.AssetRef{
				OriginalURL: src,
				LocalPath:   localPath,
			})

			// Update the src attribute
			img.SetAttr("src", localPath)

			if s.log.Enabled() {
				s.log.Debugw("image asset processed",
					"url", src,
					"local", localPath)
			}
		}
	})

	return assets, nil
}

func (s *Scraper) processAttachments(doc *goquery.Document, baseURL string) ([]storage.AssetRef, error) {
	var assets []storage.AssetRef
	basePath := "assets/attachments"

	doc.Find("a[href*='/download/attachments/']").Each(func(i int, link *goquery.Selection) {
		href, exists := link.Attr("href")
		if !exists || href == "" {
			return
		}

		localPath, _ := s.downloadAsset(href, basePath, baseURL)

		assets = append(assets, storage.AssetRef{
			OriginalURL: href,
			LocalPath:   localPath,
		})

		link.SetAttr("href", localPath)

		if s.log.Enabled() {
			s.log.Debugw("attachment processed",
				"url", href,
				"local", localPath)
		}
	})

	return assets, nil
}

func (s *Scraper) downloadAsset(rawURL, basePath, baseURL string) (string, error) {
	// Asset downloading is temporarily disabled (HTTP 202 from Atlassian CDN)
	// TODO: implement asset download via go-rod browser context for authenticated access
	// For now, keep the original URL so images/attachments are accessible in a browser session
	return rawURL, nil
}

func (s *Scraper) rewriteInternalLinks(doc *goquery.Document) {
	doc.Find("a[href]").Each(func(i int, link *goquery.Selection) {
		href, exists := link.Attr("href")
		if !exists || !strings.HasPrefix(href, "/") {
			return
		}

		// Convert Confluence internal links to local file references
		if strings.Contains(href, "/spaces/") && strings.Contains(href, "/page/") {
			// Extract page ID and look up local file path
			re := regexp.MustCompile(`/page/(\d+)`)
			matches := re.FindStringSubmatch(href)
			if len(matches) >= 2 {
				link.SetAttr("href", "#")
				link.SetAttr("data-original-href", href)
				if s.log.Enabled() {
					s.log.Debugw("internal link rewritten", "original", href)
				}
			}
		}
	})
}

func (s *Scraper) cleanupEmptyElements(doc *goquery.Document) {
	// Remove empty divs and spans that result from chrome stripping
	doc.Find("div:empty, span:empty").Remove()
}

func extractConfluenceBaseURL(url string) string {
	if url == "" {
		return ""
	}

	url = strings.TrimRight(url, "/")

	if strings.Contains(url, "atlassian.net/wiki/") {
		parts := strings.Split(url, "/wiki/")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	if strings.Contains(url, "/wiki/") {
		parts := strings.Split(url, "/wiki/")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	if strings.Contains(url, "/confluence/") {
		parts := strings.Split(url, "/confluence/")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	return url
}

// extractTextFromHTML extracts plain text from cleaned HTML for embedding.
func extractTextFromHTML(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}

	// Get text content
	text := doc.Text()

	// Clean up whitespace
	text = strings.Join(strings.Fields(text), " ")
	text = strings.TrimSpace(text)

	// Truncate to reasonable length for embedding
	if len(text) > 50000 {
		text = text[:50000]
	}

	return text
}
