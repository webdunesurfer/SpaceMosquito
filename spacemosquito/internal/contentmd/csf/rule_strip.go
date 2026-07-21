package csf

import "github.com/PuerkitoBio/goquery"

// stripRule removes editor/metadata noise:
//   - ac:placeholder      → dropped (editor ghost text)
//   - ac:parameter        → dropped (macro parameters; rules read them via
//     attributes, never via the walk, so dropping here kills "1solid"-class noise)
//   - ac:inline-comment-marker → unwrapped (inner text kept)
type stripRule struct{}

func (stripRule) Name() string { return "strip" }

func (stripRule) Match(sel *goquery.Selection) bool {
	switch tagName(sel) {
	case "ac:placeholder", "ac:parameter", "ac:inline-comment-marker":
		return true
	}
	return false
}

func (stripRule) Render(sel *goquery.Selection, ctx *RenderContext) (string, error) {
	switch tagName(sel) {
	case "ac:inline-comment-marker":
		return ctx.RenderChildren(sel), nil // unwrap: keep inner text
	case "ac:parameter":
		// Drop the parameter's own value (its text is macro-parameter noise),
		// but still recurse element children: HTML parsing of a self-closing
		// <ac:parameter/> mis-nests following real content (e.g.
		// ac:rich-text-body) inside the parameter, which must not be lost.
		return renderElementChildren(sel, ctx), nil
	default: // ac:placeholder
		return "", nil
	}
}
