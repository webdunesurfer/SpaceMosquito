package mcp

import (
	"testing"
)

func TestParseListSpaceArgs(t *testing.T) {
	t.Run("valid defaults", func(t *testing.T) {
		got, err := parseListSpaceArgs(map[string]interface{}{"space_key": "PROJ"})
		if err != nil {
			t.Fatal(err)
		}
		if got.SpaceKey != "PROJ" || got.Limit != 50 || got.AfterConfluenceID != nil || got.IncludeContent {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("missing space_key", func(t *testing.T) {
		_, err := parseListSpaceArgs(map[string]interface{}{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("limit capped at 200", func(t *testing.T) {
		got, err := parseListSpaceArgs(map[string]interface{}{
			"space_key": "PROJ",
			"limit":     float64(999),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Limit != 200 {
			t.Errorf("limit = %d, want 200", got.Limit)
		}
	})

	t.Run("include_content caps limit at 50", func(t *testing.T) {
		got, err := parseListSpaceArgs(map[string]interface{}{
			"space_key":       "PROJ",
			"limit":           float64(999),
			"include_content": true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !got.IncludeContent {
			t.Fatal("expected include_content")
		}
		if got.Limit != 50 {
			t.Errorf("limit = %d, want 50", got.Limit)
		}
	})

	t.Run("after_confluence_id cursor", func(t *testing.T) {
		got, err := parseListSpaceArgs(map[string]interface{}{
			"space_key":           "PROJ",
			"after_confluence_id": float64(12345),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.AfterConfluenceID == nil || *got.AfterConfluenceID != 12345 {
			t.Fatalf("after = %v", got.AfterConfluenceID)
		}
	})

	t.Run("after_confluence_id zero is no cursor", func(t *testing.T) {
		got, err := parseListSpaceArgs(map[string]interface{}{
			"space_key":           "PROJ",
			"after_confluence_id": float64(0),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.AfterConfluenceID != nil {
			t.Fatalf("after = %v, want nil", got.AfterConfluenceID)
		}
	})

	t.Run("invalid after_confluence_id", func(t *testing.T) {
		_, err := parseListSpaceArgs(map[string]interface{}{
			"space_key":           "PROJ",
			"after_confluence_id": float64(1.5),
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
