package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vkh/spacemosquito/internal/api"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/cron"
	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/internal/mcp"
	"github.com/vkh/spacemosquito/internal/session"
	"github.com/vkh/spacemosquito/internal/scraper"
	"github.com/vkh/spacemosquito/internal/storage"
	"github.com/vkh/spacemosquito/pkg/logger"
	"github.com/vkh/spacemosquito/pkg/logging"
	"go.uber.org/zap"
)

func New() (*zap.Logger, *config.Config, error) {
	log, err := logger.NewProduction(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create logger: %w", err)
	}

	log.Info("initializing SpaceMosquito")

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		home, _ := os.UserConfigDir()
		cfgPath = fmt.Sprintf("%s/spacemosquito/config.yaml", home)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal("failed to load config", zap.Error(err))
	}

	return log, cfg, nil
}

type serverComponents struct {
	database        *db.DB
	cfg             *config.Config
	sessionStore    *session.Store
	storageWriter   *storage.Writer
	assetDownloader *storage.AssetDownloader
	scraperInstance *scraper.Scraper
	mcpServer       *mcp.Server
	cronManager     *cron.Manager
	cronScheduler   *cron.Scheduler
	mux             *http.ServeMux
	log             *zap.Logger
}

func setupComponents(cfg *config.Config, log *zap.Logger) (*serverComponents, error) {
	database, err := db.New(&cfg.Database, log)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	migrationsPath := "migrations"
	if abs, err := os.Getwd(); err == nil {
		migrationsPath = abs + "/migrations"
	}

	if err := db.MigrateUp(migrationsPath, database.Pool().Config().ConnString(), database.Log()); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	sessionStore := session.NewStore(cfg.Session.FilePath, logging.New("session", log))

	storageWriter := storage.NewWriter(cfg.Storage.BasePath, logging.New("storage", log))
	assetDownloader := storage.NewAssetDownloader(logging.New("assets", log))
	scraperInstance := scraper.New(cfg, database, storageWriter, assetDownloader, logging.New("scraper", log))

	searchHandler := api.NewSearchHandler(database, sessionStore, cfg, logging.New("search", log))

	crawlJobManager := scraper.NewJobManager(cfg, database, sessionStore, storageWriter, assetDownloader, logging.New("crawl", log))
	crawlHandler := api.NewCrawlHandler(crawlJobManager, cfg, logging.New("crawl_api", log))

	mcpServer := mcp.New(database, sessionStore, cfg, logging.New("mcp", log))
	mcp.ServerInstance = mcpServer

	cronConfigPath := os.Getenv("CRON_CONFIG_PATH")
	if cronConfigPath == "" {
		cronConfigPath = "./cron-config.json"
	}
	cronManager := cron.NewManager(cronConfigPath, logging.New("cron_config", log))

	cronScheduler := cron.NewScheduler(cfg, cronManager, database, sessionStore, storageWriter, assetDownloader, logging.New("cron", log))
	cronAPI := api.NewCronAPIHandler(cfg, cronManager, cronScheduler, logging.New("cron_api", log))
	cronSpaceAPI := api.NewCronSpaceHandler(cronManager, cronScheduler, logging.New("cron_space", log))

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /api/session", api.New(sessionStore, cfg, logging.New("api", log)).CreateSession)
	mux.HandleFunc("DELETE /api/session", api.New(sessionStore, cfg, logging.New("api", log)).DeleteSession)
	mux.HandleFunc("GET /api/session/status", api.New(sessionStore, cfg, logging.New("api", log)).SessionStatus)
	mux.HandleFunc("POST /api/session/validate", api.New(sessionStore, cfg, logging.New("api", log)).ValidateSession)
	mux.HandleFunc("GET /api/search", searchHandler.Search)
	mux.HandleFunc("POST /api/search/reindex", searchHandler.Reindex)
	mux.HandleFunc("GET /api/search/stats", searchHandler.Stats)
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		mcpServer.HandleRequest(w, r)
	})
	mux.HandleFunc("/mcp/session/", func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.URL.Path[len("/mcp/session/"):]
		if sessionID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"session ID required"}`))
			return
		}
		mcpServer.HandleSessionRequest(w, r, sessionID)
	})
	mux.HandleFunc("POST /api/crawl", crawlHandler.Create)
	mux.HandleFunc("GET /api/crawl/status", crawlHandler.Status)
	mux.HandleFunc("GET /api/crawl", crawlHandler.List)
	mux.HandleFunc("POST /api/crawl/cancel", crawlHandler.Cancel)
	mux.HandleFunc("POST /api/crawl/cleanup", crawlHandler.Cleanup)

	mux.HandleFunc("GET /api/cron", cronAPI.List)
	mux.HandleFunc("POST /api/cron/start", cronAPI.StartNow)
	mux.HandleFunc("GET /api/cron/config", cronAPI.ConfigGet)
	mux.HandleFunc("POST /api/cron/config", cronAPI.ConfigUpdate)
	mux.HandleFunc("POST /api/cron/reload", cronAPI.ConfigReload)

	mux.HandleFunc("POST /api/cron/space/{key}", cronSpaceAPI.Update)
	mux.HandleFunc("GET /api/cron/space/{key}", cronSpaceAPI.ConfigGet)
	mux.HandleFunc("DELETE /api/cron/space/{key}", cronSpaceAPI.Delete)

	mux.HandleFunc("GET /api/spaces", api.SpacesHandler(database, logging.New("spaces", log)))
	mux.HandleFunc("POST /api/spaces", api.SpacesHandler(database, logging.New("spaces", log)))
	mux.HandleFunc("GET /api/spaces/{key}", api.SpaceByIDHandler(database, logging.New("spaces", log)))
	mux.HandleFunc("DELETE /api/spaces/{key}", api.SpaceByIDHandler(database, logging.New("spaces", log)))
	mux.HandleFunc("GET /api/spaces/{key}/pages", api.SpacePagesHandler(database, cfg, logging.New("spaces", log)))

	return &serverComponents{
		database:        database,
		cfg:             cfg,
		sessionStore:    sessionStore,
		storageWriter:   storageWriter,
		assetDownloader: assetDownloader,
		scraperInstance: scraperInstance,
		mcpServer:       mcpServer,
		cronManager:     cronManager,
		cronScheduler:   cronScheduler,
		mux:             mux,
		log:             log,
	}, nil
}

func (c *serverComponents) Start(ctx context.Context) error {
	if err := c.scraperInstance.LaunchBrowser(); err != nil {
		c.log.Warn("failed to launch scraper browser", zap.Error(err))
	}

	if err := c.cronScheduler.Start(ctx); err != nil {
		c.log.Warn("failed to start cron scheduler", zap.Error(err))
	}

	loggingMux := api.LoggingMiddleware(c.mux, logging.New("http", c.log))
	corsMux := api.CORSMiddleware(loggingMux, logging.New("http", c.log))

	addr := fmt.Sprintf("%s:%d", c.cfg.MCP.Host, c.cfg.MCP.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: corsMux,
	}

	go func() {
		fmt.Printf("Server listening on %s\n", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			c.log.Fatal("server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	c.log.Info("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		c.log.Error("server shutdown failed", zap.Error(err))
	}

	c.scraperInstance.CloseBrowser()
	c.cronScheduler.Stop()
	c.database.Close()

	c.log.Info("server stopped")
	return nil
}

func Run(ctx context.Context) error {
	log, cfg, err := New()
	if err != nil {
		return err
	}
	defer log.Sync()

	comps, err := setupComponents(cfg, log)
	if err != nil {
		return err
	}

	return comps.Start(ctx)
}
