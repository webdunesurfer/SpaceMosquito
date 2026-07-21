# CSF rules

Each rule converts one recognized CSF node into a Markdown chunk. Rules are
small, single-responsibility, and independently unit-testable. See the
[package README](../README.md) for how the registry drives them.

> **Layout note.** Rules live in the parent `csf` package as `rule_*.go` files,
> not in this directory — that keeps `DefaultRegistry()` able to reference them
> without an import cycle. This directory holds only this catalog doc.

## Rule contract

```go
// Rule converts one recognized CSF node into a Markdown chunk.
type Rule interface {
    // Name identifies the rule (telemetry + test labels).
    Name() string
    // Match reports whether this rule handles the node.
    Match(n *goquery.Selection) bool
    // Render produces Markdown. It may call ctx.RenderChildren(n) for nested
    // content and ctx.RequestAsset(...) to schedule an asset download.
    Render(n *goquery.Selection, ctx *RenderContext) (string, error)
}
```

## Rule file format

- **One rule per file**, named `rule_<construct>.go` (`rule_code.go`,
  `rule_panel.go`, `rule_image.go`, …), `package csf`.
- Each file defines an unexported `Rule` implementation; register it in
  `DefaultRegistry()` in `registry.go` (order matters — see below).
- `Match` should be cheap and specific (tag name + `ac:name` param, or native
  tag). Prefer matching on the CSF contract (`ac:name`), not on presentational
  attributes.
- `Render` must be **pure**: no network, no DB. For media, call
  `ctx.RequestAsset(...)` and return a Markdown link. For nested content, call
  `ctx.RenderChildren(n)` — never re-parse HTML by hand.
- Return an empty string (not an error) to intentionally drop a node
  (e.g. `toc`, `placeholder`).
- A matching `*_test.go` sits beside each rule: table-driven, CSF fragment in →
  Markdown out, **no corpus data** (synthetic/anonymized fixtures only).

## Registration order

The registry is **ordered, first-match-wins**. Register specific rules before
generic ones, and the fallback rule **last**:

1. Strip & structural (strip, layout)
2. Specific macros (code, image, drawio, jira, status, panel, …)
3. Native HTML passthrough (headings, p, lists, table, links, inline code)
4. **Fallback** (last) — any `structured-macro`: recurse remaining children
   (parameters already dropped by `strip`)

## Rules

Unmatched elements fall through to the renderer's built-in default — recurse
children, add block separation for block-level tags — so any construct without a
dedicated rule still renders readable text.

| Rule | Matches | Output |
|------|---------|--------|
| `strip` | `ac:placeholder`, `ac:parameter`, `ac:inline-comment-marker` | drop placeholder/parameter; unwrap marker (keep text) |
| `layout` | `ac:layout` / `layout-section` / `layout-cell` | transparent passthrough |
| `fallback` | any `ac:structured-macro` | recurse remaining children (params already stripped) |
| `html_passthrough` | `h1-6`, `p`, `strong`/`b`, `em`/`i`, `ul`/`ol`/`li`, `hr`, `br`, `code`, `a` | native Markdown (nested lists indented) |
| `link` | `ac:link` + `ri:page`/`ri:attachment`/`ri:user`/`ri:url`/`ri:space` | `[text](url)` for `ri:url`; label text otherwise (no broken links) |
| `table` | native `<table>` | hand-rolled GFM; cells via `ctx.RenderChildren`; pipes escaped, newlines flattened |
| `code` | `ac:name="code"`, native `<pre>` | fenced block + language from `ac:parameter`; body verbatim (CDATA unwrapped) |
| `image` | `ac:image` + `ri:attachment`/`ri:url` | `![](assets/images/…)` + asset request (attachment); direct link (`ri:url`) |
| `emoticon` | `ac:emoticon` | `ac:emoji-fallback` glyph, else Unicode map (✓ ✗ ⚠ ℹ ❓), drop unknown |
| `drawio` | `ac:name="drawio"` / `inc-drawio` | `![name](assets/diagrams/name.png)` + asset request (`<name>.png?api=v2`); name is alt text |
| `jira` | `ac:name="jira"` | `**JIRA:** KEY` (minimal; no live fetch); key-less dropped |
| `status` | `ac:name="status"` | `` `title` `` inline code; colour dropped |
| `panel` | `ac:name` = `panel`/`info`/`note`/`warning`/`tip` | `info`/`note`/`warning`/`tip` → blockquote callout; generic `panel` → transparent container (bold title + body, no quote) |
| `expand` | `ac:name` = `expand`/`details` | bold title + body |
| `toc` | `ac:name="toc"` | drop |
| `tasklist` | `ac:task-list` / `ac:task` | `- [ ]` / `- [x]` from `ac:task-status` |

> Keep this table in sync as rules are added or changed.
