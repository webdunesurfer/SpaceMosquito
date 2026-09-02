package cron

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/vkh/spacemosquito/internal/confluence"
	"github.com/vkh/spacemosquito/internal/store"
)

// ResolvePageBrowseURL returns the page's real Confluence browse URL.
// Prefer metadata.json confluence_url (via MetadataPath or FileDir).
// If missing, derive a degraded URL from spaceURL + page id.
func ResolvePageBrowseURL(page store.Page, spaceKey, spaceURL string) string {
	if u := readConfluenceURL(page); u != "" {
		return u
	}
	return FallbackPageBrowseURL(spaceURL, spaceKey, page.ConfluenceID)
}

func readConfluenceURL(page store.Page) string {
	candidates := make([]string, 0, 2)
	if page.MetadataPath != "" {
		candidates = append(candidates, page.MetadataPath)
	}
	if page.FileDir != "" {
		candidates = append(candidates, filepath.Join(page.FileDir, "metadata.json"))
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var meta struct {
			ConfluenceURL string `json:"confluence_url"`
		}
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		if u := strings.TrimSpace(meta.ConfluenceURL); u != "" {
			return u
		}
	}
	return ""
}

// FallbackPageBrowseURL builds a degraded browse URL from the space URL and
// page id when metadata.json is missing or has no confluence_url. Prefer
// ResolvePageBrowseURL for production paths.
func FallbackPageBrowseURL(spaceURL, spaceKey string, pageID int) string {
	u, err := url.Parse(spaceURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	path := u.Path
	if i := strings.Index(path, "/spaces/"); i >= 0 {
		base := strings.TrimRight(fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, path[:i]), "/")
		return fmt.Sprintf("%s/spaces/%s/pages/%d", base, spaceKey, pageID)
	}
	if i := strings.Index(path, "/display/"); i >= 0 {
		base := strings.TrimRight(fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, path[:i]), "/")
		return fmt.Sprintf("%s/pages/viewpage.action?pageId=%d", base, pageID)
	}
	origin := confluence.BaseURL(spaceURL)
	if origin == "" {
		return ""
	}
	return fmt.Sprintf("%s/spaces/%s/pages/%d", origin, spaceKey, pageID)
}
