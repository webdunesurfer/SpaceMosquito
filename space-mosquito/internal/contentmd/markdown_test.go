package contentmd

import (
	"strings"
	"testing"
)

func TestHTMLToMarkdown_adjacentParagraphs(t *testing.T) {
	html := `<p>orders are defined in acquisition definition</p><p>No Articles are given</p>`
	md, err := HTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("HTMLToMarkdown: %v", err)
	}
	if strings.Contains(md, "definitionNo") {
		t.Fatalf("merged words in %q", md)
	}
	if !strings.Contains(md, "definition") || !strings.Contains(md, "No Articles") {
		t.Fatalf("missing expected words in %q", md)
	}
}

func TestHTMLToMarkdown_headersBoldLinks(t *testing.T) {
	html := `<h2>Section</h2><p>See <a href="https://example.com/x">link</a> and <strong>bold</strong> text.</p>`
	md, err := HTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("HTMLToMarkdown: %v", err)
	}
	if !strings.Contains(md, "Section") {
		t.Fatalf("expected heading text in %q", md)
	}
	if !strings.Contains(md, "bold") {
		t.Fatalf("expected bold text in %q", md)
	}
	if !strings.Contains(md, "link") {
		t.Fatalf("expected link text in %q", md)
	}
	if strings.Contains(md, "SectionSee") {
		t.Fatalf("heading and paragraph merged in %q", md)
	}
}

func TestHTMLToMarkdown_table(t *testing.T) {
	html := `<table><tr><th>A</th><th>B</th></tr><tr><td>1</td><td>2</td></tr></table>`
	md, err := HTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("HTMLToMarkdown: %v", err)
	}
	if !strings.Contains(md, "|") {
		t.Fatalf("expected table pipes in %q", md)
	}
	if strings.Contains(md, "12") && !strings.Contains(md, "| 2") {
		t.Fatalf("table cells may be merged in %q", md)
	}
}

func TestHTMLToMarkdown_scopesWikiContent(t *testing.T) {
	html := `<div id="sidebar">noise</div><div class="wiki-content"><p>scoped body</p></div>`
	md, err := HTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("HTMLToMarkdown: %v", err)
	}
	if strings.Contains(md, "noise") {
		t.Fatalf("wiki-content scoping failed: %q", md)
	}
	if !strings.Contains(md, "scoped body") {
		t.Fatalf("expected scoped body in %q", md)
	}
}

func TestHTMLToMarkdown_truncation(t *testing.T) {
	html := "<p>" + strings.Repeat("a", 60000) + "</p>"
	md, err := HTMLToMarkdown(html)
	if err != nil {
		t.Fatalf("HTMLToMarkdown: %v", err)
	}
	if len(md) != MaxContentLen {
		t.Fatalf("len = %d, want %d", len(md), MaxContentLen)
	}
}

func TestNormalizeMarkdown_collapsesBlankLines(t *testing.T) {
	got := normalizeMarkdown("a\n\n\n\nb")
	if got != "a\n\nb" {
		t.Fatalf("got %q", got)
	}
}
