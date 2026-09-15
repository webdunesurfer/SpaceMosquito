package scraper

import (
	"context"

	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logging"
)

// persistDiscoveryPagesTotal ensures the space row exists, then stores the
// discovery page count for idle crawled/total UI. Update runs even if
// CreateSpace fails (e.g. race), so a re-crawl cannot leave a stale total.
func persistDiscoveryPagesTotal(
	ctx context.Context,
	db store.Store,
	spaceKey, spaceName, spaceURL string,
	discoveryTotal int,
	log logging.Sugar,
) {
	if _, err := db.CreateSpace(ctx, spaceKey, spaceName, spaceURL); err != nil {
		log.Warnw("failed to ensure space after discovery",
			"space_key", spaceKey,
			"error", err)
	}
	if err := db.UpdateSpacePagesTotal(ctx, spaceKey, discoveryTotal); err != nil {
		log.Warnw("failed to store discovery pages_total",
			"space_key", spaceKey,
			"pages_total", discoveryTotal,
			"error", err)
	}
}

// reconcileSpacePagesTotal sets pages_total to max(discovery, catalog count)
// so idle UI never shows crawled > total after a crawl (stale leftover pages
// or a missed mid-crawl total update).
func reconcileSpacePagesTotal(
	ctx context.Context,
	db store.Store,
	spaceKey string,
	discoveryTotal int,
	log logging.Sugar,
) {
	space, err := db.GetSpaceByKey(ctx, spaceKey)
	if err != nil {
		log.Warnw("reconcile pages_total: get space failed",
			"space_key", spaceKey,
			"error", err)
		return
	}
	count, err := db.CountPagesBySpaceID(ctx, space.ID)
	if err != nil {
		log.Warnw("reconcile pages_total: count failed",
			"space_key", spaceKey,
			"error", err)
		return
	}
	total := discoveryTotal
	if count > total {
		total = count
	}
	if err := db.UpdateSpacePagesTotal(ctx, spaceKey, total); err != nil {
		log.Warnw("reconcile pages_total: update failed",
			"space_key", spaceKey,
			"pages_total", total,
			"error", err)
		return
	}
	if log.Enabled() && total != discoveryTotal {
		log.Infow("pages_total raised to catalog count",
			"space_key", spaceKey,
			"discovery_total", discoveryTotal,
			"catalog_count", count,
			"pages_total", total)
	}
}
