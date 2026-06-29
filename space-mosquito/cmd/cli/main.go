package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/vkh/spacemosquito/internal/app"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/internal/search"
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
	case "migrate-down":
		runMigrateDown(cfg, log)
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
	case "reindex":
		runReindex(cfg, log)
	case "search":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: spacemosquito search <query> [space-key]")
			os.Exit(1)
		}
		spaceKey := ""
		if len(os.Args) >= 4 {
			spaceKey = os.Args[3]
		}
		runSearch(cfg, os.Args[2], spaceKey, log)
	case "stats":
		runStats(cfg, log)
	case "cron":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: spacemosquito cron <list|config|run-now>")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "list":
			runCronList(cfg, log)
		case "config":
			runCronConfig(cfg)
		case "run-now":
			runCronRunNow(cfg, log)
		default:
			fmt.Fprintf(os.Stderr, "unknown cron subcommand: %s\n", os.Args[2])
			fmt.Fprintln(os.Stderr, "usage: spacemosquito cron <list|config|run-now>")
			os.Exit(1)
		}
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

func runMigrateDown(cfg *config.Config, log *zap.Logger) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName, cfg.Database.SSLMode,
	)

	migrationsPath := "migrations"
	if abs, err := filepath.Abs(migrationsPath); err == nil {
		migrationsPath = abs
	}

	sugar := logging.New("cli", log)
	sugar.Infow("rolling back migration", "path", migrationsPath)

	if err := db.MigrateDown(migrationsPath, dsn, sugar); err != nil {
		sugar.Errorw("migration rollback failed", "error", err)
		os.Exit(1)
	}
	sugar.Info("migration rolled back")
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
	_ = cfg
	_ = log
	fmt.Println("Starting SpaceMosquito server...")
	if err := app.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
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

func runReindex(cfg *config.Config, log *zap.Logger) {
	sugar := logging.New("reindex", log)

	database, err := db.New(&cfg.Database, log)
	if err != nil {
		sugar.Errorw("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := database.IndexAllPageContents(context.Background()); err != nil {
		sugar.Errorw("reindex failed", "error", err)
		os.Exit(1)
	}

	sugar.Info("all pages reindexed successfully")
	fmt.Println("All pages reindexed successfully")
}

func runSearch(cfg *config.Config, query, spaceKey string, log *zap.Logger) {
	sugar := logging.New("search", log)

	database, err := db.New(&cfg.Database, log)
	if err != nil {
		sugar.Errorw("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	results, err := database.SearchPages(context.Background(), query, spaceKey, 10)
	if err != nil {
		sugar.Errorw("search failed", "query", query, "error", err)
		os.Exit(1)
	}

	if results == nil {
		fmt.Println("No results found")
		return
	}

	hits := search.ToSearchHits(results, cfg.MCP.ExposeInternalIDs)

	fmt.Printf("\n=== Search Results for '%s' ===\n", query)
	if spaceKey != "" {
		fmt.Printf("Space: %s\n", spaceKey)
	}
	fmt.Printf("Total: %d results\n\n", len(hits))

	for i, r := range hits {
		fmt.Printf("%d. %s (Space: %s, ID: %d)\n", i+1, r.Title, r.SpaceKey, r.ConfluenceID)
		fmt.Printf("   Similarity: %.4f\n", r.Similarity)
		if r.Excerpt != "" {
			excerpt := r.Excerpt
			if len(excerpt) > 150 {
				excerpt = excerpt[:150] + "..."
			}
			fmt.Printf("   Excerpt: %s\n", excerpt)
		}
		fmt.Println()
	}
}

func runStats(cfg *config.Config, log *zap.Logger) {
	sugar := logging.New("stats", log)

	database, err := db.New(&cfg.Database, log)
	if err != nil {
		sugar.Errorw("stats failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	stats, err := database.GetPageStats(context.Background())
	if err != nil {
		sugar.Errorw("stats failed", "error", err)
		os.Exit(1)
	}

	fmt.Println("\n=== SpaceMosquito Statistics ===")
	fmt.Printf("Total Spaces: %d\n", stats.TotalSpaces)
	fmt.Printf("Total Pages: %d\n", stats.TotalPages)
	fmt.Printf("Content Indexing: %s\n", stats.ContentLang)
	fmt.Printf("Last Crawled: %s\n", stats.LastCrawledStr)
	fmt.Println()
}

func runCronList(cfg *config.Config, log *zap.Logger) {
	sugar := logging.New("cron", log)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/api/cron", cfg.MCP.Port))
	if err != nil {
		sugar.Errorw("failed to connect to server", "error", err)
		fmt.Fprintln(os.Stderr, "Cannot connect to server. Is it running?")
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Server returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var jobs []struct {
		ID       string    `json:"ID"`
		NextRun  time.Time `json:"NextRun"`
		LastRun  time.Time `json:"LastRun"`
		Disabled bool      `json:"Disabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		sugar.Errorw("failed to decode response", "error", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Scheduled Crawl Jobs ===")
	if len(jobs) == 0 {
		fmt.Println("No jobs configured")
		return
	}
	for _, j := range jobs {
		status := "active"
		if j.Disabled {
			status = "disabled"
		}
		fmt.Printf("  %s [%s]\n", j.ID, status)
		if !j.NextRun.IsZero() {
			fmt.Printf("    Next run: %s\n", j.NextRun.Format(time.RFC3339))
		}
		if !j.LastRun.IsZero() {
			fmt.Printf("    Last run: %s\n", j.LastRun.Format(time.RFC3339))
		}
	}
	fmt.Println()
}

func runCronConfig(cfg *config.Config) {
	fmt.Println("\n=== Cron Configuration ===")
	if cfg.Cron.FullCrawl == nil && cfg.Cron.Incremental == nil {
		fmt.Println("Cron is not configured")
		return
	}

	if cfg.Cron.FullCrawl != nil {
		fmt.Println("\nFull Crawl:")
		fmt.Printf("  Enabled:    %v\n", cfg.Cron.FullCrawl.Enabled)
		fmt.Printf("  Interval:   %s\n", cfg.Cron.FullCrawl.Interval)
		fmt.Printf("  Max Duration: %s\n", cfg.Cron.FullCrawl.MaxDuration)
		fmt.Printf("  Spaces:\n")
		for _, sp := range cfg.Cron.FullCrawl.Spaces {
			fmt.Printf("    - %s\n", sp)
		}
	}

	if cfg.Cron.Incremental != nil {
		fmt.Println("\nIncremental Scan:")
		fmt.Printf("  Enabled:    %v\n", cfg.Cron.Incremental.Enabled)
		fmt.Printf("  Interval:   %s\n", cfg.Cron.Incremental.Interval)
		fmt.Printf("  Max Duration: %s\n", cfg.Cron.Incremental.MaxDuration)
		fmt.Printf("  Detection:  %s\n", cfg.Cron.Incremental.Detection)
		fmt.Printf("  Spaces:\n")
		for _, sp := range cfg.Cron.Incremental.Spaces {
			fmt.Printf("    - %s\n", sp)
		}
	}
	fmt.Println()
}

func runCronRunNow(cfg *config.Config, log *zap.Logger) {
	sugar := logging.New("cron", log)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(fmt.Sprintf("http://localhost:%d/api/cron/start", cfg.MCP.Port), "application/json", nil)
	if err != nil {
		sugar.Errorw("failed to connect to server", "error", err)
		fmt.Fprintln(os.Stderr, "Cannot connect to server. Is it running?")
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Server returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	fmt.Println("Jobs triggered successfully")
}

func printUsage() {
	fmt.Println("Usage: spacemosquito <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init           Run database migrations")
	fmt.Println("  migrate-down   Rollback last migration")
	fmt.Println("  save <url>     Save a Confluence page")
	fmt.Println("  crawl <url>    Crawl a full Confluence space")
	fmt.Println("  search <q>     Search pages (optional: <space-key>)")
	fmt.Println("  reindex        Rebuild FTS indexes for all pages")
	fmt.Println("  stats          Show database statistics")
	fmt.Println("  cron list      List scheduled crawl jobs")
	fmt.Println("  cron config    Show cron configuration")
	fmt.Println("  cron run-now   Trigger all jobs immediately")
	fmt.Println("  serve          Start the API and MCP server")
}
