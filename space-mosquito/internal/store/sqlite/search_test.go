package sqlite_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/datastore"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logger"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func openSearchTestDB(t *testing.T) (store.Store, context.Context, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Driver: "sqlite",
			Path:   dbPath,
		},
		Storage: config.StorageConfig{BasePath: filepath.Join(dir, "saved")},
	}
	log, err := logger.NewProduction(nil)
	if err != nil {
		t.Fatal(err)
	}
	migrationsRoot := filepath.Join("..", "..", "..", "migrations")
	if _, err := os.Stat(filepath.Join(migrationsRoot, "sqlite")); err != nil {
		migrationsRoot = filepath.Join("..", "..", "migrations")
	}
	if err := datastore.MigrateUp(cfg, migrationsRoot, logging.New("test", log)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := datastore.Open(cfg, log)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, context.Background(), dbPath
}

func TestSearchPages_multiWordTitleRanksInTopN(t *testing.T) {
	db, ctx, _ := openSearchTestDB(t)

	spaceID, err := db.CreateSpace(ctx, "TST", "Test Space", "https://example.atlassian.net/wiki/spaces/TST")
	if err != nil {
		t.Fatalf("create space: %v", err)
	}

	target := &store.Page{
		SpaceID:      spaceID,
		ConfluenceID: 1,
		Version:      1,
		Title:        "Alpha Beta Gamma",
		Content:      "short",
		HTMLPath:     "saved/TST/1/index.html",
	}
	if err := db.UpsertPage(ctx, target); err != nil {
		t.Fatalf("upsert target: %v", err)
	}

	words := []string{"Alpha", "Beta", "Gamma"}
	for i := 2; i <= 31; i++ {
		word := words[(i-2)%len(words)]
		page := &store.Page{
			SpaceID:      spaceID,
			ConfluenceID: i,
			Version:      1,
			Title:        "Decoy Page",
			Content:      word + " " + word + " " + word + " repeated many times in body only",
			HTMLPath:     filepath.Join("saved/TST/decoy", strconv.Itoa(i)),
		}
		if err := db.UpsertPage(ctx, page); err != nil {
			t.Fatalf("upsert decoy %d: %v", i, err)
		}
	}

	results, err := db.SearchPages(ctx, "Alpha Beta Gamma", "TST", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for full title query")
	}
	if results[0].ConfluenceID != 1 {
		t.Fatalf("top result confluence_id = %d, want 1 (title %q)", results[0].ConfluenceID, results[0].Title)
	}
}

func TestSearchPages_titleBoost(t *testing.T) {
	db, ctx, _ := openSearchTestDB(t)

	spaceID, err := db.CreateSpace(ctx, "TST", "Test Space", "https://example.atlassian.net/wiki/spaces/TST")
	if err != nil {
		t.Fatalf("create space: %v", err)
	}

	titlePage := &store.Page{
		SpaceID:      spaceID,
		ConfluenceID: 100,
		Version:      1,
		Title:        "UniqueTitleWord",
		Content:      "",
		HTMLPath:     "saved/TST/100/index.html",
	}
	bodyPage := &store.Page{
		SpaceID:      spaceID,
		ConfluenceID: 101,
		Version:      1,
		Title:        "Other Page",
		Content:      "UniqueTitleWord UniqueTitleWord UniqueTitleWord UniqueTitleWord UniqueTitleWord",
		HTMLPath:     "saved/TST/101/index.html",
	}
	for _, page := range []*store.Page{titlePage, bodyPage} {
		if err := db.UpsertPage(ctx, page); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}

	results, err := db.SearchPages(ctx, "UniqueTitleWord", "TST", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ConfluenceID != 100 {
		t.Fatalf("top result confluence_id = %d, want 100 (title match should beat body-only)", results[0].ConfluenceID)
	}
}

func TestSearchPages_titleFallback(t *testing.T) {
	db, ctx, dbPath := openSearchTestDB(t)

	spaceID, err := db.CreateSpace(ctx, "TST", "Test Space", "https://example.atlassian.net/wiki/spaces/TST")
	if err != nil {
		t.Fatalf("create space: %v", err)
	}

	page := &store.Page{
		SpaceID:      spaceID,
		ConfluenceID: 50,
		Version:      1,
		Title:        "Fallback Title Here",
		Content:      "indexed body",
		HTMLPath:     "saved/TST/50/index.html",
	}
	if err := db.UpsertPage(ctx, page); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := db.GetPage(ctx, "TST", 50)
	if err != nil {
		t.Fatalf("get page: %v", err)
	}

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer raw.Close()
	if _, err := raw.Exec(`DELETE FROM pages_fts WHERE page_id = ?`, got.ID.String()); err != nil {
		t.Fatalf("delete fts row: %v", err)
	}

	results, err := db.SearchPages(ctx, "Fallback Title", "TST", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("fallback results len = %d, want 1", len(results))
	}
	if results[0].ConfluenceID != 50 {
		t.Fatalf("confluence_id = %d, want 50", results[0].ConfluenceID)
	}
}

func TestSearchPages_multiWordAND_excludesSingleTermDecoys(t *testing.T) {
	db, ctx, _ := openSearchTestDB(t)

	spaceID, err := db.CreateSpace(ctx, "TST", "Test Space", "https://example.atlassian.net/wiki/spaces/TST")
	if err != nil {
		t.Fatalf("create space: %v", err)
	}

	pages := []*store.Page{
		{
			SpaceID: spaceID, ConfluenceID: 1, Version: 1,
			Title: "Only Alpha", Content: "Alpha Alpha Alpha", HTMLPath: "saved/TST/a",
		},
		{
			SpaceID: spaceID, ConfluenceID: 2, Version: 1,
			Title: "Alpha Beta Gamma", Content: "", HTMLPath: "saved/TST/b",
		},
	}
	for _, page := range pages {
		if err := db.UpsertPage(ctx, page); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}

	results, err := db.SearchPages(ctx, "Alpha Beta Gamma", "TST", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].ConfluenceID != 2 {
		t.Fatalf("AND search = %+v, want only page 2", results)
	}
}
