package bootstrap

import (
	"context"

	"github.com/vkh/spacemosquito/internal/contentmd"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logging"
)

// BackfillPageBodyFormats fills empty pages.body_format from on-disk metadata/raw.html.
// Safe to call on every startup; only touches rows with empty body_format.
func BackfillPageBodyFormats(ctx context.Context, db store.Store, log logging.Sugar) (updated int, err error) {
	pages, err := db.ListAllPages(ctx)
	if err != nil {
		return 0, err
	}
	for _, p := range pages {
		if p.BodyFormat == "storage" || p.BodyFormat == "rendered" {
			continue
		}
		format := "rendered"
		if p.FileDir != "" {
			format = contentmd.DetectBodyFormat(p.FileDir)
		}
		if format != "storage" && format != "rendered" {
			format = "rendered"
		}
		if err := db.UpdatePageBodyFormat(ctx, p.ID, format); err != nil {
			if log.Enabled() {
				log.Warnw("body_format backfill failed",
					"page_id", p.ID,
					"confluence_id", p.ConfluenceID,
					"error", err)
			}
			continue
		}
		updated++
	}
	if updated > 0 && log.Enabled() {
		log.Infow("body_format backfill complete", "updated", updated, "scanned", len(pages))
	}
	return updated, nil
}
