package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/internal/session"
	"github.com/vkh/spacemosquito/internal/scraper"
	"github.com/vkh/spacemosquito/internal/storage"
	"github.com/vkh/spacemosquito/pkg/logger"
	"github.com/vkh/spacemosquito/pkg/logging"
	"go.uber.org/zap"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	log, err := logger.NewProduction(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		home, _ := os.UserConfigDir()
		cfgPath = filepath.Join(home, "spacemosquito", "config.yaml")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		runInit(cfg, log)
	case "save":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: spacemosquito save <url>")
			os.Exit(1)
		}
		runSave(cfg, os.Args[2], log)
	case "crawl":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: spacemosquito crawl <space-url>")
			os.Exit(1)
		}
		runCrawl(cfg, os.Args[2], log)
	case "serve":
		runServe(cfg, log)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runInit(cfg *config.Config, log *zap.Logger) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName, cfg.Database.SSLMode,
	)

	migrationsPath := "migrations"
	if abs, err := filepath.Abs(migrationsPath); err == nil {
		migrationsPath = abs
	}

	sugar := logging.New("cli", log)
	sugar.Infow("running migrations", "path", migrationsPath)

	if err := db.MigrateUp(migrationsPath, dsn, sugar); err != nil {
		sugar.Errorw("migration failed", "error", err)
		os.Exit(1)
	}
	sugar.Info("migrations complete")
}

func runSave(cfg *config.Config, pageURL string, log *zap.Logger) {
	sugar := logging.New("cli", log)
	w := storage.NewWriter(cfg.Storage.BasePath, sugar)

	spaceKey := "unknown"
	pageTitle := "untitled"

	dir, err := w.MakePageDir(spaceKey, pageTitle)
	if err != nil {
		sugar.Errorw("failed to create page dir", "error", err)
		os.Exit(1)
	}

	meta := &storage.Metadata{
		Title:         pageTitle,
		ConfluenceURL: pageURL,
		SpaceKey:      spaceKey,
		SavedAt:       time.Now(),
	}

	if err := w.SaveMetadata(dir, meta); err != nil {
		sugar.Errorw("failed to save metadata", "error", err)
		os.Exit(1)
	}

	sugar.Infow("page saved", "path", dir, "url", pageURL)
}

func runServe(cfg *config.Config, log *zap.Logger) {
	sugar := logging.New("cli", log)
	sugar.Infow("starting server", "port", cfg.MCP.Port)
	// Phase 5: MCP server
	// Phase 2: API server
}

func runCrawl(cfg *config.Config, spaceURL string, log *zap.Logger) {
	requestID := fmt.Sprintf("crawl-%d", time.Now().Unix())
	sugar := logging.New("crawl", log)
	sugar.Infow("crawl command initiated",
		"space_url", spaceURL,
		"request_id", requestID)

	// Setup database
	database, err := db.New(&cfg.Database, log)
	if err != nil {
		sugar.Errorw("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Load session
	store := session.NewStore(cfg.Session.FilePath, sugar)
	if !store.HasSession() {
		sugar.Errorw("no session found — run the Firefox extension to capture cookies first")
		os.Exit(1)
	}

	encKey := cfg.Session.EncryptionKey
	if encKey == "" {
		sugar.Errorw("encryption key not configured")
		os.Exit(1)
	}

	sess, err := store.Load(encKey)
	if err != nil {
		sugar.Errorw("failed to load session", "error", err)
		os.Exit(1)
	}

	sugar.Infow("session loaded",
		"cookie_count", len(sess.Cookies),
		"confluence_url", sess.ConfluenceURL)

	// Setup storage
	storageWriter := storage.NewWriter(cfg.Storage.BasePath, sugar)
	assetDownloader := storage.NewAssetDownloader(sugar)

	// Create scraper and run crawl
	s := scraper.New(cfg, database, storageWriter, assetDownloader, sugar)

	if err := s.LaunchBrowser(); err != nil {
		sugar.Errorw("failed to launch browser", "error", err)
		os.Exit(1)
	}

	if err := s.CrawlSpace(spaceURL, sess); err != nil {
		sugar.Errorw("crawl failed", "error", err)
		os.Exit(1)
	}

	sugar.Infow("crawl completed successfully", "request_id", requestID)

	fmt.Println()
	fmt.Println("=== Crawl Summary ===")
	fmt.Println()
}

func printUsage() {
	fmt.Println("Usage: spacemosquito <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init        Run database migrations")
	fmt.Println("  save <url>  Save a Confluence page")
	fmt.Println("  crawl <url> Crawl a full Confluence space")
	fmt.Println("  serve       Start the API and MCP server")
}
