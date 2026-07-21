package csf

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// imageRule renders ac:image. An ri:attachment child is a page attachment: the
// rule schedules a download and links the local copy under assets/images/. An
// ri:url child is an external image linked directly (no download).
type imageRule struct{}

func (imageRule) Name() string { return "image" }

func (imageRule) Match(sel *goquery.Selection) bool {
	return tagName(sel) == "ac:image"
}

func (imageRule) Render(sel *goquery.Selection, ctx *RenderContext) (string, error) {
	alt := strings.TrimSpace(sel.AttrOr("ac:alt", ""))

	// External image: <ri:url ri:value="https://…"/>
	if u := findDescendant(sel, "ri:url"); u != nil {
		if v := strings.TrimSpace(u.AttrOr("ri:value", "")); v != "" {
			return fmt.Sprintf("![%s](%s)", alt, v), nil
		}
	}

	// Attachment: <ri:attachment ri:filename="…"/>
	att := findDescendant(sel, "ri:attachment")
	if att == nil {
		return "", nil
	}
	filename := strings.TrimSpace(att.AttrOr("ri:filename", ""))
	if filename == "" {
		return "", nil
	}
	local := sanitizeAssetName(filename)

	downloadURL := attachmentURL(ctx.BaseURL, ctx.PageID, filename, ctx.Cloud)
	if downloadURL != "" {
		ctx.RequestAsset(AssetRequest{Kind: "image", Filename: local, URL: downloadURL})
	}
	return fmt.Sprintf("![%s](assets/images/%s)", alt, url.PathEscape(local)), nil
}

// attachmentURL builds the download URL for a page attachment, flavor-aware.
// Returns "" if base/pageID/filename are missing.
//
// The Server/DC shape (/download/attachments/...) is verified against a live
// instance. The Cloud shape (/wiki/download/attachments/...) is ASSUMED and not
// yet verified on a live Cloud instance — revisit if Cloud downloads 404.
func attachmentURL(base string, pageID int, filename string, cloud bool) string {
	if base == "" || pageID == 0 || filename == "" {
		return ""
	}
	prefix := "/download/attachments/"
	if cloud {
		prefix = "/wiki/download/attachments/"
	}
	return fmt.Sprintf("%s%s%d/%s", base, prefix, pageID, url.PathEscape(filename))
}

// sanitizeAssetName makes an attachment filename safe as a single path segment,
// preserving spaces and the extension (path separators become underscores).
func sanitizeAssetName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return strings.TrimSpace(name)
}
