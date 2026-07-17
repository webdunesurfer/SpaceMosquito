package sqlite_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/datastore"
	"github.com/vkh/spacemosquito/pkg/logger"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func TestMigrateUp_embedded(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPACEMOSQUITO_MIGRATIONS_DIR", "")
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Driver: "sqlite",
			Path:   filepath.Join(dir, "embed.db"),
		},
		Storage: config.StorageConfig{BasePath: filepath.Join(dir, "saved")},
	}

	log, err := logger.NewProduction(nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := datastore.MigrateUp(cfg, "", logging.New("test", log)); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := datastore.Open(cfg, log)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.CreateSpace(context.Background(), "E", "Embed", "https://example.test/wiki/spaces/E"); err != nil {
		t.Fatalf("create space: %v", err)
	}
}

func TestMigrateUp_fileTree(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoMigrations := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")

	dir := t.TempDir()
	t.Setenv("SPACEMOSQUITO_MIGRATIONS_DIR", "")
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Driver: "sqlite",
			Path:   filepath.Join(dir, "file.db"),
		},
		Storage: config.StorageConfig{BasePath: filepath.Join(dir, "saved")},
	}

	log, err := logger.NewProduction(nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := datastore.MigrateUp(cfg, repoMigrations, logging.New("test", log)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}
