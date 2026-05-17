package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/db"
	"go.uber.org/zap"
)

func main() {
	log, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		home, _ := os.UserConfigDir()
		cfgPath = fmt.Sprintf("%s/spacemosquito/config.yaml", home)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal("failed to load config", zap.Error(err))
	}

	database, err := db.New(&cfg.Database, log)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer database.Close()

	migrationsPath := "migrations"
	if abs, err := os.Getwd(); err == nil {
		migrationsPath = abs + "/migrations"
	}

	if err := db.MigrateUp(migrationsPath, database.Pool().Config().ConnString()); err != nil {
		log.Fatal("failed to run migrations", zap.Error(err))
	}

	log.Info("initializing SpaceMosquito server")
	log.Info("MCP server will listen on", zap.Int("port", cfg.MCP.Port))

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf(":%d", cfg.MCP.Port)
	server := &http.Server{Addr: addr}

	go func() {
		fmt.Printf("Server listening on %s\n", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")
	if err := server.Close(); err != nil {
		log.Fatal("server shutdown failed", zap.Error(err))
	}
	log.Info("server stopped")
}
