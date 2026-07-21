package csf

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var (
	// whitespace collapses runs of whitespace within a text node to a single
	// space (HTML text semantics; <pre> handling comes in Phase 2).
	whitespace = regexp.MustCompile(`\s+`)
	// multiNewline collapses 3+ blank lines to a single blank line.
	multiNewline = regexp.MustCompile(`\n{3,}`)
)

// blockTags are native/structural elements that, absent a specific rule,
// render as their inner content surrounded by blank lines so block boundaries
// (and word boundaries) are preserved. Rich Markdown markup (#, -, **, GFM
// tables) is added by the html_passthrough rule in Phase 1.
var blockTags = map[string]bool{
	"p": true, "div": true, "section": true, "article": true,
	"header": true, "footer": true, "aside": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "li": true,
	"table": true, "thead": true, "tbody": true, "tr": true, "caption": true,
	"blockquote": true, "hr": true, "pre": true,
	"dl": true, "dt": true, "dd": true,
	"ac:rich-text-body": true, "ac:plain-text-body": true,
}

// renderChildren walks the child nodes of sel in document order, dispatching
// element nodes through the registry and emitting text nodes directly.
func renderChildren(sel *goquery.Selection, ctx *RenderContext) string {
	var sb strings.Builder
	sel.Contents().Each(func(_ int, s *goquery.Selection) {
		n := s.Get(0)
		if n == nil {
			return
		}
		switch n.Type {
		case html.TextNode:
			sb.WriteString(whitespace.ReplaceAllString(n.Data, " "))
		case html.ElementNode:
			if rule, ok := ctx.registry.Match(s); ok {
				// Phase 0: swallow rule errors and continue; the epic's
				// robustness requirement is "must not panic".
				if md, err := rule.Render(s, ctx); err == nil {
					sb.WriteString(md)
				}
			} else {
				sb.WriteString(defaultRender(s, ctx))
			}
		}
		// Comment/Doctype/CDATA-as-comment nodes are skipped.
	})
	return sb.String()
}

// renderElementChildren dispatches only the element children of sel through
// the registry, skipping text nodes. Used to preserve content that HTML
// parsing may have mis-nested inside a dropped element (see stripRule).
func renderElementChildren(sel *goquery.Selection, ctx *RenderContext) string {
	var sb strings.Builder
	sel.Contents().Each(func(_ int, s *goquery.Selection) {
		n := s.Get(0)
		if n == nil || n.Type != html.ElementNode {
			return
		}
		if rule, ok := ctx.registry.Match(s); ok {
			if md, err := rule.Render(s, ctx); err == nil {
				sb.WriteString(md)
			}
		} else {
			sb.WriteString(defaultRender(s, ctx))
		}
	})
	return sb.String()
}

// defaultRender handles element nodes with no matching rule: recurse children,
// adding block separation for block-level tags so content does not run together.
func defaultRender(sel *goquery.Selection, ctx *RenderContext) string {
	name := tagName(sel)
	inner := renderChildren(sel, ctx)
	switch name {
	case "br":
		return "\n"
	case "td", "th":
		return strings.TrimSpace(inner) + " "
	default:
		if blockTags[name] {
			if inner = strings.TrimSpace(inner); inner == "" {
				return ""
			}
			return "\n\n" + inner + "\n\n"
		}
		return inner
	}
}

// normalize trims trailing whitespace per line (leading whitespace is
// significant — it carries list indentation), collapses excess blank lines,
// and trims the whole string. It applies no length cap (see Epic OQ#6).
func normalize(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	s = strings.Join(lines, "\n")
	s = multiNewline.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
