package mcp

import (
	"fmt"
	"math"
)

func parseGetPageArgs(args map[string]interface{}) (spaceKey string, confluenceID int, err error) {
	spaceKey, ok := args["space_key"].(string)
	if !ok || spaceKey == "" {
		return "", 0, fmt.Errorf("space_key is required")
	}

	idFloat, ok := args["confluence_id"].(float64)
	if !ok {
		return "", 0, fmt.Errorf("confluence_id is required")
	}
	if idFloat != math.Trunc(idFloat) {
		return "", 0, fmt.Errorf("invalid confluence_id")
	}
	confluenceID = int(idFloat)
	if confluenceID <= 0 {
		return "", 0, fmt.Errorf("invalid confluence_id")
	}

	return spaceKey, confluenceID, nil
}
