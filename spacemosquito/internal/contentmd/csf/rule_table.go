package csf

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// tableRule converts a native <table> into a GFM table. It is hand-rolled (not
// delegated to a library) so each cell can be routed back through the registry
// via ctx.RenderChildren, letting nested macros/links render with their own
// rules. The first row is the header.
type tableRule struct{}

func (tableRule) Name() string { return "table" }

func (tableRule) Match(sel *goquery.Selection) bool {
	return tagName(sel) == "table"
}

func (tableRule) Render(sel *goquery.Selection, ctx *RenderContext) (string, error) {
	rows := collectRows(sel, ctx)
	if len(rows) == 0 {
		return "", nil
	}

	ncol := 0
	for _, r := range rows {
		if len(r) > ncol {
			ncol = len(r)
		}
	}
	if ncol == 0 {
		return "", nil
	}

	pad := func(r []string) []string {
		for len(r) < ncol {
			r = append(r, "")
		}
		return r
	}
	row := func(cells []string) string {
		return "| " + strings.Join(pad(cells), " | ") + " |"
	}

	seps := make([]string, ncol)
	for i := range seps {
		seps[i] = "---"
	}

	var lines []string
	lines = append(lines, row(rows[0]), row(seps))
	for _, r := range rows[1:] {
		lines = append(lines, row(r))
	}
	return "\n\n" + strings.Join(lines, "\n") + "\n\n", nil
}

// collectRows gathers the table's direct rows (handling thead/tbody/tfoot
// wrappers) and renders each cell to a single-line, GFM-safe string. Nested
// tables inside a cell recurse through the registry and are flattened by
// sanitizeCell.
func collectRows(sel *goquery.Selection, ctx *RenderContext) [][]string {
	var rows [][]string
	addTr := func(tr *goquery.Selection) {
		var cells []string
		tr.Children().Each(func(_ int, c *goquery.Selection) {
			switch tagName(c) {
			case "td", "th":
				cells = append(cells, sanitizeCell(ctx.RenderChildren(c)))
			}
		})
		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	}
	sel.Children().Each(func(_ int, c *goquery.Selection) {
		switch tagName(c) {
		case "tr":
			addTr(c)
		case "thead", "tbody", "tfoot":
			c.Children().Each(func(_ int, r *goquery.Selection) {
				if tagName(r) == "tr" {
					addTr(r)
				}
			})
		}
	})
	return rows
}

// sanitizeCell flattens rendered cell Markdown to a single GFM-safe line:
// collapse whitespace/newlines to spaces and escape pipes.
func sanitizeCell(s string) string {
	s = whitespace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}
