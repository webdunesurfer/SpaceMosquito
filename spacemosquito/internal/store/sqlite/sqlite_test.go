package sqlite_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/datastore"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logger"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func TestSQLiteStoreCRUDAndSearch(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Driver: "sqlite",
			Path:   filepath.Join(dir, "test.db"),
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
	defer db.Close()

	ctx := context.Background()
	spaceID, err := db.CreateSpace(ctx, "TST", "Test Space", "https://example.atlassian.net/wiki/spaces/TST")
	if err != nil {
		t.Fatalf("create space: %v", err)
	}

	page := &store.Page{
		SpaceID:      spaceID,
		ConfluenceID: 42,
		Version:      1,
		Title:        "Hello World",
		Content:      "mosquito search content",
		HTMLPath:     "saved/TST/42.html",
	}
	if err := db.UpsertPage(ctx, page); err != nil {
		t.Fatalf("upsert page: %v", err)
	}

	got, err := db.GetPage(ctx, "TST", 42)
	if err != nil {
		t.Fatalf("get page: %v", err)
	}
	if got.Title != "Hello World" {
		t.Fatalf("title = %q", got.Title)
	}

	results, err := db.SearchPages(ctx, "mosquito", "TST", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if results[0].ConfluenceID != 42 {
		t.Fatalf("confluence_id = %d", results[0].ConfluenceID)
	}

	count, err := db.CountPagesBySpaceID(ctx, spaceID)
	if err != nil || count != 1 {
		t.Fatalf("count = %d, err = %v", count, err)
	}

	if err := db.DeleteSpace(ctx, "TST"); err != nil {
		t.Fatalf("delete space: %v", err)
	}
}
