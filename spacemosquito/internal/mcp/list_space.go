package mcp

import (
	"fmt"
	"math"

	"github.com/vkh/spacemosquito/internal/search"
)

type listSpaceArgs struct {
	SpaceKey          string
	Limit             int
	AfterConfluenceID *int
	IncludeContent    bool
}

func parseListSpaceArgs(args map[string]interface{}) (listSpaceArgs, error) {
	spaceKey, ok := args["space_key"].(string)
	if !ok || spaceKey == "" {
		return listSpaceArgs{}, fmt.Errorf("space_key is required")
	}

	includeContent := false
	if v, ok := args["include_content"].(bool); ok {
		includeContent = v
	}

	limit := 0
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	limit = search.ClampListSpaceLimit(limit, includeContent)

	var after *int
	if v, ok := args["after_confluence_id"].(float64); ok {
		if v != math.Trunc(v) {
			return listSpaceArgs{}, fmt.Errorf("invalid after_confluence_id")
		}
		id := int(v)
		after = search.NormalizeAfterConfluenceID(&id)
	}

	return listSpaceArgs{
		SpaceKey:          spaceKey,
		Limit:             limit,
		AfterConfluenceID: after,
		IncludeContent:    includeContent,
	}, nil
}
