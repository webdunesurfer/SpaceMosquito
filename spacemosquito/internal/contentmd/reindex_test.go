package contentmd_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/contentmd"
	"github.com/vkh/spacemosquito/internal/datastore"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logger"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func TestReindexAll_updatesContent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	cfg := &config.Config{
		Database: config.DatabaseConfig{Driver: "sqlite", Path: dbPath},
		Storage:  config.StorageConfig{BasePath: filepath.Join(dir, "saved")},
	}
	log, err := logger.NewProduction(nil)
	if err != nil {
		t.Fatal(err)
	}
	migrationsRoot := filepath.Join("..", "..", "migrations")
	if err := datastore.MigrateUp(cfg, migrationsRoot, logging.New("test", log)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := datastore.Open(cfg, log)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	spaceID, err := db.CreateSpace(ctx, "TST", "Test", "https://example/spaces/TST")
	if err != nil {
		t.Fatal(err)
	}

	pageDir := filepath.Join(dir, "saved", "TST", "Page")
	if err := os.MkdirAll(pageDir, 0755); err != nil {
		t.Fatal(err)
	}
	html := `<p>orders are defined in acquisition definition</p><p>No Articles are given</p>`
	htmlPath := filepath.Join(pageDir, "index.html")
	if err := os.WriteFile(htmlPath, []byte(html), 0644); err != nil {
		t.Fatal(err)
	}

	if err := db.UpsertPage(ctx, &store.Page{
		SpaceID:      spaceID,
		ConfluenceID: 42,
		Title:        "Test",
		Content:      "old flat definitionNo text",
		HTMLPath:     htmlPath,
		FileDir:      pageDir,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := contentmd.ReindexAll(ctx, db, cfg.Storage.BasePath)
	if err != nil {
		t.Fatalf("ReindexAll: %v", err)
	}
	if result.Updated != 1 {
		t.Fatalf("updated = %d, want 1 (skipped=%d errors=%v)", result.Updated, result.Skipped, result.Errors)
	}

	got, err := db.GetPage(ctx, "TST", 42)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Content, "definitionNo") {
		t.Fatalf("merged words remain: %q", got.Content)
	}
	if !strings.Contains(got.Content, "definition") || !strings.Contains(got.Content, "No Articles") {
		t.Fatalf("content = %q", got.Content)
	}

	mdBytes, err := os.ReadFile(filepath.Join(pageDir, contentmd.ContentFileName))
	if err != nil {
		t.Fatalf("content.md missing: %v", err)
	}
	if strings.Contains(string(mdBytes), "definitionNo") {
		t.Fatalf("content.md has merged words: %s", mdBytes)
	}
}
