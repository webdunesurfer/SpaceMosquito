package sqlite

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	sqliteDriver "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/migrations"
	"github.com/vkh/spacemosquito/pkg/logging"

	_ "modernc.org/sqlite"
)

const envMigrationsDir = "SPACEMOSQUITO_MIGRATIONS_DIR"

// MigrateUp applies pending SQLite schema migrations.
func MigrateUp(cfg *config.Config, migrationsRoot string, log logging.Sugar) error {
	m, src, err := newMigrator(cfg, migrationsRoot, log)
	if err != nil {
		return err
	}
	defer m.Close()
	if log.Enabled() {
		log.Infow("running migrations", "source", src, "driver", "sqlite")
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("sqlite migrate up: %w", err)
	}
	if log.Enabled() {
		log.Info("migrations complete")
	}
	return nil
}

// MigrateDown rolls back one SQLite migration step.
func MigrateDown(cfg *config.Config, migrationsRoot string, log logging.Sugar) error {
	m, src, err := newMigrator(cfg, migrationsRoot, log)
	if err != nil {
		return err
	}
	defer m.Close()
	if log.Enabled() {
		log.Infow("rolling back migration", "source", src, "driver", "sqlite")
	}
	if err := m.Steps(-1); err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("sqlite migrate down: %w", err)
	}
	if log.Enabled() {
		log.Info("migration rolled back")
	}
	return nil
}

func newMigrator(cfg *config.Config, migrationsRoot string, log logging.Sugar) (*migrate.Migrate, string, error) {
	db, err := openForMigrate(cfg)
	if err != nil {
		return nil, "", err
	}

	driver, err := sqliteDriver.WithInstance(db, &sqliteDriver.Config{})
	if err != nil {
		db.Close()
		return nil, "", fmt.Errorf("create sqlite migrator: %w", err)
	}

	if fileDir, ok := resolveFileSource(migrationsRoot); ok {
		m, err := migrate.NewWithDatabaseInstance("file://"+fileDir, "sqlite", driver)
		if err != nil {
			db.Close()
			return nil, "", fmt.Errorf("create migrator: %w", err)
		}
		return m, "file:" + fileDir, nil
	}

	fsys, err := fs.Sub(migrations.SQLite, "sqlite")
	if err != nil {
		db.Close()
		return nil, "", fmt.Errorf("embedded migrations: %w", err)
	}
	source, err := iofs.New(fsys, ".")
	if err != nil {
		db.Close()
		return nil, "", fmt.Errorf("iofs source: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		db.Close()
		return nil, "", fmt.Errorf("create migrator: %w", err)
	}
	return m, "embed", nil
}

func resolveFileSource(migrationsRoot string) (string, bool) {
	if dir := os.Getenv(envMigrationsDir); dir != "" {
		p := filepath.Join(dir, "sqlite")
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p, true
		}
	}
	if migrationsRoot != "" {
		dir := filepath.Join(migrationsRoot, "sqlite")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, true
		}
	}
	return "", false
}

func openForMigrate(cfg *config.Config) (*sql.DB, error) {
	path, err := DBFilePath(cfg)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return sql.Open("sqlite", path)
}
