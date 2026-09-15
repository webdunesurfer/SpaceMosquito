package scraper

import (
	"context"
	"testing"

	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func TestReconcileSpacePagesTotal_raisesStaleTotal(t *testing.T) {
	db, _ := openTestDB(t, t.TempDir())
	ctx := context.Background()
	log := logging.Sugar{}

	spaceID, err := db.CreateSpace(ctx, "ENG", "Eng", "https://example.com/wiki/spaces/ENG")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateSpacePagesTotal(ctx, "ENG", 118); err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 120; i++ {
		if err := db.UpsertPage(ctx, &store.Page{
			SpaceID:      spaceID,
			ConfluenceID: i,
			Version:      1,
			Title:        "p",
			Content:      "x",
		}); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}

	reconcileSpacePagesTotal(ctx, db, "ENG", 118, log)

	space, err := db.GetSpaceByKey(ctx, "ENG")
	if err != nil {
		t.Fatal(err)
	}
	if space.PagesTotal != 120 {
		t.Fatalf("pages_total = %d, want 120", space.PagesTotal)
	}
}

func TestPersistDiscoveryPagesTotal_updatesEvenWhenSpaceExists(t *testing.T) {
	db, _ := openTestDB(t, t.TempDir())
	ctx := context.Background()
	log := logging.Sugar{}

	if _, err := db.CreateSpace(ctx, "ENG", "Eng", "https://example.com/wiki/spaces/ENG"); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateSpacePagesTotal(ctx, "ENG", 118); err != nil {
		t.Fatal(err)
	}

	persistDiscoveryPagesTotal(ctx, db, "ENG", "Eng", "https://example.com/wiki/spaces/ENG", 120, log)

	space, err := db.GetSpaceByKey(ctx, "ENG")
	if err != nil {
		t.Fatal(err)
	}
	if space.PagesTotal != 120 {
		t.Fatalf("pages_total = %d, want 120", space.PagesTotal)
	}
}
