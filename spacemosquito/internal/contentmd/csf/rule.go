// Package csf converts Confluence Storage Format (the <ac:…>/<ri:…> XHTML
// dialect returned by the REST API's body.storage) into Markdown via an
// ordered, first-match-wins rule registry.
//
// See README.md for the top-level architecture and rules/README.md for the
// per-rule contract.
package csf

import "github.com/PuerkitoBio/goquery"

// Rule converts one recognized CSF node into a Markdown chunk.
type Rule interface {
	// Name identifies the rule (telemetry + test labels).
	Name() string
	// Match reports whether this rule handles the node.
	Match(sel *goquery.Selection) bool
	// Render produces Markdown. It may call ctx.RenderChildren(sel) for nested
	// content and ctx.RequestAsset(...) to schedule an asset download. Render
	// must be pure: no network, no DB.
	Render(sel *goquery.Selection, ctx *RenderContext) (string, error)
}

// tagName returns the lower-cased tag name of the node backing sel, including
// any namespace prefix (e.g. "ac:structured-macro"). Empty for non-elements.
func tagName(sel *goquery.Selection) string {
	n := sel.Get(0)
	if n == nil {
		return ""
	}
	return n.Data
}
