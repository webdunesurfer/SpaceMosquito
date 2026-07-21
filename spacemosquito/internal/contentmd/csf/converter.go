package csf

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// cdataRe matches a CDATA section (non-greedy, dot matches newline).
var cdataRe = regexp.MustCompile(`(?s)<!\[CDATA\[(.*?)\]\]>`)

// unwrapCDATA replaces each <![CDATA[…]]> section with the HTML-escaped text of
// its contents. Confluence uses CDATA for code bodies (ac:plain-text-body) and
// some link labels, but the HTML5 parser treats CDATA as a bogus comment and
// truncates it at the first '>'. Escaping the contents into a plain text node
// lets goquery's .Text() recover the original verbatim.
func unwrapCDATA(s string) string {
	return cdataRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := cdataRe.FindStringSubmatch(m)
		return html.EscapeString(sub[1])
	})
}

// CSFToMarkdown converts a Confluence Storage Format document into Markdown.
// It returns the Markdown, any asset requests scheduled by rules (empty until
// Phase 3), and an error only on unparseable input.
//
// ctx may be nil; a default RenderContext with the default registry is used.
// If ctx.registry is unset, the default registry is installed.
func CSFToMarkdown(storageXML string, ctx *RenderContext) (string, []AssetRequest, error) {
	if ctx == nil {
		ctx = &RenderContext{}
	}
	if ctx.registry == nil {
		ctx.registry = DefaultRegistry()
	}

	storageXML = strings.TrimSpace(storageXML)
	if storageXML == "" {
		return "", nil, nil
	}
	storageXML = unwrapCDATA(storageXML)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(storageXML))
	if err != nil {
		return "", nil, fmt.Errorf("parse csf: %w", err)
	}

	md := renderChildren(doc.Find("body"), ctx)
	return normalize(md), ctx.assets, nil
}
