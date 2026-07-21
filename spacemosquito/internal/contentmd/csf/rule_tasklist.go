package csf

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// tasklistRule renders ac:task-list as GFM checkboxes. Each ac:task contributes
// a "- [ ]"/"- [x]" line from its ac:task-status; the body comes from
// ac:task-body. Task id/uuid metadata is ignored. Nested task-lists inside a
// body are rendered via the normal walk.
type tasklistRule struct{}

func (tasklistRule) Name() string { return "tasklist" }

func (tasklistRule) Match(sel *goquery.Selection) bool {
	return tagName(sel) == "ac:task-list"
}

func (tasklistRule) Render(sel *goquery.Selection, ctx *RenderContext) (string, error) {
	var lines []string
	sel.Children().Each(func(_ int, task *goquery.Selection) {
		if tagName(task) != "ac:task" {
			return
		}
		status, body := "", ""
		task.Children().Each(func(_ int, c *goquery.Selection) {
			switch tagName(c) {
			case "ac:task-status":
				status = strings.TrimSpace(c.Text())
			case "ac:task-body":
				body = strings.TrimSpace(ctx.RenderChildren(c))
			}
		})
		mark := "[ ]"
		if status == "complete" {
			mark = "[x]"
		}
		line := "- " + mark
		if body != "" {
			line += " " + body
		}
		lines = append(lines, line)
	})
	if len(lines) == 0 {
		return "", nil
	}
	return "\n\n" + strings.Join(lines, "\n") + "\n\n", nil
}
