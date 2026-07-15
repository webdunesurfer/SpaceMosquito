package bootstrap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/contentmd"
	"github.com/vkh/spacemosquito/internal/datastore"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logging"
	"go.uber.org/zap"
)

func TestImportSaved_HappyPathAndSearch(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig(tmp)
	db := openSQLiteTestDB(t, cfg)
	defer db.Close()

	fromDir := filepath.Join(tmp, "saved")
	writeFixturePage(t, fromDir, "TST", "Overview", map[string]any{
		"title":          "Overview",
		"space_key":      "TST",
		"confluence_url": "https://example.atlassian.net/wiki/spaces/TST/pages/10/Overview",
		"updated_at":     time.Now().UTC().Format(time.RFC3339),
	}, "<h1>Overview</h1><p>alpha mosquito beta</p>")

	writeFixturePage(t, fromDir, "TST", "Architecture", map[string]any{
		"title":          "Architecture",
		"space_key":      "TST",
		"confluence_url": "https://example.atlassian.net/wiki/spaces/TST/pages/20/Architecture",
		"updated_at":     time.Now().UTC().Format(time.RFC3339),
	}, "<h1>Architecture</h1><p>systems page</p>")

	reportDir := filepath.Join(tmp, "reports")
	report, err := ImportSaved(context.Background(), db, Options{
		FromDir:   fromDir,
		ReportDir: reportDir,
	}, logging.Sugar{})
	if err != nil {
		t.Fatalf("ImportSaved: %v", err)
	}
	if report.ImportedPages != 2 {
		t.Fatalf("imported pages = %d, want 2", report.ImportedPages)
	}
	if report.ImportedSpaces != 1 {
		t.Fatalf("imported spaces = %d, want 1", report.ImportedSpaces)
	}

	results, err := db.SearchPages(context.Background(), "mosquito", "TST", 10)
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if len(results) != 1 || results[0].ConfluenceID != 10 {
		t.Fatalf("unexpected results: %+v", results)
	}

	reportFiles, err := filepath.Glob(filepath.Join(reportDir, "bootstrap-import-*.json"))
	if err != nil {
		t.Fatalf("glob reports: %v", err)
	}
	if len(reportFiles) != 1 {
		t.Fatalf("report files = %d, want 1", len(reportFiles))
	}

	page, err := db.GetPage(context.Background(), "TST", 10)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if !strings.Contains(page.Content, "mosquito") {
		t.Fatalf("content = %q, want markdown with mosquito", page.Content)
	}
	contentMD, err := os.ReadFile(filepath.Join(fromDir, "TST", "Overview", contentmd.ContentFileName))
	if err != nil {
		t.Fatalf("content.md: %v", err)
	}
	if !strings.Contains(string(contentMD), "mosquito") {
		t.Fatalf("content.md = %q", contentMD)
	}
}

func TestImportSaved_RequiresForceOnPopulatedDB(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig(tmp)
	db := openSQLiteTestDB(t, cfg)
	defer db.Close()

	spaceID, err := db.CreateSpace(context.Background(), "TST", "Test", "https://example/spaces/TST")
	if err != nil {
		t.Fatal(err)
	}
	err = db.UpsertPage(context.Background(), &store.Page{
		SpaceID:      spaceID,
		ConfluenceID: 1,
		Title:        "Existing",
		Content:      "existing content",
		HTMLPath:     filepath.Join(tmp, "saved", "TST", "Existing", "index.html"),
		MetadataPath: filepath.Join(tmp, "saved", "TST", "Existing", "metadata.json"),
		FileDir:      filepath.Join(tmp, "saved", "TST", "Existing"),
	})
	if err != nil {
		t.Fatal(err)
	}

	fromDir := filepath.Join(tmp, "saved")
	writeFixturePage(t, fromDir, "TST", "New", map[string]any{
		"title":          "New",
		"space_key":      "TST",
		"confluence_url": "https://example.atlassian.net/wiki/spaces/TST/pages/2/New",
		"updated_at":     time.Now().UTC().Format(time.RFC3339),
	}, "<p>new mosquito</p>")

	_, err = ImportSaved(context.Background(), db, Options{
		FromDir:   fromDir,
		ReportDir: filepath.Join(tmp, "reports"),
	}, logging.Sugar{})
	if err == nil {
		t.Fatal("expected error without force")
	}
}

func TestImportSaved_DeduplicatesByUpdatedAt(t *testing.T) {
	tmp := t.TempDir()
	cfg := testConfig(tmp)
	db := openSQLiteTestDB(t, cfg)
	defer db.Close()

	fromDir := filepath.Join(tmp, "saved")
	writeFixturePage(t, fromDir, "TST", "OldVersion", map[string]any{
		"title":          "OldVersion",
		"space_key":      "TST",
		"confluence_url": "https://example.atlassian.net/wiki/spaces/TST/pages/42/Page",
		"updated_at":     "2024-01-01T00:00:00Z",
	}, "<p>old body</p>")
	writeFixturePage(t, fromDir, "TST", "NewVersion", map[string]any{
		"title":          "NewVersion",
		"space_key":      "TST",
		"confluence_url": "https://example.atlassian.net/wiki/spaces/TST/pages/42/Page",
		"updated_at":     "2025-01-01T00:00:00Z",
	}, "<p>new body mosquito</p>")

	report, err := ImportSaved(context.Background(), db, Options{
		FromDir:   fromDir,
		ReportDir: filepath.Join(tmp, "reports"),
	}, logging.Sugar{})
	if err != nil {
		t.Fatalf("ImportSaved: %v", err)
	}
	if report.Deduplicated != 1 {
		t.Fatalf("deduplicated = %d, want 1", report.Deduplicated)
	}

	page, err := db.GetPage(context.Background(), "TST", 42)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if page.Title != "NewVersion" {
		t.Fatalf("title = %q, want NewVersion", page.Title)
	}
}

func testConfig(tmp string) *config.Config {
	return &config.Config{
		Database: config.DatabaseConfig{
			Driver: "sqlite",
			Path:   filepath.Join(tmp, "spacemosquito.db"),
		},
		Storage: config.StorageConfig{
			BasePath: filepath.Join(tmp, "saved"),
		},
	}
}

func openSQLiteTestDB(t *testing.T, cfg *config.Config) store.Store {
	t.Helper()
	migrationsRoot := filepath.Join(repoRoot(t), "migrations")
	if err := datastore.MigrateUp(cfg, migrationsRoot, logging.Sugar{}); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	db, err := datastore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return db
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

func writeFixturePage(t *testing.T, base, space, title string, meta map[string]any, html string) {
	t.Helper()
	dir := filepath.Join(base, space, title)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(dir, "metadata.json")
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaPath, metaBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
}
