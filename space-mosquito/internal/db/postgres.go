package db

import (
	"context"
	"fmt"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/pkg/logging"
	"go.uber.org/zap"
)

type DB struct {
	pool *pgxpool.Pool
	log  logging.Sugar
}

func New(cfg *config.DatabaseConfig, log *zap.Logger) (*DB, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode,
	)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	poolCfg.MaxConns = 10
	poolCfg.MinConns = 2
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	log.Info("connected to database", zap.String("host", cfg.Host))

	return &DB{
		pool: pool,
		log:  logging.New("db", log),
	}, nil
}

func (d *DB) Pool() *pgxpool.Pool {
	return d.pool
}

func (d *DB) Log() logging.Sugar {
	return d.log
}

func (d *DB) Close() {
	d.pool.Close()
}

func MigrateUp(migrationsPath, dsn string, log logging.Sugar) error {
	if log.Enabled() {
		log.Infow("running migrations", "path", migrationsPath)
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
