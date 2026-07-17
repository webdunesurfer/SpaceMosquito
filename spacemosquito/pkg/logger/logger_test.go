package logger

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Level != "info" {
		t.Errorf("Level = %q, want info", cfg.Level)
	}
	if cfg.Format != "json" {
		t.Errorf("Format = %q, want json", cfg.Format)
	}
}

func TestNewProductionWithLevel(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		t.Run(level, func(t *testing.T) {
			zl, err := NewProductionWithLevel(level)
			if err != nil {
				t.Fatal(err)
			}
			if zl == nil {
				t.Fatal("expected non-nil logger")
			}
		})
	}
}

func TestNewProduction_invalidLevelFallsBack(t *testing.T) {
	zl, err := NewProduction(&Config{Level: "not-a-level", Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	if zl == nil {
		t.Fatal("expected non-nil logger with fallback level")
	}
}
