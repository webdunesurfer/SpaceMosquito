package csf

import "github.com/PuerkitoBio/goquery"

// jiraRule renders the jira macro minimally: the issue key as a bold ref. We
// cannot resolve the JIRA base URL from the macro (it carries a server display
// name / UUID, not a URL), so no link is emitted. Key-less jira macros (JQL
// tables) render nothing. server/columns params are dropped.
type jiraRule struct{}

func (jiraRule) Name() string { return "jira" }

func (jiraRule) Match(sel *goquery.Selection) bool {
	return tagName(sel) == "ac:structured-macro" && macroName(sel) == "jira"
}

func (jiraRule) Render(sel *goquery.Selection, _ *RenderContext) (string, error) {
	key := macroParam(sel, "key")
	if key == "" {
		return "", nil
	}
	return "**JIRA:** " + key, nil
}
