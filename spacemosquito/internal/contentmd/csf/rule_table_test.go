package csf

import (
	"strings"
	"testing"
)

func TestTable_basicGFM(t *testing.T) {
	got := md(t, `<table>
		<tr><th>A</th><th>B</th></tr>
		<tr><td>1</td><td>2</td></tr>
	</table>`)
	if !strings.Contains(got, "| A | B |") {
		t.Fatalf("header row missing: %q", got)
	}
	if !strings.Contains(got, "| --- | --- |") {
		t.Fatalf("separator row missing: %q", got)
	}
	if !strings.Contains(got, "| 1 | 2 |") {
		t.Fatalf("body row missing: %q", got)
	}
}

func TestTable_theadTbody(t *testing.T) {
	got := md(t, `<table>
		<thead><tr><th>H1</th><th>H2</th></tr></thead>
		<tbody><tr><td>a</td><td>b</td></tr></tbody>
	</table>`)
	if !strings.Contains(got, "| H1 | H2 |") || !strings.Contains(got, "| a | b |") {
		t.Fatalf("thead/tbody not handled: %q", got)
	}
}

func TestTable_escapesPipeAndFlattensNewlines(t *testing.T) {
	got := md(t, `<table><tr><th>col</th></tr><tr><td>a | b
	c</td></tr></table>`)
	if !strings.Contains(got, `a \| b`) {
		t.Fatalf("pipe not escaped: %q", got)
	}
	// The cell must remain a single logical line (no raw newline inside it).
	for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
		if strings.HasPrefix(line, "|") && strings.Contains(line, "a ") && !strings.Contains(line, "c") {
			t.Fatalf("cell newline not flattened: %q", got)
		}
	}
}

func TestTable_nestedMacroInCellRoutesThroughRegistry(t *testing.T) {
	// A native link inside a cell should render via the html rule.
	got := md(t, `<table><tr><th>h</th></tr><tr><td><a href="https://e.com">L</a></td></tr></table>`)
	if !strings.Contains(got, "[L](https://e.com)") {
		t.Fatalf("cell content not routed through registry: %q", got)
	}
}
