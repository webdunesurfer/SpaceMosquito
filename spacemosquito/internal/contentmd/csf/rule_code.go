package csf

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// codeRule renders code as a fenced Markdown block:
//   - the `code` macro → fence with language from ac:parameter[language] and
//     body from ac:plain-text-body (CDATA already unwrapped by the converter);
//   - a native <pre> block → bare fence.
//
// Code text is read verbatim via .Text() (not through the whitespace-collapsing
// walk) so indentation and newlines are preserved.
type codeRule struct{}

func (codeRule) Name() string { return "code" }

func (codeRule) Match(sel *goquery.Selection) bool {
	switch tagName(sel) {
	case "pre":
		return true
	case "ac:structured-macro":
		return macroName(sel) == "code"
	}
	return false
}

func (codeRule) Render(sel *goquery.Selection, ctx *RenderContext) (string, error) {
	if tagName(sel) == "pre" {
		return fence("", sel.Text()), nil
	}
	lang := macroParam(sel, "language")
	body := ""
	if b := findDescendant(sel, "ac:plain-text-body"); b != nil {
		body = b.Text()
	}
	return fence(lang, body), nil
}

// fence builds a fenced code block, lengthening the fence if the body contains
// a backtick run that would otherwise close it early. Leading/trailing blank
// lines are trimmed; interior indentation is preserved.
func fence(lang, body string) string {
	body = strings.Trim(body, "\n")
	if strings.TrimSpace(body) == "" {
		return ""
	}
	ticks := "```"
	for strings.Contains(body, ticks) {
		ticks += "`"
	}
	return "\n\n" + ticks + lang + "\n" + body + "\n" + ticks + "\n\n"
}

// macroName returns the ac:name of a structured-macro node.
func macroName(sel *goquery.Selection) string {
	return sel.AttrOr("ac:name", "")
}

// macroParam returns the trimmed text of the macro's ac:parameter with the
// given ac:name, or "".
func macroParam(sel *goquery.Selection, name string) string {
	var val string
	sel.Find("*").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if tagName(s) == "ac:parameter" && s.AttrOr("ac:name", "") == name {
			val = strings.TrimSpace(s.Text())
			return false
		}
		return true
	})
	return val
}
