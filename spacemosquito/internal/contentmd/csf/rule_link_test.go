package csf

import (
	"strings"
	"testing"
)

func TestLink_url(t *testing.T) {
	got := md(t, `<p><ac:link><ri:url ri:value="https://example.com/p"/><ac:link-body>External</ac:link-body></ac:link></p>`)
	if !strings.Contains(got, "[External](https://example.com/p)") {
		t.Fatalf("url link not rendered: %q", got)
	}
}

func TestLink_urlWithoutLabelUsesURL(t *testing.T) {
	got := md(t, `<p><ac:link><ri:url ri:value="https://example.com/p"/></ac:link></p>`)
	if !strings.Contains(got, "[https://example.com/p](https://example.com/p)") {
		t.Fatalf("url label fallback failed: %q", got)
	}
}

func TestLink_pageRendersLabelText(t *testing.T) {
	// Page links are not resolvable to a URL in Phase 1 → label text, no broken link.
	got := md(t, `<p><ac:link><ri:page ri:content-title="Target Page" ri:space-key="DEMO"/></ac:link></p>`)
	if !strings.Contains(got, "Target Page") {
		t.Fatalf("page title missing: %q", got)
	}
	if strings.Contains(got, "](") {
		t.Fatalf("page link should be plain text, got %q", got)
	}
}

func TestLink_user(t *testing.T) {
	got := md(t, `<p><ac:link><ri:user ri:username="jdoe"/></ac:link></p>`)
	if !strings.Contains(got, "@jdoe") {
		t.Fatalf("user mention not rendered: %q", got)
	}
}

func TestLink_attachmentLabelFromFilename(t *testing.T) {
	got := md(t, `<p><ac:link><ri:attachment ri:filename="spec.pdf"/></ac:link></p>`)
	if !strings.Contains(got, "spec.pdf") {
		t.Fatalf("attachment filename missing: %q", got)
	}
}
