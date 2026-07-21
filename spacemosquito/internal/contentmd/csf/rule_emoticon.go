package csf

import "github.com/PuerkitoBio/goquery"

// emoticonRule renders ac:emoticon as Unicode. Modern Confluence carries the
// glyph in ac:emoji-fallback; legacy emoticons only have ac:name, mapped via a
// static table. Unknown emoticons are dropped.
type emoticonRule struct{}

func (emoticonRule) Name() string { return "emoticon" }

func (emoticonRule) Match(sel *goquery.Selection) bool {
	return tagName(sel) == "ac:emoticon"
}

func (emoticonRule) Render(sel *goquery.Selection, ctx *RenderContext) (string, error) {
	glyph := ""
	if fb := sel.AttrOr("ac:emoji-fallback", ""); fb != "" {
		glyph = fb
	} else if u, ok := emoticonUnicode[sel.AttrOr("ac:name", "")]; ok {
		glyph = u
	}
	// ac:emoticon is empty, but its self-closing form makes the HTML parser
	// mis-nest following siblings inside it; render them so nothing is lost.
	return glyph + ctx.RenderChildren(sel), nil
}

// emoticonUnicode maps legacy Confluence emoticon names to Unicode glyphs.
var emoticonUnicode = map[string]string{
	"tick":        "✓",
	"cross":       "✗",
	"warning":     "⚠",
	"information": "ℹ",
	"question":    "❓",
	"plus":        "＋",
	"minus":       "－",
	"check":       "✓",
	"error":       "✗",
	"smile":       "🙂",
	"sad":         "🙁",
	"cheeky":      "😛",
	"laugh":       "😂",
	"wink":        "😉",
	"thumbs-up":   "👍",
	"thumbs-down": "👎",
	"heart":       "❤",
	"broken-heart": "💔",
	"star":         "⭐",
	"yellow-star":  "⭐",
	"red-star":     "⭐",
	"green-star":   "⭐",
	"blue-star":    "⭐",
}
