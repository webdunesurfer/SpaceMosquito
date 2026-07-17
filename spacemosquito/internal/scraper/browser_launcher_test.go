package scraper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func TestResolveChromiumBin_envOverride(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "chromium")
	if err := os.WriteFile(bin, []byte{0}, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envChromiumPath, bin)
	t.Cleanup(func() { t.Setenv(envChromiumPath, "") })

	got, source, err := resolveChromiumBin(nil, logging.Sugar{})
	if err != nil {
		t.Fatal(err)
	}
	if got != bin {
		t.Fatalf("bin = %q, want %q", got, bin)
	}
	if source != envChromiumPath {
		t.Fatalf("source = %q, want %q", source, envChromiumPath)
	}
}

func TestResolveChromiumBin_configPath(t *testing.T) {
	t.Setenv(envChromiumPath, "")
	bin := filepath.Join(t.TempDir(), "chrome")
	if err := os.WriteFile(bin, []byte{0}, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Browser: config.BrowserConfig{Path: bin}}
	got, source, err := resolveChromiumBin(cfg, logging.Sugar{})
	if err != nil {
		t.Fatal(err)
	}
	if got != bin {
		t.Fatalf("bin = %q, want %q", got, bin)
	}
	if source != "config" {
		t.Fatalf("source = %q, want config", source)
	}
}

func TestResolveChromiumBin_missingEnv(t *testing.T) {
	t.Setenv(envChromiumPath, "/nonexistent/chromium")
	_, _, err := resolveChromiumBin(nil, logging.Sugar{})
	if err == nil {
		t.Fatal("expected error for missing CHROMIUM_PATH")
	}
}
