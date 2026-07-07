package search

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/vkh/spacemosquito/internal/db"
)

const (
	ListSpaceDefaultLimit         = 50
	ListSpaceMaxLimit             = 200
	ListSpaceMaxLimitWithContent  = 50
)

// ListSpaceOptions holds parsed list-space request parameters.
type ListSpaceOptions struct {
	Limit          int
	After          *int
	IncludeContent bool
}

// ListSpacePage is one row in a paginated space listing (MCP + REST).
type ListSpacePage struct {
	ConfluenceID       int       `json:"confluence_id"`
	Title              string    `json:"title"`
	Version            int       `json:"version,omitempty"`
	ParentConfluenceID *int      `json:"parent_confluence_id,omitempty"`
	CreatedAt          time.Time `json:"created_at,omitempty"`
	UpdatedAt          time.Time `json:"updated_at,omitempty"`
	Content            string    `json:"content,omitempty"`
	InternalID         string    `json:"internal_id,omitempty"`
}

// ListSpaceResult is the paginated list response for MCP and REST.
type ListSpaceResult struct {
	SpaceKey              string          `json:"space_key"`
	Pages                 []ListSpacePage `json:"pages"`
	Count                 int             `json:"count"`
	HasMore               bool            `json:"has_more"`
	NextAfterConfluenceID *int            `json:"next_after_confluence_id,omitempty"`
}

// ClampListSpaceLimit applies default and max limits for MCP/REST list requests.
func ClampListSpaceLimit(limit int, includeContent bool) int {
	if limit <= 0 {
		limit = ListSpaceDefaultLimit
	}
	max := ListSpaceMaxLimit
	if includeContent {
		max = ListSpaceMaxLimitWithContent
	}
	if limit > max {
		return max
	}
	return limit
}

// NormalizeAfterConfluenceID treats nil and non-positive values as no cursor.
func NormalizeAfterConfluenceID(id *int) *int {
	if id == nil || *id <= 0 {
		return nil
	}
	return id
}

// ParseListSpaceQuery parses REST query parameters for space page listing.
func ParseListSpaceQuery(limitStr, afterStr, includeContentStr string) (ListSpaceOptions, error) {
	opts := ListSpaceOptions{}
	if limitStr != "" {
		l, parseErr := strconv.Atoi(limitStr)
		if parseErr != nil || l < 0 {
			return opts, fmt.Errorf("invalid limit")
		}
		opts.Limit = l
	}
	if includeContentStr != "" {
		include, parseErr := strconv.ParseBool(includeContentStr)
		if parseErr != nil {
			return opts, fmt.Errorf("invalid include_content")
		}
		opts.IncludeContent = include
	}
	opts.Limit = ClampListSpaceLimit(opts.Limit, opts.IncludeContent)

	if afterStr != "" {
		id, parseErr := strconv.Atoi(afterStr)
		if parseErr != nil {
			return opts, fmt.Errorf("invalid after_confluence_id")
		}
		opts.After = NormalizeAfterConfluenceID(&id)
	}
	return opts, nil
}

// ToListSpacePageSummary maps a DB summary row to the list shape.
func ToListSpacePageSummary(summary *db.PageSummary, exposeInternalIDs bool) ListSpacePage {
	row := ListSpacePage{
		ConfluenceID:       summary.ConfluenceID,
		Title:              summary.Title,
		Version:            summary.Version,
		ParentConfluenceID: summary.ParentConfluenceID,
		CreatedAt:          summary.CreatedAt,
		UpdatedAt:          summary.UpdatedAt,
	}
	if exposeInternalIDs && summary.ID != uuid.Nil {
		row.InternalID = summary.ID.String()
	}
	return row
}

// ToListSpacePageFull maps a DB page to the list shape with content (include_content mode).
func ToListSpacePageFull(page *db.Page, exposeInternalIDs bool) ListSpacePage {
	row := ListSpacePage{
		ConfluenceID:       page.ConfluenceID,
		Title:              page.Title,
		Version:            page.Version,
		ParentConfluenceID: page.ParentConfluenceID,
		CreatedAt:          page.CreatedAt,
		UpdatedAt:          page.UpdatedAt,
		Content:            page.Content,
	}
	if exposeInternalIDs && page.ID != uuid.Nil {
		row.InternalID = page.ID.String()
	}
	return row
}

// BuildListSpaceResultFromSummaries trims an over-fetched summary slice and sets pagination metadata.
func BuildListSpaceResultFromSummaries(spaceKey string, summaries []db.PageSummary, limit int, exposeInternalIDs bool) ListSpaceResult {
	hasMore := len(summaries) > limit
	if hasMore {
		summaries = summaries[:limit]
	}

	result := ListSpaceResult{
		SpaceKey: spaceKey,
		Pages:    make([]ListSpacePage, len(summaries)),
		Count:    len(summaries),
		HasMore:  hasMore,
	}
	for i := range summaries {
		result.Pages[i] = ToListSpacePageSummary(&summaries[i], exposeInternalIDs)
	}
	if hasMore && len(summaries) > 0 {
		last := summaries[len(summaries)-1].ConfluenceID
		result.NextAfterConfluenceID = &last
	}
	return result
}

// BuildListSpaceResultFromPages trims an over-fetched page slice and sets pagination metadata.
func BuildListSpaceResultFromPages(spaceKey string, pages []db.Page, limit int, exposeInternalIDs bool) ListSpaceResult {
	hasMore := len(pages) > limit
	if hasMore {
		pages = pages[:limit]
	}

	result := ListSpaceResult{
		SpaceKey: spaceKey,
		Pages:    make([]ListSpacePage, len(pages)),
		Count:    len(pages),
		HasMore:  hasMore,
	}
	for i := range pages {
		result.Pages[i] = ToListSpacePageFull(&pages[i], exposeInternalIDs)
	}
	if hasMore && len(pages) > 0 {
		last := pages[len(pages)-1].ConfluenceID
		result.NextAfterConfluenceID = &last
	}
	return result
}
