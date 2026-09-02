package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vkh/spacemosquito/pkg/logging"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Normal Title", "Normal Title"},
		{"a/b:c\\d", "a-b-c-d"},
		{"  spaced  ", "spaced"},
		{"日本語タイトル", "日本語タイトル"},
		{strings.Repeat("x", 120), strings.Repeat("x", 100)},
	}
	for _, tc := range tests {
		got := sanitizeFilename(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestWriter_MakePageDir(t *testing.T) {
	base := t.TempDir()
	w := NewWriter(base, logging.Sugar{})

	dir, err := w.MakePageDir("PROJ", "My/Page:Title", 542576204)
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(base, "PROJ", "542576204-My-Page-Title")
	if dir != want {
		t.Errorf("MakePageDir = %q, want %q", dir, want)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("page dir not created: %v", err)
	}
}

func TestWriter_MakePageDir_collisionFree(t *testing.T) {
	base := t.TempDir()
	w := NewWriter(base, logging.Sugar{})

	long := strings.Repeat("Test Report ", 20) // > 100 chars after sanitize
	dir1, err := w.MakePageDir("PROJ", long, 100)
	if err != nil {
		t.Fatal(err)
	}
	dir2, err := w.MakePageDir("PROJ", long, 200)
	if err != nil {
		t.Fatal(err)
	}
	if dir1 == dir2 {
		t.Fatalf("expected distinct dirs for same truncated title, got %q", dir1)
	}

	// ID is a full prefix even when title is truncated.
	base1 := filepath.Base(dir1)
	base2 := filepath.Base(dir2)
	if !strings.HasPrefix(base1, "100-") || !strings.HasPrefix(base2, "200-") {
		t.Fatalf("expected id prefixes, got %q and %q", base1, base2)
	}
	if len(base1) <= len("100-") || len(strings.TrimPrefix(base1, "100-")) > 100 {
		t.Fatalf("title part should be truncated to <=100: %q", base1)
	}

	// Character folding: different titles that sanitize identically still
	// differ by ID.
	a, err := w.MakePageDir("PROJ", "A/B", 1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := w.MakePageDir("PROJ", "A:B", 2)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("char-fold titles with different IDs must not share a dir")
	}
	if filepath.Base(a) != "1-A-B" || filepath.Base(b) != "2-A-B" {
		t.Fatalf("got bases %q and %q", filepath.Base(a), filepath.Base(b))
	}
}

func TestWriter_ClearPageDir(t *testing.T) {
	base := t.TempDir()
	w := NewWriter(base, logging.Sugar{})
	dir := filepath.Join(base, "PROJ", "Page")
	if err := os.MkdirAll(filepath.Join(dir, "assets", "images"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "images", "a.png"), []byte("img"), 0644); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "stale.txt")
	if err := os.WriteFile(stale, []byte("gone"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := w.ClearPageDir(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("directory itself should remain: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty dir after clear, got %d entries", len(entries))
	}

	missing := filepath.Join(base, "missing")
	if err := w.ClearPageDir(missing); err != nil {
		t.Fatalf("missing dir should be no-op: %v", err)
	}
}

func TestWriter_SaveHTML_and_SaveRawHTML(t *testing.T) {
	dir := t.TempDir()
	w := NewWriter(t.TempDir(), logging.Sugar{})

	html := "<html><body>hello</body></html>"
	if err := w.SaveHTML(dir, html); err != nil {
		t.Fatal(err)
	}
	if err := w.SaveRawHTML(dir, "<raw/>"); err != nil {
		t.Fatal(err)
	}
	if err := w.SaveMarkdown(dir, "# Title\n\nbody"); err != nil {
		t.Fatal(err)
	}

	indexBytes, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(indexBytes) != html {
		t.Errorf("index.html content mismatch")
	}

	rawBytes, err := os.ReadFile(filepath.Join(dir, "raw.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(rawBytes) != "<raw/>" {
		t.Errorf("raw.html content mismatch")
	}

	mdBytes, err := os.ReadFile(filepath.Join(dir, "content.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(mdBytes) != "# Title\n\nbody" {
		t.Errorf("content.md = %q", mdBytes)
	}
}

func TestWriter_SaveMetadata_roundTrip(t *testing.T) {
	dir := t.TempDir()
	w := NewWriter(t.TempDir(), logging.Sugar{})

	meta := &Metadata{
		Title:         "Test Page",
		ConfluenceURL: "https://example.atlassian.net/wiki/spaces/PROJ/pages/1",
		SpaceKey:      "PROJ",
		CreatedAt:     time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		UpdatedAt:     time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		SavedAt:       time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := w.SaveMetadata(dir, meta); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}

	var loaded Metadata
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Title != meta.Title || loaded.SpaceKey != meta.SpaceKey {
		t.Errorf("metadata round-trip mismatch: %+v", loaded)
	}
}
