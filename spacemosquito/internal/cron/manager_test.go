package cron

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vkh/spacemosquito/pkg/logging"
)

func TestManager_missingFile(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "missing.json"), logging.Sugar{})
	if got := m.List(); len(got) != 0 {
		t.Fatalf("expected empty configs, got %d", len(got))
	}
}

func TestManager_corruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cron.json")
	if err := os.WriteFile(path, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(path, logging.Sugar{})
	if got := m.List(); len(got) != 0 {
		t.Fatalf("expected empty configs on corrupt file, got %d", len(got))
	}
}

func TestManager_Upsert_Get_Delete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cron.json")
	m := NewManager(path, logging.Sugar{})

	cfg := PerSpaceCronConfig{
		SpaceKey:          "PROJ",
		SpaceURL:          "https://example.atlassian.net/wiki/spaces/PROJ",
		FullCrawl:         true,
		FullCrawlInterval: "6h",
	}
	if err := m.Upsert(cfg); err != nil {
		t.Fatal(err)
	}

	got := m.GetOverride("PROJ")
	if got == nil || got.FullCrawlInterval != "6h" {
		t.Fatalf("GetOverride = %+v", got)
	}
	if m.GetSpaceURL("PROJ") != cfg.SpaceURL {
		t.Errorf("GetSpaceURL = %q", m.GetSpaceURL("PROJ"))
	}

	cfg.FullCrawlInterval = "12h"
	if err := m.Upsert(cfg); err != nil {
		t.Fatal(err)
	}
	if m.GetOverride("PROJ").FullCrawlInterval != "12h" {
		t.Fatal("update failed")
	}

	m2 := NewManager(path, logging.Sugar{})
	if m2.GetOverride("PROJ").FullCrawlInterval != "12h" {
		t.Fatal("persisted config not loaded")
	}

	if err := m.Delete("PROJ"); err != nil {
		t.Fatal(err)
	}
	if m.GetOverride("PROJ") != nil {
		t.Fatal("expected nil after delete")
	}
	m3 := NewManager(path, logging.Sugar{})
	if len(m3.List()) != 0 {
		t.Fatalf("expected empty after delete, got %d", len(m3.List()))
	}
}

func TestManager_List_returnsCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cron.json")
	m := NewManager(path, logging.Sugar{})
	_ = m.Upsert(PerSpaceCronConfig{SpaceKey: "A", SpaceURL: "http://a"})

	list := m.List()
	list[0].SpaceKey = "MUTATED"

	if m.GetOverride("A").SpaceKey != "A" {
		t.Fatal("List should return a copy")
	}
}

func TestSanitizeSpaceKey(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://company.atlassian.net/wiki/spaces/PROJ/overview", "PROJ"},
		{"https://example.com/spaces/MY-SPACE", "MY-SPACE"},
		{"https://example.com/unknown", "unknown"},
	}
	for _, tc := range tests {
		if got := sanitizeSpaceKey(tc.url); got != tc.want {
			t.Errorf("sanitizeSpaceKey(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}
