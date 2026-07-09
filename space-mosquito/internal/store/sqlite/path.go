package sqlite

import (
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/paths"
)

// DBFilePath resolves the SQLite database file path from config.
func DBFilePath(cfg *config.Config) (string, error) {
	return paths.ResolveDB(cfg)
}
