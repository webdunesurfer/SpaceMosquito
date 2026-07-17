package scraper

import (
	"fmt"
	"os"
	"runtime"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/paths"
	"github.com/vkh/spacemosquito/pkg/logging"
)

const envChromiumPath = "CHROMIUM_PATH"

// DownloadChromium pre-fetches Chromium into the data-directory browser folder.
func DownloadChromium(log logging.Sugar) error {
	dir, err := paths.ResolveBrowser()
	if err != nil {
		return err
	}
	if log.Enabled() {
		log.Infow("downloading Chromium", "dir", dir)
	}
	b := launcher.NewBrowser()
	b.RootDir = dir
	if _, err := b.Get(); err != nil {
		return fmt.Errorf("download chromium: %w", err)
	}
	if log.Enabled() {
		log.Info("Chromium download complete")
	}
	return nil
}

func resolveChromiumBin(cfg *config.Config, log logging.Sugar) (bin, source string, err error) {
	if path := os.Getenv(envChromiumPath); path != "" {
		if err := statExecutable(path); err != nil {
			return "", "", fmt.Errorf("%s: %w", envChromiumPath, err)
		}
		return path, envChromiumPath, nil
	}

	if cfg != nil && cfg.Browser.Path != "" {
		if err := statExecutable(cfg.Browser.Path); err != nil {
			return "", "", fmt.Errorf("browser.path: %w", err)
		}
		return cfg.Browser.Path, "config", nil
	}

	if found, ok := launcher.LookPath(); ok {
		return found, "lookpath", nil
	}

	dir, err := paths.ResolveBrowser()
	if err != nil {
		return "", "", err
	}
	if log.Enabled() {
		log.Infow("no system browser found, downloading Chromium", "dir", dir)
	}
	b := launcher.NewBrowser()
	b.RootDir = dir
	bin, err = b.Get()
	if err != nil {
		return "", "", fmt.Errorf("download chromium: %w", err)
	}
	return bin, "download", nil
}

func statExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%q is a directory", path)
	}
	return nil
}

func buildLauncher(bin string) *launcher.Launcher {
	l := launcher.New().
		Bin(bin).
		Headless(true).
		Set("disable-features", "VizDisplayCompositor,TranslateUI,BlinkGenPropertyTrees").
		Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	if runtime.GOOS == "linux" {
		l = l.
			NoSandbox(true).
			Set("disable-gpu", "").
			Set("disable-dev-shm-usage", "").
			Set("disable-gpu-sandbox", "").
			Set("disable-setuid-sandbox", "").
			Set("disable-seccomp-filter-sandbox", "")
	}

	return l
}
