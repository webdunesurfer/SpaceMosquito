package csf

import (
	"strings"
	"testing"
)

func TestCode_macroWithLanguage(t *testing.T) {
	in := `<ac:structured-macro ac:name="code">
		<ac:parameter ac:name="language">json</ac:parameter>
		<ac:plain-text-body><![CDATA[{"a": 1, "b": [2, 3]}
if (x > y) { return; }]]></ac:plain-text-body>
	</ac:structured-macro>`
	got := md(t, in)

	if !strings.Contains(got, "```json") {
		t.Fatalf("language fence missing: %q", got)
	}
	// Body must survive verbatim, including chars the HTML parser would mangle.
	if !strings.Contains(got, `{"a": 1, "b": [2, 3]}`) {
		t.Fatalf("json body lost: %q", got)
	}
	if !strings.Contains(got, "if (x > y) { return; }") {
		t.Fatalf("code with '>' truncated: %q", got)
	}
	if strings.Contains(got, "CDATA") {
		t.Fatalf("CDATA marker leaked: %q", got)
	}
}

func TestCode_macroNoLanguage(t *testing.T) {
	in := `<ac:structured-macro ac:name="code"><ac:plain-text-body><![CDATA[plain code]]></ac:plain-text-body></ac:structured-macro>`
	got := md(t, in)
	if !strings.Contains(got, "```\nplain code\n```") {
		t.Fatalf("bare fence expected: %q", got)
	}
}

func TestCode_preservesIndentation(t *testing.T) {
	in := "<ac:structured-macro ac:name=\"code\"><ac:parameter ac:name=\"language\">go</ac:parameter>" +
		"<ac:plain-text-body><![CDATA[func main() {\n\tx := 1\n\treturn x\n}]]></ac:plain-text-body></ac:structured-macro>"
	got := md(t, in)
	if !strings.Contains(got, "\n\tx := 1\n") {
		t.Fatalf("indentation not preserved: %q", got)
	}
}

func TestCode_nativePre(t *testing.T) {
	in := "<pre>line one\nline two</pre>"
	got := md(t, in)
	if !strings.Contains(got, "```\nline one\nline two\n```") {
		t.Fatalf("pre not fenced: %q", got)
	}
}

func TestCode_fenceLengthenedForBacktickBody(t *testing.T) {
	in := "<ac:structured-macro ac:name=\"code\"><ac:plain-text-body><![CDATA[a ``` b]]></ac:plain-text-body></ac:structured-macro>"
	got := md(t, in)
	if !strings.Contains(got, "````") {
		t.Fatalf("fence not lengthened for backtick body: %q", got)
	}
}

func TestCode_languageParamNotLeakedAsText(t *testing.T) {
	// The language value must appear only in the fence info string, not as body.
	in := `<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">python</ac:parameter><ac:plain-text-body><![CDATA[print(1)]]></ac:plain-text-body></ac:structured-macro>`
	got := md(t, in)
	if strings.Count(got, "python") != 1 {
		t.Fatalf("language should appear exactly once (in fence): %q", got)
	}
}
