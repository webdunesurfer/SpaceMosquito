// Package confluence holds small shared helpers for working with Confluence
// URLs, usable by scraper, contentmd/csf, and reindex without import cycles.
package confluence

import (
	"fmt"
	"net/url"
)

// BaseURL returns the scheme://host origin of a Confluence URL (e.g.
// "https://wiki.company.net/spaces/SP/pages/1/Title" → "https://wiki.company.net").
// It returns "" for empty or unparseable input.
func BaseURL(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
}
