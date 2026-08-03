package scraper

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/datastore"
	"github.com/vkh/spacemosquito/internal/session"
	"github.com/vkh/spacemosquito/internal/storage"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logger"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func openTestDB(t *testing.T, dir string) (store.Store, *config.Config) {
	t.Helper()
	cfg := &config.Config{
		Database: config.DatabaseConfig{Driver: "sqlite", Path: filepath.Join(dir, "test.db")},
		Storage:  config.StorageConfig{BasePath: filepath.Join(dir, "saved")},
	}
	log, err := logger.NewProduction(nil)
	if err != nil {
		t.Fatal(err)
	}
	migrationsRoot := filepath.Join("..", "..", "migrations")
	if err := datastore.MigrateUp(cfg, migrationsRoot, logging.New("test", log)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db, err := datastore.Open(cfg, log)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, cfg
}

func TestCrawlPage_overwriteWipeAndForceAssets(t *testing.T) {
	dir := t.TempDir()
	db, cfg := openTestDB(t, dir)

	var assetHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/rest/api/content/42"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":    "42",
				"title": "Renamed Page",
				"version": map[string]any{
					"number": 7,
				},
				"space": map[string]any{"key": "PROJ"},
				"body": map[string]any{
					"storage": map[string]any{
						"value": `<p>fresh body</p><ac:image><ri:attachment ri:filename="pic.png"/></ac:image>`,
					},
				},
				"_links": map[string]any{
					"webui": "/wiki/spaces/PROJ/pages/42/Renamed+Page",
				},
			})
		case strings.Contains(r.URL.Path, "/download/attachments/42/pic.png"):
			assetHits.Add(1)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("new-png"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	spaceID, err := db.CreateSpace(ctx, "PROJ", "Proj", srv.URL+"/wiki/spaces/PROJ")
	if err != nil {
		t.Fatal(err)
	}

	pageDir := filepath.Join(cfg.Storage.BasePath, "PROJ", "Old Title")
	if err := os.MkdirAll(filepath.Join(pageDir, "assets", "images"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageDir, "index.html"), []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageDir, "stale-orphan.txt"), []byte("orphan"), 0644); err != nil {
		t.Fatal(err)
	}
	oldAsset := filepath.Join(pageDir, "assets", "images", "pic.png")
	if err := os.WriteFile(oldAsset, []byte("old-png"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := db.UpsertPage(ctx, &store.Page{
		SpaceID:      spaceID,
		ConfluenceID: 42,
		Version:      1,
		Title:        "Old Title",
		Content:      "stale",
		FileDir:      pageDir,
	}); err != nil {
		t.Fatal(err)
	}

	writer := storage.NewWriter(cfg.Storage.BasePath, logging.Sugar{})
	assets := storage.NewAssetDownloader(logging.Sugar{})
	assets.SetClientForTest(srv.Client())
	s := New(cfg, db, writer, assets, logging.Sugar{})
	sess := &session.Session{
		ConfluenceURL: srv.URL,
		Flavor:        session.FlavorCloud,
	}

	if err := s.CrawlPage("PROJ", 42, sess); err != nil {
		t.Fatalf("CrawlPage: %v", err)
	}

	if assetHits.Load() != 1 {
		t.Fatalf("expected forced asset GET, hits=%d", assetHits.Load())
	}

	got, err := db.GetPage(ctx, "PROJ", 42)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Renamed Page" || got.Version != 7 {
		t.Fatalf("db page = title=%q version=%d", got.Title, got.Version)
	}
	if got.FileDir != pageDir {
		t.Fatalf("file_dir changed to %q, want reuse %q", got.FileDir, pageDir)
	}
	if !strings.Contains(got.Content, "fresh body") {
		t.Fatalf("content not updated: %q", got.Content)
	}

	if _, err := os.Stat(filepath.Join(pageDir, "stale-orphan.txt")); !os.IsNotExist(err) {
		t.Fatal("orphan file should be wiped")
	}
	png, err := os.ReadFile(filepath.Join(pageDir, "assets", "images", "pic.png"))
	if err != nil {
		t.Fatal(err)
	}
	if string(png) != "new-png" {
		t.Fatalf("asset = %q, want new-png", png)
	}
	// Title rename must not create a second page directory.
	newTitleDir := filepath.Join(cfg.Storage.BasePath, "PROJ", "Renamed Page")
	if _, err := os.Stat(newTitleDir); !os.IsNotExist(err) {
		t.Fatalf("unexpected new title dir created: %v", err)
	}
}

func TestCrawlPage_wrongSpace(t *testing.T) {
	dir := t.TempDir()
	db, cfg := openTestDB(t, dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "99",
			"title":   "Elsewhere",
			"version": map[string]any{"number": 1},
			"space":   map[string]any{"key": "OTHER"},
			"body":    map[string]any{"storage": map[string]any{"value": `<p>x</p>`}},
		})
	}))
	defer srv.Close()

	ctx := context.Background()
	if _, err := db.CreateSpace(ctx, "PROJ", "Proj", srv.URL+"/wiki/spaces/PROJ"); err != nil {
		t.Fatal(err)
	}

	assets := storage.NewAssetDownloader(logging.Sugar{})
	assets.SetClientForTest(srv.Client())
	s := New(cfg, db, storage.NewWriter(cfg.Storage.BasePath, logging.Sugar{}), assets, logging.Sugar{})
	err := s.CrawlPage("PROJ", 99, &session.Session{ConfluenceURL: srv.URL, Flavor: session.FlavorCloud})
	if err == nil {
		t.Fatal("expected wrong-space error")
	}
	if !strings.Contains(err.Error(), "OTHER") || !strings.Contains(err.Error(), "PROJ") {
		t.Fatalf("error = %v", err)
	}
}

func TestCrawlPage_notFound(t *testing.T) {
	dir := t.TempDir()
	db, cfg := openTestDB(t, dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	ctx := context.Background()
	if _, err := db.CreateSpace(ctx, "PROJ", "Proj", srv.URL+"/wiki/spaces/PROJ"); err != nil {
		t.Fatal(err)
	}

	assets := storage.NewAssetDownloader(logging.Sugar{})
	assets.SetClientForTest(srv.Client())
	s := New(cfg, db, storage.NewWriter(cfg.Storage.BasePath, logging.Sugar{}), assets, logging.Sugar{})
	err := s.CrawlPage("PROJ", 404, &session.Session{ConfluenceURL: srv.URL, Flavor: session.FlavorCloud})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveSpaceURL_fromSession(t *testing.T) {
	dir := t.TempDir()
	db, cfg := openTestDB(t, dir)
	s := New(cfg, db, storage.NewWriter(cfg.Storage.BasePath, logging.Sugar{}), nil, logging.Sugar{})

	cloud, err := s.resolveSpaceURL("ABC", &session.Session{
		ConfluenceURL: "https://ex.atlassian.net",
		Flavor:        session.FlavorCloud,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cloud != "https://ex.atlassian.net/wiki/spaces/ABC" {
		t.Fatalf("cloud url = %q", cloud)
	}

	server, err := s.resolveSpaceURL("ABC", &session.Session{
		ConfluenceURL: "https://wiki.example.com",
		Flavor:        session.FlavorServer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if server != "https://wiki.example.com/spaces/ABC" {
		t.Fatalf("server url = %q", server)
	}
}

func TestCrawlPage_rejectsBadArgs(t *testing.T) {
	s := &Scraper{}
	if err := s.CrawlPage("", 1, &session.Session{}); err == nil {
		t.Fatal("expected empty space key error")
	}
	if err := s.CrawlPage("X", 0, &session.Session{}); err == nil {
		t.Fatal("expected bad id error")
	}
	if err := s.CrawlPage("X", 1, nil); err == nil {
		t.Fatal("expected nil session error")
	}
}
