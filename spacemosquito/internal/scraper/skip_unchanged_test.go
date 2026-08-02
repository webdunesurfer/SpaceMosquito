package scraper

import (
	"errors"
	"testing"

	"github.com/vkh/spacemosquito/internal/store"
)

func TestSkipUnchangedPage(t *testing.T) {
	existing := &store.Page{Version: 3}
	tests := []struct {
		name               string
		force              bool
		discoveredVersion  int
		existing           *store.Page
		err                error
		wantSkip           bool
	}{
		{name: "unchanged", discoveredVersion: 3, existing: existing, wantSkip: true},
		{name: "older catalog", discoveredVersion: 4, existing: existing, wantSkip: false},
		{name: "newer catalog", discoveredVersion: 2, existing: existing, wantSkip: true},
		{name: "force bypasses", force: true, discoveredVersion: 3, existing: existing, wantSkip: false},
		{name: "no version", discoveredVersion: 0, existing: existing, wantSkip: false},
		{name: "not found", discoveredVersion: 3, err: errors.New("missing"), wantSkip: false},
		{name: "nil existing", discoveredVersion: 3, existing: nil, wantSkip: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := skipUnchangedPage(tc.force, tc.discoveredVersion, tc.existing, tc.err)
			if got != tc.wantSkip {
				t.Fatalf("skipUnchangedPage = %v, want %v", got, tc.wantSkip)
			}
		})
	}
}

func TestScraper_SetForce(t *testing.T) {
	s := &Scraper{}
	s.SetForce(true)
	s.mu.Lock()
	got := s.force
	s.mu.Unlock()
	if !got {
		t.Fatal("expected force=true")
	}
}
