package csf

import (
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

// drawioRule renders drawio / inc-drawio macros. The macro carries only the
// diagram name (plus revision/macro-id); the rendered preview is a page
// attachment named "<diagramName>.png". The rule schedules that PNG for
// download into assets/diagrams/ and links it. The image alt text is the
// diagram name, so a missing PNG still shows the name (graceful degradation).
//
// inc-drawio embeds a diagram from another page, but we resolve the attachment
// against the host page's ID (ctx.PageID); cross-page includes may 404, in
// which case the alt text still shows the diagram name.
type drawioRule struct{}

func (drawioRule) Name() string { return "drawio" }

func (drawioRule) Match(sel *goquery.Selection) bool {
	if tagName(sel) != "ac:structured-macro" {
		return false
	}
	switch macroName(sel) {
	case "drawio", "inc-drawio":
		return true
	}
	return false
}

func (drawioRule) Render(sel *goquery.Selection, ctx *RenderContext) (string, error) {
	name := macroParam(sel, "diagramName")
	if name == "" {
		return "", nil
	}
	local := sanitizeAssetName(name + ".png")
	if u := drawioURL(ctx.BaseURL, ctx.PageID, name, ctx.Cloud); u != "" {
		ctx.RequestAsset(AssetRequest{Kind: "diagram", Filename: local, URL: u})
	}
	return "\n\n![" + name + "](assets/diagrams/" + url.PathEscape(local) + ")\n\n", nil
}

// drawioURL builds the rendered-preview download URL: the attachment endpoint
// for "<diagramName>.png" with the ?api=v2 query (verified on Server/DC).
func drawioURL(base string, pageID int, diagramName string, cloud bool) string {
	u := attachmentURL(base, pageID, diagramName+".png", cloud)
	if u == "" {
		return ""
	}
	return u + "?api=v2"
}
