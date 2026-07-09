package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func migrationsDir(root, driver string) string {
	nested := filepath.Join(root, driver)
	if _, err := os.Stat(nested); err == nil {
		return nested
	}
	if driver == "postgres" {
		return root
	}
	return nested
}

// MigrateUp applies pending Postgres schema migrations.
func MigrateUp(cfg *config.Config, migrationsRoot string, log logging.Sugar) error {
	dir := migrationsDir(migrationsRoot, "postgres")
	dsn, err := migrateDSN(cfg)
	if err != nil {
		return err
	}
	return runUp(dir, dsn, log)
}

// MigrateDown rolls back one Postgres migration step.
func MigrateDown(cfg *config.Config, migrationsRoot string, log logging.Sugar) error {
	dir := migrationsDir(migrationsRoot, "postgres")
	dsn, err := migrateDSN(cfg)
	if err != nil {
		return err
	}
	return runDown(dir, dsn, log)
}

func migrateDSN(cfg *config.Config) (string, error) {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url, nil
	}
	c := cfg.Database
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode,
	), nil
}

func runUp(migrationsPath, dsn string, log logging.Sugar) error {
	if log.Enabled() {
		log.Infow("running migrations", "path", migrationsPath, "driver", "postgres")
	}

	m, err := migrate.New("file://"+migrationsPath, dsn)
	if err != nil {
		if log.Enabled() {
			log.Errorw("create migrator failed", "error", err)
		}
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		if log.Enabled() {
			log.Errorw("migrate up failed", "error", err)
		}
		return fmt.Errorf("migrate up: %w", err)
	}

	if log.Enabled() {
		log.Info("migrations complete")
	}
	return nil
}

func runDown(migrationsPath, dsn string, log logging.Sugar) error {
	if log.Enabled() {
		log.Infow("rolling back migration", "path", migrationsPath, "driver", "postgres")
	}

	m, err := migrate.New("file://"+migrationsPath, dsn)
	if err != nil {
		if log.Enabled() {
			log.Errorw("create migrator failed", "error", err)
		}
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Steps(-1); err != nil && err != migrate.ErrNilVersion {
		if log.Enabled() {
			log.Errorw("migrate down failed", "error", err)
		}
		return fmt.Errorf("migrate down: %w", err)
	}

	if log.Enabled() {
		log.Info("migration rolled back")
	}
	return nil
}
