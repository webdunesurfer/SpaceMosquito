package search

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/vkh/spacemosquito/internal/db"
)

func TestToSearchHits_confluenceID(t *testing.T) {
	internal := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	results := []db.SearchResult{{
		ConfluenceID: 12345,
		SpaceKey:     "PROJ",
		Title:        "Test Page",
		Excerpt:      "hello",
		Similarity:   0.5,
		FilePath:     "saved/PROJ/Test/index.html",
		InternalID:   internal,
	}}

	hits := ToSearchHits(results, false)
	if len(hits) != 1 {
		t.Fatalf("len = %d, want 1", len(hits))
	}

	data, err := json.Marshal(hits[0])
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}

	if m["confluence_id"] != float64(12345) {
		t.Errorf("confluence_id = %v", m["confluence_id"])
	}
	if m["space_key"] != "PROJ" {
		t.Errorf("space_key = %v", m["space_key"])
	}
	if _, ok := m["PageID"]; ok {
		t.Error("should not contain PageID")
	}
	if _, ok := m["internal_id"]; ok {
		t.Error("internal_id should be omitted when expose flag is false")
	}
}

func TestToSearchHits_empty(t *testing.T) {
	hits := ToSearchHits(nil, false)
	if hits == nil || len(hits) != 0 {
		t.Errorf("expected empty slice, got %v", hits)
	}
}

func TestToPageDetail(t *testing.T) {
	pageID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	page := &db.Page{
		ID:           pageID,
		ConfluenceID: 99,
		Title:        "Title",
		Version:      3,
		Content:      "body text",
		HTMLPath:     "/secret/path",
	}
	detail := ToPageDetail(page, "PROJ", false)

	data, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{`"confluence_id":99`, `"space_key":"PROJ"`, `"content":"body text"`} {
		if !strings.Contains(s, want) {
			t.Errorf("json missing %q: %s", want, s)
		}
	}
	for _, omit := range []string{`html_path`, `file_dir`, `internal_id`} {
		if strings.Contains(s, omit) {
			t.Errorf("json should omit %q: %s", omit, s)
		}
	}
}
