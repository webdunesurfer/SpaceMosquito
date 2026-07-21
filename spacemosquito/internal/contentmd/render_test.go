package contentmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vkh/spacemosquito/internal/contentmd"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectBodyFormat_metadataWins(t *testing.T) {
	dir := t.TempDir()
	// raw.html looks like rendered HTML, but metadata says storage → trust metadata.
	writeFile(t, dir, "raw.html", `<p>plain</p>`)
	writeFile(t, dir, "metadata.json", `{"body_format":"storage"}`)
	if got := contentmd.DetectBodyFormat(dir); got != "storage" {
		t.Fatalf("got %q, want storage", got)
	}
}

func TestDetectBodyFormat_legacySniff(t *testing.T) {
	dir := t.TempDir()
	// No metadata → sniff raw.html.
	writeFile(t, dir, "raw.html", `<ac:structured-macro ac:name="panel"><ac:rich-text-body><p>x</p></ac:rich-text-body></ac:structured-macro>`)
	if got := contentmd.DetectBodyFormat(dir); got != "storage" {
		t.Fatalf("got %q, want storage (sniff)", got)
	}
}

func TestDetectBodyFormat_defaultsRendered(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "raw.html", `<div class="wiki-content"><p>plain rendered</p></div>`)
	if got := contentmd.DetectBodyFormat(dir); got != "rendered" {
		t.Fatalf("got %q, want rendered", got)
	}
}

func TestRenderDirMarkdown_storageUsesCSF(t *testing.T) {
	dir := t.TempDir()
	// index.html would produce macro-parameter noise via the generic converter;
	// the storage path must use raw.html + CSF and drop the params.
	writeFile(t, dir, "metadata.json", `{"body_format":"storage"}`)
	writeFile(t, dir, "index.html", `<p>ignored</p>`)
	writeFile(t, dir, "raw.html",
		`<ac:structured-macro ac:name="panel"><ac:parameter ac:name="borderStyle">solid</ac:parameter><ac:rich-text-body><p>Storage body</p></ac:rich-text-body></ac:structured-macro>`)

	md, skip, err := contentmd.RenderDirMarkdown(dir)
	if err != nil || skip != "" {
		t.Fatalf("err=%v skip=%q", err, skip)
	}
	if !strings.Contains(md, "Storage body") {
		t.Fatalf("CSF body missing: %q", md)
	}
	if strings.Contains(md, "solid") || strings.Contains(md, "ignored") {
		t.Fatalf("wrong source/params leaked: %q", md)
	}
}

func TestRenderDirMarkdown_renderedUsesIndex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "metadata.json", `{"body_format":"rendered"}`)
	writeFile(t, dir, "index.html", `<h2>Title</h2><p>Rendered body</p>`)
	writeFile(t, dir, "raw.html", `<h2>Title</h2><p>Rendered body</p>`)

	md, skip, err := contentmd.RenderDirMarkdown(dir)
	if err != nil || skip != "" {
		t.Fatalf("err=%v skip=%q", err, skip)
	}
	if !strings.Contains(md, "Rendered body") {
		t.Fatalf("rendered body missing: %q", md)
	}
}

func TestRenderDirMarkdown_legacyStorageSniffed(t *testing.T) {
	dir := t.TempDir()
	// No metadata.json (legacy). raw.html is CSF → sniff routes to CSF.
	writeFile(t, dir, "index.html", `<p>ignored generic</p>`)
	writeFile(t, dir, "raw.html",
		`<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">go</ac:parameter><ac:plain-text-body><![CDATA[x := 1]]></ac:plain-text-body></ac:structured-macro>`)

	md, _, err := contentmd.RenderDirMarkdown(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "```go") || !strings.Contains(md, "x := 1") {
		t.Fatalf("legacy storage not routed to CSF: %q", md)
	}
}
