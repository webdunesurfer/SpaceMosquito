package sqlite_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/datastore"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logger"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func TestGetPageByConfluenceID(t *testing.T) {
	db, ctx := openTestDB(t)
	defer db.Close()

	spaceID, err := db.CreateSpace(ctx, "TST", "Test", "https://example/spaces/TST")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPage(ctx, &store.Page{
		SpaceID:      spaceID,
		ConfluenceID: 42,
		Title:        "Hello",
		Content:      "body",
	}); err != nil {
		t.Fatal(err)
	}

	page, key, err := db.GetPageByConfluenceID(ctx, 42, "")
	if err != nil {
		t.Fatalf("without space_key: %v", err)
	}
	if key != "TST" || page.Title != "Hello" {
		t.Fatalf("got key=%q title=%q", key, page.Title)
	}

	page, key, err = db.GetPageByConfluenceID(ctx, 42, "TST")
	if err != nil || key != "TST" {
		t.Fatalf("with space_key: page=%+v key=%q err=%v", page, key, err)
	}

	_, _, err = db.GetPageByConfluenceID(ctx, 42, "NOPE")
	if !errors.Is(err, store.ErrPageNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestGetPageByConfluenceID_ambiguous(t *testing.T) {
	db, ctx := openTestDB(t)
	defer db.Close()

	id1, err := db.CreateSpace(ctx, "AAA", "A", "https://example/spaces/AAA")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := db.CreateSpace(ctx, "BBB", "B", "https://example/spaces/BBB")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPage(ctx, &store.Page{
		SpaceID: id1, ConfluenceID: 99, Title: "In AAA", Content: "a",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPage(ctx, &store.Page{
		SpaceID: id2, ConfluenceID: 99, Title: "In BBB", Content: "b",
	}); err != nil {
		t.Fatal(err)
	}

	_, _, err = db.GetPageByConfluenceID(ctx, 99, "")
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	var ambiguous *store.AmbiguousPageError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("want AmbiguousPageError, got %T: %v", err, err)
	}
	if len(ambiguous.Candidates) != 2 {
		t.Fatalf("candidates = %+v", ambiguous.Candidates)
	}

	page, key, err := db.GetPageByConfluenceID(ctx, 99, "BBB")
	if err != nil || key != "BBB" || page.Title != "In BBB" {
		t.Fatalf("disambiguated: key=%q title=%q err=%v", key, page.Title, err)
	}
}

func openTestDB(t *testing.T) (store.Store, context.Context) {
	t.Helper()
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
	return db, context.Background()
}
