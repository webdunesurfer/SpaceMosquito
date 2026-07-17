package datastore

import (
	"fmt"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/internal/store/sqlite"
	"go.uber.org/zap"
)

// Open connects to the SQLite database.
func Open(cfg *config.Config, log *zap.Logger) (store.Store, error) {
	switch cfg.Database.DriverName() {
	case "sqlite":
		return sqlite.New(cfg, log)
	case "postgres":
		return nil, fmt.Errorf("database driver %q is no longer supported; use sqlite", cfg.Database.Driver)
	default:
		return nil, fmt.Errorf("unsupported database driver %q (only sqlite is supported)", cfg.Database.Driver)
	}
}
