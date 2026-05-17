package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/internal/storage"
	"go.uber.org/zap"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	log, err := zap.NewProduction()
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

	fmt.Println("Running migrations...")
	if err := db.MigrateUp(migrationsPath, dsn); err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Migrations complete.")
}

func runSave(cfg *config.Config, pageURL string, log *zap.Logger) {
	w := storage.NewWriter(cfg.Storage.BasePath)

	spaceKey := "unknown"
	pageTitle := "untitled"

	dir, err := w.MakePageDir(spaceKey, pageTitle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create page dir: %v\n", err)
		os.Exit(1)
	}

	meta := &storage.Metadata{
		Title:         pageTitle,
		ConfluenceURL: pageURL,
		SpaceKey:      spaceKey,
		SavedAt:       time.Now(),
	}

	if err := w.SaveMetadata(dir, meta); err != nil {
		fmt.Fprintf(os.Stderr, "failed to save metadata: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Page saved to: %s\n", dir)
}

func runServe(cfg *config.Config, log *zap.Logger) {
	fmt.Printf("Starting server on :%d\n", cfg.MCP.Port)
	// Phase 5: MCP server
	// Phase 2: API server
	log.Info("server started", zap.Int("port", cfg.MCP.Port))
}

func printUsage() {
	fmt.Println("Usage: spacemosquito <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init        Run database migrations")
	fmt.Println("  save <url>  Save a Confluence page")
	fmt.Println("  serve       Start the API and MCP server")
}
