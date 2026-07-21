package csf

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// fallbackRule handles any ac:structured-macro without a dedicated rule. It
// renders the macro's remaining children (rich-text-body / plain-text-body /
// nested content) as a block; parameters are already dropped by stripRule
// during recursion, so no macro-parameter noise survives. This single rule
// gives graceful degradation for the entire long tail of macro types.
type fallbackRule struct{}

func (fallbackRule) Name() string { return "fallback" }

func (fallbackRule) Match(sel *goquery.Selection) bool {
	return tagName(sel) == "ac:structured-macro"
}

func (fallbackRule) Render(sel *goquery.Selection, ctx *RenderContext) (string, error) {
	inner := strings.TrimSpace(ctx.RenderChildren(sel))
	if inner == "" {
		return "", nil
	}
	return "\n\n" + inner + "\n\n", nil
}
