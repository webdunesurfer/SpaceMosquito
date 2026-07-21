package csf

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// expandRule renders expand/details macros as a bold summary line followed by
// the body. The collapsed content is always shown (search/readability).
type expandRule struct{}

func (expandRule) Name() string { return "expand" }

func (expandRule) Match(sel *goquery.Selection) bool {
	if tagName(sel) != "ac:structured-macro" {
		return false
	}
	switch macroName(sel) {
	case "expand", "details":
		return true
	}
	return false
}

func (expandRule) Render(sel *goquery.Selection, ctx *RenderContext) (string, error) {
	title := macroParam(sel, "title")
	if title == "" {
		title = "Details"
	}
	body := strings.TrimSpace(ctx.RenderChildren(sel))
	out := "\n\n**" + title + "**\n\n"
	if body != "" {
		out += body + "\n\n"
	}
	return out, nil
}
