// Package migrations embeds SQLite schema migrations for release binaries.
package migrations

import "embed"

//go:embed sqlite/*.sql
var SQLite embed.FS
