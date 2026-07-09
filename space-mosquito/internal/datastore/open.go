package datastore

import (
	"fmt"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/internal/store/sqlite"
	"go.uber.org/zap"
)

// Open connects to the configured database backend.
func Open(cfg *config.Config, log *zap.Logger) (store.Store, error) {
	switch cfg.Database.DriverName() {
	case "sqlite":
		return sqlite.New(cfg, log)
	case "postgres":
		return db.New(&cfg.Database, log)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Database.Driver)
	}
}
