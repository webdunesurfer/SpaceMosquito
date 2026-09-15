package bootstrap_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vkh/spacemosquito/internal/bootstrap"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/datastore"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logger"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func TestBackfillPageBodyFormats(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Database: config.DatabaseConfig{Driver: "sqlite", Path: filepath.Join(dir, "test.db")},
		Storage:  config.StorageConfig{BasePath: filepath.Join(dir, "saved")},
	}
	zl, err := logger.NewProduction(nil)
	if err != nil {
		t.Fatal(err)
	}
	log := logging.New("test", zl)
	migrationsRoot := filepath.Join("..", "..", "migrations")
	if err := datastore.MigrateUp(cfg, migrationsRoot, log); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := datastore.Open(cfg, zl)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	spaceID, err := db.CreateSpace(ctx, "ENG", "Eng", "https://example.com/wiki/spaces/ENG")
	if err != nil {
		t.Fatal(err)
	}

	csfDir := filepath.Join(dir, "saved", "ENG", "1-a")
	htmlDir := filepath.Join(dir, "saved", "ENG", "2-b")
	legacyDir := filepath.Join(dir, "saved", "ENG", "3-c")
	for _, d := range []string{csfDir, htmlDir, legacyDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(csfDir, "metadata.json"), []byte(`{"body_format":"storage"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(htmlDir, "metadata.json"), []byte(`{"body_format":"rendered"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "metadata.json"), []byte(`{"body_format":"storage"}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := db.UpsertPage(ctx, &store.Page{
		SpaceID: spaceID, ConfluenceID: 1, Title: "a", FileDir: csfDir, BodyFormat: "storage",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPage(ctx, &store.Page{
		SpaceID: spaceID, ConfluenceID: 2, Title: "b", FileDir: htmlDir, BodyFormat: "rendered",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPage(ctx, &store.Page{
		SpaceID: spaceID, ConfluenceID: 3, Title: "c", FileDir: legacyDir, BodyFormat: "",
	}); err != nil {
		t.Fatal(err)
	}

	before, err := db.CountPagesByBodyFormat(ctx, spaceID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Storage != 1 || before.Rendered != 2 {
		t.Fatalf("before = %+v", before)
	}

	n, err := bootstrap.BackfillPageBodyFormats(ctx, db, logging.Sugar{})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("updated = %d", n)
	}

	after, err := db.CountPagesByBodyFormat(ctx, spaceID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Storage != 2 || after.Rendered != 1 {
		t.Fatalf("after = %+v", after)
	}
}
