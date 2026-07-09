//go:build integration

package app

import (
	"net/http/httptest"
	"testing"

	"github.com/vkh/spacemosquito/internal/api"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logger"
	"github.com/vkh/spacemosquito/pkg/logging"
)

// TestServer is an in-process HTTP server wired like production serve.
type TestServer struct {
	URL    string
	Store  store.Store
	Config *config.Config
	close  func()
}

// NewTestServer boots migrations, store, and the full HTTP mux (cron not started).
func NewTestServer(t *testing.T, cfg *config.Config) *TestServer {
	t.Helper()

	log, err := logger.NewProduction(nil)
	if err != nil {
		t.Fatalf("logger: %v", err)
	}

	comps, err := setupComponents(cfg, log)
	if err != nil {
		t.Fatalf("setupComponents: %v", err)
	}

	httpLog := logging.New("http", log)
	handler := api.CORSMiddleware(api.LoggingMiddleware(comps.mux, httpLog), httpLog)
	srv := httptest.NewServer(handler)

	t.Cleanup(func() {
		srv.Close()
		comps.cronScheduler.Stop()
		comps.database.Close()
	})

	return &TestServer{
		URL:    srv.URL,
		Store:  comps.database,
		Config: comps.cfg,
		close:  srv.Close,
	}
}
