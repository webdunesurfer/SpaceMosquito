package datastore

import (
	"fmt"
	"path/filepath"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/internal/store/sqlite"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func migrationsDir(root, driver string) string {
	return filepath.Join(root, driver)
}

// MigrateUp applies pending schema migrations for the configured driver.
func MigrateUp(cfg *config.Config, migrationsRoot string, log logging.Sugar) error {
	switch cfg.Database.DriverName() {
	case "sqlite":
		return sqlite.MigrateUp(cfg, migrationsDir(migrationsRoot, "sqlite"), log)
	case "postgres":
		return store.MigrateUp(cfg, migrationsRoot, log)
	default:
		return fmt.Errorf("unsupported database driver %q", cfg.Database.Driver)
	}
}

// MigrateDown rolls back one migration step for the configured driver.
func MigrateDown(cfg *config.Config, migrationsRoot string, log logging.Sugar) error {
	switch cfg.Database.DriverName() {
	case "sqlite":
		return sqlite.MigrateDown(cfg, migrationsDir(migrationsRoot, "sqlite"), log)
	case "postgres":
		return store.MigrateDown(cfg, migrationsRoot, log)
	default:
		return fmt.Errorf("unsupported database driver %q", cfg.Database.Driver)
	}
}
