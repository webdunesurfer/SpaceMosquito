package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/cron"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func TestCronAPIHandler_ConfigGet_nilYAMLBlocks(t *testing.T) {
	cfg := &config.Config{
		// FullCrawl and Incremental left nil — reproduces extension popup panic.
		Cron: config.CronConfig{},
	}
	manager := cron.NewManager(filepath.Join(t.TempDir(), "cron.json"), logging.Sugar{})
	h := NewCronAPIHandler(cfg, manager, nil, logging.Sugar{})

	req := httptest.NewRequest(http.MethodGet, "/api/cron/config", nil)
	rec := httptest.NewRecorder()
	h.ConfigGet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"yaml_full_crawl", "yaml_incremental"} {
		block, ok := body[key].(map[string]interface{})
		if !ok {
			t.Fatalf("%s missing or wrong type: %#v", key, body[key])
		}
		if enabled, _ := block["enabled"].(bool); enabled {
			t.Fatalf("%s.enabled = true, want false", key)
		}
		if block["interval"] != "" {
			t.Fatalf("%s.interval = %#v, want empty", key, block["interval"])
		}
	}
}

func TestCronAPIHandler_ConfigGet_populated(t *testing.T) {
	cfg := &config.Config{
		Cron: config.CronConfig{
			FullCrawl: &config.CronJobConfig{
				Enabled:     true,
				Interval:    "24h",
				Spaces:      []string{"PROJ"},
				MaxDuration: "4h",
			},
			Incremental: &config.CronJobConfig{
				Enabled:   false,
				Interval:  "1h",
				Detection: "api",
			},
		},
	}
	manager := cron.NewManager(filepath.Join(t.TempDir(), "cron.json"), logging.Sugar{})
	h := NewCronAPIHandler(cfg, manager, nil, logging.Sugar{})

	req := httptest.NewRequest(http.MethodGet, "/api/cron/config", nil)
	rec := httptest.NewRecorder()
	h.ConfigGet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var body struct {
		Full struct {
			Enabled  bool     `json:"enabled"`
			Interval string   `json:"interval"`
			Spaces   []string `json:"spaces"`
		} `json:"yaml_full_crawl"`
		Incr struct {
			Enabled   bool   `json:"enabled"`
			Detection string `json:"detection"`
		} `json:"yaml_incremental"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Full.Enabled || body.Full.Interval != "24h" || len(body.Full.Spaces) != 1 {
		t.Fatalf("yaml_full_crawl = %+v", body.Full)
	}
	if body.Incr.Enabled || body.Incr.Detection != "api" {
		t.Fatalf("yaml_incremental = %+v", body.Incr)
	}
}

func TestCronJobYAMLView_nil(t *testing.T) {
	v := cronJobYAMLView(nil)
	if v["enabled"] != false {
		t.Fatalf("%v", v)
	}
}
