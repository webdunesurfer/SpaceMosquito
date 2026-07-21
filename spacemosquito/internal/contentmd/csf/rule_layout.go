package csf

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// layoutRule renders Confluence page layout containers transparently: their
// children are emitted as blocks, the wrapper itself contributes nothing.
// Without this, the ~34k layout elements in the corpus would swallow most
// modern pages' content.
type layoutRule struct{}

func (layoutRule) Name() string { return "layout" }

func (layoutRule) Match(sel *goquery.Selection) bool {
	switch tagName(sel) {
	case "ac:layout", "ac:layout-section", "ac:layout-cell":
		return true
	}
	return false
}

func (layoutRule) Render(sel *goquery.Selection, ctx *RenderContext) (string, error) {
	inner := strings.TrimSpace(ctx.RenderChildren(sel))
	if inner == "" {
		return "", nil
	}
	return "\n\n" + inner + "\n\n", nil
}
