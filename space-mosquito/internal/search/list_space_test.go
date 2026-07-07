package search

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/vkh/spacemosquito/internal/db"
)

func TestClampListSpaceLimit(t *testing.T) {
	tests := []struct {
		in             int
		includeContent bool
		want           int
	}{
		{0, false, ListSpaceDefaultLimit},
		{-1, false, ListSpaceDefaultLimit},
		{50, false, 50},
		{200, false, 200},
		{500, false, ListSpaceMaxLimit},
		{0, true, ListSpaceDefaultLimit},
		{50, true, ListSpaceMaxLimitWithContent},
		{100, true, ListSpaceMaxLimitWithContent},
	}
	for _, tc := range tests {
		if got := ClampListSpaceLimit(tc.in, tc.includeContent); got != tc.want {
			t.Errorf("ClampListSpaceLimit(%d, %v) = %d, want %d", tc.in, tc.includeContent, got, tc.want)
		}
	}
}

func TestNormalizeAfterConfluenceID(t *testing.T) {
	zero := 0
	neg := -1
	pos := 100

	if got := NormalizeAfterConfluenceID(nil); got != nil {
		t.Errorf("nil = %v, want nil", got)
	}
	if got := NormalizeAfterConfluenceID(&zero); got != nil {
		t.Errorf("0 = %v, want nil", got)
	}
	if got := NormalizeAfterConfluenceID(&neg); got != nil {
		t.Errorf("-1 = %v, want nil", got)
	}
	if got := NormalizeAfterConfluenceID(&pos); got == nil || *got != 100 {
		t.Errorf("100 = %v, want 100", got)
	}
}

func TestParseListSpaceQuery(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		opts, err := ParseListSpaceQuery("", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if opts.Limit != ListSpaceDefaultLimit || opts.After != nil || opts.IncludeContent {
			t.Fatalf("opts=%+v", opts)
		}
	})

	t.Run("include_content", func(t *testing.T) {
		opts, err := ParseListSpaceQuery("100", "", "true")
		if err != nil {
			t.Fatal(err)
		}
		if !opts.IncludeContent {
			t.Fatal("expected include_content")
		}
		if opts.Limit != ListSpaceMaxLimitWithContent {
			t.Errorf("limit = %d, want %d", opts.Limit, ListSpaceMaxLimitWithContent)
		}
	})

	t.Run("after zero is no cursor", func(t *testing.T) {
		opts, err := ParseListSpaceQuery("10", "0", "")
		if err != nil {
			t.Fatal(err)
		}
		if opts.After != nil {
			t.Fatalf("after=%v, want nil", opts.After)
		}
	})

	t.Run("invalid limit", func(t *testing.T) {
		_, err := ParseListSpaceQuery("nope", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid after", func(t *testing.T) {
		_, err := ParseListSpaceQuery("", "nope", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid include_content", func(t *testing.T) {
		_, err := ParseListSpaceQuery("", "", "nope")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestBuildListSpaceResultFromSummaries_hasMore(t *testing.T) {
	now := time.Now()
	summaries := []db.PageSummary{
		{ConfluenceID: 10, Title: "A", UpdatedAt: now},
		{ConfluenceID: 20, Title: "B", UpdatedAt: now},
		{ConfluenceID: 30, Title: "C", UpdatedAt: now},
	}

	result := BuildListSpaceResultFromSummaries("PROJ", summaries, 2, false)
	if result.Count != 2 {
		t.Errorf("count = %d, want 2", result.Count)
	}
	if !result.HasMore {
		t.Error("has_more should be true")
	}
	if result.NextAfterConfluenceID == nil || *result.NextAfterConfluenceID != 20 {
		t.Errorf("next_after = %v, want 20", result.NextAfterConfluenceID)
	}
	if len(result.Pages) != 2 || result.Pages[1].ConfluenceID != 20 {
		t.Errorf("pages = %+v", result.Pages)
	}
}

func TestBuildListSpaceResultFromSummaries_noMore(t *testing.T) {
	summaries := []db.PageSummary{{ConfluenceID: 1, Title: "Only"}}
	result := BuildListSpaceResultFromSummaries("PROJ", summaries, 50, false)
	if result.HasMore {
		t.Error("has_more should be false")
	}
	if result.NextAfterConfluenceID != nil {
		t.Errorf("next_after = %v, want nil", result.NextAfterConfluenceID)
	}
}

func TestBuildListSpaceResultFromSummaries_omitsContent(t *testing.T) {
	summaries := []db.PageSummary{{ConfluenceID: 1, Title: "T", CreatedAt: time.Now()}}
	result := BuildListSpaceResultFromSummaries("PROJ", summaries, 50, false)
	data, err := json.Marshal(result.Pages[0])
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, omit := range []string{`"content"`, `"html_path"`, `"file_dir"`} {
		if strings.Contains(s, omit) {
			t.Errorf("summary json should omit %s: %s", omit, s)
		}
	}
}

func TestBuildListSpaceResultFromPages_includesContent(t *testing.T) {
	pages := []db.Page{{ConfluenceID: 1, Title: "T", Content: "body"}}
	result := BuildListSpaceResultFromPages("PROJ", pages, 50, false)
	if result.Pages[0].Content != "body" {
		t.Errorf("content = %q", result.Pages[0].Content)
	}
}

func TestToListSpacePageSummary_exposeInternalID(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	summary := &db.PageSummary{
		ID:           id,
		ConfluenceID: 42,
		Title:        "T",
	}

	if got := ToListSpacePageSummary(summary, false); got.InternalID != "" {
		t.Errorf("internal_id = %q, want empty", got.InternalID)
	}
	if got := ToListSpacePageSummary(summary, true); got.InternalID != id.String() {
		t.Errorf("internal_id = %q", got.InternalID)
	}
}

func TestToListSpacePageFull_exposeInternalID(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	page := &db.Page{
		ID:           id,
		ConfluenceID: 42,
		Title:        "T",
		Content:      "body",
	}

	if got := ToListSpacePageFull(page, false); got.InternalID != "" {
		t.Errorf("internal_id = %q, want empty", got.InternalID)
	}
	if got := ToListSpacePageFull(page, true); got.InternalID != id.String() {
		t.Errorf("internal_id = %q", got.InternalID)
	}
}
