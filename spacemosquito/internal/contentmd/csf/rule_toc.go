package csf

import "github.com/PuerkitoBio/goquery"

// tocRule drops the toc macro: a table of contents is navigation generated from
// headings and adds no searchable content.
type tocRule struct{}

func (tocRule) Name() string { return "toc" }

func (tocRule) Match(sel *goquery.Selection) bool {
	return tagName(sel) == "ac:structured-macro" && macroName(sel) == "toc"
}

func (tocRule) Render(_ *goquery.Selection, _ *RenderContext) (string, error) {
	return "", nil
}
