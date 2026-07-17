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

func TestWriter_MakePageDir_and_GetSavedPath(t *testing.T) {
	base := t.TempDir()
	w := NewWriter(base, logging.Sugar{})

	dir, err := w.MakePageDir("PROJ", "My/Page:Title")
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(base, "PROJ", "My-Page-Title")
	if dir != want {
		t.Errorf("MakePageDir = %q, want %q", dir, want)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("page dir not created: %v", err)
	}

	got := w.GetSavedPath("PROJ", "My/Page:Title")
	if got != want {
		t.Errorf("GetSavedPath = %q, want %q", got, want)
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
