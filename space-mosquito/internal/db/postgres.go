package db

import (
	"context"
	"fmt"
	"time"

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

	log.Info("connected to database", zap.String("host", cfg.Host), zap.String("driver", "postgres"))

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

func (d *DB) UpdateSpaceLastCrawled(ctx context.Context, spaceKey string) error {
	_, err := d.pool.Exec(ctx,
		`UPDATE spaces SET last_crawled = $1 WHERE key = $2`,
		time.Now(), spaceKey,
	)
	return err
}
