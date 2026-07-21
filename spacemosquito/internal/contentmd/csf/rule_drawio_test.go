package csf

import (
	"strings"
	"testing"
)

func drawioMacro(name, diagram string) string {
	return `<ac:structured-macro ac:name="` + name + `">` +
		`<ac:parameter ac:name="diagramName">` + diagram + `</ac:parameter>` +
		`<ac:parameter ac:name="revision">3</ac:parameter>` +
		`</ac:structured-macro>`
}

func TestDrawio_pngAssetAndLink(t *testing.T) {
	ctx := &RenderContext{PageID: 542576204, BaseURL: "https://wiki.example.net"}
	got, assets := convert(t, drawioMacro("drawio", "Component Overview"), ctx)

	if !strings.Contains(got, "![Component Overview](assets/diagrams/Component%20Overview.png)") {
		t.Fatalf("diagram link wrong: %q", got)
	}
	if len(assets) != 1 {
		t.Fatalf("want 1 diagram asset, got %d", len(assets))
	}
	a := assets[0]
	if a.Kind != "diagram" {
		t.Fatalf("kind should be diagram: %+v", a)
	}
	if a.Filename != "Component Overview.png" {
		t.Fatalf("local filename wrong: %q", a.Filename)
	}
	want := "https://wiki.example.net/download/attachments/542576204/Component%20Overview.png?api=v2"
	if a.URL != want {
		t.Fatalf("download URL wrong:\n got %q\nwant %q", a.URL, want)
	}
}

func TestDrawio_incDrawioSameScheme(t *testing.T) {
	ctx := &RenderContext{PageID: 10, BaseURL: "https://wiki.example.net"}
	_, assets := convert(t, drawioMacro("inc-drawio", "Flow"), ctx)
	if len(assets) != 1 || assets[0].Kind != "diagram" || assets[0].Filename != "Flow.png" {
		t.Fatalf("inc-drawio not handled: %+v", assets)
	}
}

func TestDrawio_cloudURLShape(t *testing.T) {
	ctx := &RenderContext{PageID: 7, BaseURL: "https://x.atlassian.net", Cloud: true}
	_, assets := convert(t, drawioMacro("drawio", "D"), ctx)
	want := "https://x.atlassian.net/wiki/download/attachments/7/D.png?api=v2"
	if len(assets) != 1 || assets[0].URL != want {
		t.Fatalf("cloud drawio URL wrong: %+v", assets)
	}
}

func TestDrawio_altTextDegradesGracefully(t *testing.T) {
	// Even with no base/pageID (no download scheduled), the diagram name is the
	// alt text so a missing image still shows the name.
	got, assets := convert(t, drawioMacro("drawio", "My Diagram"), &RenderContext{})
	if !strings.Contains(got, "![My Diagram](assets/diagrams/My%20Diagram.png)") {
		t.Fatalf("alt/link wrong: %q", got)
	}
	if len(assets) != 0 {
		t.Fatalf("no download without base/pageID, got %d", len(assets))
	}
}

func TestDrawio_noDiagramNameDropped(t *testing.T) {
	got, assets := convert(t, `<ac:structured-macro ac:name="drawio"><ac:parameter ac:name="revision">1</ac:parameter></ac:structured-macro>`, &RenderContext{PageID: 1, BaseURL: "https://e.net"})
	if strings.Contains(got, "assets/diagrams") {
		t.Fatalf("should emit nothing without diagramName: %q", got)
	}
	if len(assets) != 0 {
		t.Fatalf("no asset without diagramName, got %d", len(assets))
	}
}
