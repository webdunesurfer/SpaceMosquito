package scraper

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestStripChrome(t *testing.T) {
	s := testScraper()
	html := loadFixture(t, "page_with_chrome.html")
	doc, err := goqueryNew(html)
	if err != nil {
		t.Fatal(err)
	}

	removed := s.stripChrome(doc)
	if removed == 0 {
		t.Error("expected chrome elements removed")
	}
	if doc.Find("script").Length() != 0 {
		t.Error("scripts should be removed")
	}
	if doc.Find("#banner-wrapper").Length() != 0 {
		t.Error("banner should be removed")
	}
	if doc.Find("main h1").Length() == 0 {
		t.Error("main content should remain")
	}
}

func TestCleanupEmptyElements(t *testing.T) {
	s := testScraper()
	doc, err := goqueryNew(`<div><div></div><span></span><p>ok</p></div>`)
	if err != nil {
		t.Fatal(err)
	}
	s.cleanupEmptyElements(doc)
	if doc.Find("div:empty").Length() != 0 {
		t.Error("empty divs should be removed")
	}
	if doc.Find("p").Length() != 1 {
		t.Error("non-empty p should remain")
	}
}

func TestRewriteInternalLinks(t *testing.T) {
	s := testScraper()
	doc, err := goqueryNew(`<a href="/wiki/spaces/PROJ/page/12345/X">link</a>`)
	if err != nil {
		t.Fatal(err)
	}
	s.rewriteInternalLinks(doc)
	link := doc.Find("a")
	href, _ := link.Attr("href")
	if href != "#" {
		t.Errorf("href = %q, want #", href)
	}
	orig, _ := link.Attr("data-original-href")
	if orig == "" {
		t.Error("expected data-original-href")
	}
}

func TestExtractContent_fixture(t *testing.T) {
	s := testScraper()
	html := loadFixture(t, "page_with_chrome.html")
	clean, images, attachments, err := s.extractContent(html, "Test", "https://example.atlassian.net")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(clean, "Hello") {
		t.Error("expected cleaned content to contain text")
	}
	if strings.Contains(clean, "banner-wrapper") {
		t.Error("chrome should be stripped from output")
	}
	if len(images) != 0 || len(attachments) != 0 {
		t.Errorf("expected no downloaded assets, got images=%d attachments=%d", len(images), len(attachments))
	}
}

// goqueryNew parses HTML without importing goquery in every test helper name clash.
func goqueryNew(html string) (*goquery.Document, error) {
	return goquery.NewDocumentFromReader(strings.NewReader(html))
}
