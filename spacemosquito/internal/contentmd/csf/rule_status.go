package csf

import "github.com/PuerkitoBio/goquery"

// statusRule renders the status macro as its title in inline code (e.g.
// `Tested successfully`). The colour param is dropped.
type statusRule struct{}

func (statusRule) Name() string { return "status" }

func (statusRule) Match(sel *goquery.Selection) bool {
	return tagName(sel) == "ac:structured-macro" && macroName(sel) == "status"
}

func (statusRule) Render(sel *goquery.Selection, _ *RenderContext) (string, error) {
	title := macroParam(sel, "title")
	if title == "" {
		return "", nil
	}
	return renderInlineCode(title), nil
}
