package search

import (
	"context"
	"fmt"

	"github.com/vkh/spacemosquito/internal/store"
)

// GetPageDetail resolves a page by Confluence ID and maps it to the public API shape.
func GetPageDetail(ctx context.Context, db store.Store, confluenceID int, spaceKey string, exposeInternalIDs bool) (PageDetail, error) {
	if confluenceID <= 0 {
		return PageDetail{}, fmt.Errorf("invalid confluence_id")
	}
	page, resolvedKey, err := db.GetPageByConfluenceID(ctx, confluenceID, spaceKey)
	if err != nil {
		return PageDetail{}, err
	}
	return ToPageDetail(page, resolvedKey, exposeInternalIDs), nil
}
