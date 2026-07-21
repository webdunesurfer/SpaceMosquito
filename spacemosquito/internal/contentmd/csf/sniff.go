package csf

import "strings"

// IsStorageFormat reports whether s looks like Confluence Storage Format (the
// <ac:…>/<ri:…> XHTML dialect) rather than rendered HTML. It is the single
// classifier that routes a page to the CSF converter vs the generic converter,
// used at crawl time and (Phase 6) at reindex/import time.
func IsStorageFormat(s string) bool {
	return strings.Contains(s, "xmlns:ac") ||
		strings.Contains(s, "<ac:") ||
		strings.Contains(s, "<ri:")
}
