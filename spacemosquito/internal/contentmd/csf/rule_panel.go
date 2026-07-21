package csf

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// panelRule renders panel/info/note/warning/tip macros. The body comes from
// ac:rich-text-body (via the walk); border/style params are dropped.
//
// info/note/warning/tip are admonitions → blockquote callouts with a type
// label. A generic `panel`, by contrast, is just a visual box and is often used
// to wrap large sections or whole pages, so it renders as a transparent
// container (optional bold title + body, no blockquote) to avoid quoting an
// entire document.
type panelRule struct{}

func (panelRule) Name() string { return "panel" }

// admonitionLabels are the macros rendered as blockquote callouts.
var admonitionLabels = map[string]string{
	"info":    "Info",
	"note":    "Note",
	"warning": "Warning",
	"tip":     "Tip",
}

func (panelRule) Match(sel *goquery.Selection) bool {
	if tagName(sel) != "ac:structured-macro" {
		return false
	}
	name := macroName(sel)
	if name == "panel" {
		return true
	}
	_, ok := admonitionLabels[name]
	return ok
}

func (panelRule) Render(sel *goquery.Selection, ctx *RenderContext) (string, error) {
	name := macroName(sel)
	body := strings.TrimSpace(ctx.RenderChildren(sel))

	if name == "panel" {
		// Transparent container: optional title as a bold line, body as-is.
		title := macroParam(sel, "title")
		var out string
		if title != "" {
			out = "**" + title + "**\n\n"
		}
		out += body
		if strings.TrimSpace(out) == "" {
			return "", nil
		}
		return "\n\n" + out + "\n\n", nil
	}

	return calloutBlock(admonitionLabels[name], body), nil
}

// calloutBlock renders an optional bold label and body as a Markdown
// blockquote. Returns "" if both are empty.
func calloutBlock(label, body string) string {
	body = strings.TrimSpace(body)
	var content string
	switch {
	case label != "" && body != "":
		content = "**" + label + ":**\n\n" + body
	case label != "":
		content = "**" + label + ":**"
	default:
		content = body
	}
	if content == "" {
		return ""
	}
	// Collapse runs of blank lines so the blockquote doesn't accumulate empty
	// "> " lines between blocks.
	content = multiNewline.ReplaceAllString(content, "\n\n")
	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if line == "" {
			b.WriteString(">\n")
		} else {
			b.WriteString("> " + line + "\n")
		}
	}
	return "\n\n" + strings.TrimRight(b.String(), "\n") + "\n\n"
}
