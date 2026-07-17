package logging

import (
	"go.uber.org/zap"
)

// Sugar is a convenience wrapper that holds a *zap.SugaredLogger.
// Embed this in your structs to get a ready-to-use sugared logger
// with a named scope for each package.
type Sugar struct {
	*zap.SugaredLogger
}

// New creates a named sugared logger for a package.
// The name is used as the logger field in all log entries.
func New(name string, logger *zap.Logger) Sugar {
	return Sugar{
		SugaredLogger: logger.Named(name).Sugar(),
	}
}

// Enabled returns true if the underlying SugaredLogger is non-nil.
func (s Sugar) Enabled() bool {
	return s.SugaredLogger != nil
}
