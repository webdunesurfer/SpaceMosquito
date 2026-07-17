package datastore

import (
	"fmt"
	"path/filepath"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/store/sqlite"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func migrationsDir(root string) string {
	return filepath.Join(root, "sqlite")
}

// MigrateUp applies pending SQLite schema migrations.
func MigrateUp(cfg *config.Config, migrationsRoot string, log logging.Sugar) error {
	if err := requireSQLite(cfg); err != nil {
		return err
	}
	return sqlite.MigrateUp(cfg, migrationsDir(migrationsRoot), log)
}

// MigrateDown rolls back one SQLite migration step.
func MigrateDown(cfg *config.Config, migrationsRoot string, log logging.Sugar) error {
	if err := requireSQLite(cfg); err != nil {
		return err
	}
	return sqlite.MigrateDown(cfg, migrationsDir(migrationsRoot), log)
}

func requireSQLite(cfg *config.Config) error {
	switch cfg.Database.DriverName() {
	case "sqlite":
		return nil
	case "postgres":
		return fmt.Errorf("database driver %q is no longer supported; use sqlite", cfg.Database.Driver)
	default:
		return fmt.Errorf("unsupported database driver %q (only sqlite is supported)", cfg.Database.Driver)
	}
}
