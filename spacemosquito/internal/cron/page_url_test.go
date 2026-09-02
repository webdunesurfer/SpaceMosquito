package cron

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vkh/spacemosquito/internal/store"
)

func TestResolvePageBrowseURL_FromMetadata(t *testing.T) {
	dir := t.TempDir()
	metaPath := filepath.Join(dir, "metadata.json")
	want := "https://wiki.example.com/confluence/spaces/ENG/pages/42/Architecture"
	if err := os.WriteFile(metaPath, []byte(`{"confluence_url":"`+want+`","title":"Architecture"}`), 0644); err != nil {
		t.Fatal(err)
	}

	page := store.Page{
		ConfluenceID: 42,
		Title:        "Architecture",
		MetadataPath: metaPath,
		FileDir:      dir,
	}
	got := ResolvePageBrowseURL(page, "ENG", "https://wiki.example.com/confluence/spaces/ENG")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if strings.Contains(got, "teamnetconomy") {
		t.Fatalf("resolved URL still uses hardcoded tenant: %q", got)
	}
}

func TestResolvePageBrowseURL_FromFileDirOnly(t *testing.T) {
	dir := t.TempDir()
	want := "https://acme.atlassian.net/wiki/spaces/DEMO/pages/99/Home"
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(`{"confluence_url":"`+want+`"}`), 0644); err != nil {
		t.Fatal(err)
	}

	page := store.Page{ConfluenceID: 99, FileDir: dir}
	got := ResolvePageBrowseURL(page, "DEMO", "https://other.atlassian.net/wiki/spaces/DEMO")
	if got != want {
		t.Fatalf("got %q, want metadata URL %q", got, want)
	}
}

func TestResolvePageBrowseURL_FallbackCloudSpace(t *testing.T) {
	page := store.Page{ConfluenceID: 7}
	got := ResolvePageBrowseURL(page, "PROJ", "https://acme.atlassian.net/wiki/spaces/PROJ")
	want := "https://acme.atlassian.net/wiki/spaces/PROJ/pages/7"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolvePageBrowseURL_FallbackServerDisplay(t *testing.T) {
	page := store.Page{ConfluenceID: 55}
	got := ResolvePageBrowseURL(page, "ENG", "https://wiki.company.net/confluence/display/ENG")
	want := "https://wiki.company.net/confluence/pages/viewpage.action?pageId=55"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFallbackPageBrowseURL_Empty(t *testing.T) {
	if got := FallbackPageBrowseURL("", "K", 1); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
