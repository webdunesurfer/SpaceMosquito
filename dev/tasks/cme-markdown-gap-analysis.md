# Gap analysis — CME Markdown conversion vs CSF converter

- **Task ID:** `cme-markdown-gap-analysis`
- **Status:** ready

## Goal

Extract the behavioral transformation rules from
[Spenhouet/confluence-markdown-exporter](https://github.com/Spenhouet/confluence-markdown-exporter)
(CME, MIT) and compare them to SpaceMosquito’s CSF→Markdown pipeline. Produce a
**gap matrix** and prioritized recommendations — do **not** port rules in this
task.

## Why

CME is a mature, well-tested Confluence→Markdown exporter. Its unit tests are
mostly HTML-in / Markdown-out fixtures. Mining those (plus the converter code)
is cheaper than rediscovering edge cases ourselves, and clarifies where our
converter is weaker, equivalent, or intentionally different.

## Important mismatch (do not ignore)

| | CME | SpaceMosquito |
|---|-----|----------------|
| Input | Rendered **HTML** (`body` / export view) via BeautifulSoup + `markdownify` | **CSF / storage XML** via `internal/contentmd/csf` |
| Rule shape | Imperative `Page.Converter.convert_*` overrides | Explicit `rule_*.go` registry |
| Scope | Also: attachments layout, Obsidian presets, Jira API enrichment, lockfile skip | Crawl + search/MCP; attachments handled elsewhere |

A CME “rule” is **not** drop-in for CSF. Gap analysis must map **intent**
(e.g. “info panel → GitHub alert”) to our CSF handlers, and note when CME
behavior depends on HTML-only markup we never see in storage.

## Scope

### In scope

1. **Inventory CME rules** from:
   - Unit tests under `tests/unit/` (primary): `test_confluence.py`,
     `test_alert_conversion.py`, `test_plantuml_*.py`, `test_include_macro_*.py`,
     `test_emoticon_conversion.py`, `test_nbsp_fix.py`, etc.
   - `Page.Converter` / `macro_handlers` in
     [`confluence.py`](https://github.com/Spenhouet/confluence-markdown-exporter/blob/main/confluence_markdown_exporter/confluence.py)
   - Supporting utils (tables, draw.io) only where they affect Markdown body
2. **Inventory our rules** from `spacemosquito/internal/contentmd/csf/`
   (`registry.go`, `rule_*.go`, existing `*_test.go` fixtures)
3. **Gap matrix** (spreadsheet or markdown table in the task notes / a short
   `dev/notes/` doc — keep it in-repo):

   | Capability | CME | Ours | Severity | Notes |
   |------------|-----|------|----------|-------|
   | … | … | … | high/med/low/n/a | HTML-only? CSF map? |

4. **Recommendations**: top gaps worth follow-up implement tasks vs ignore
   (preset-specific, out of product scope, HTML-only, quality already fine)

### Out of scope

- Porting CME code or rewriting the converter
- Calling CME as a subprocess / dependency
- Changing scrape to fetch HTML-only for conversion
- Full fixture import into our test suite (may propose a follow-up)

## Method (suggested)

1. Pin a CME commit / tag (e.g. `5.3.0`) for reproducibility.
2. Skim `Page.Converter` for `convert_*` and macro name → handler map; list
   supported macros and element overrides.
3. Parse or manually sample unit tests into a corpus of
   `(case_id, html_or_snippet, expected_md_asserts, category)`.
4. For each category, find the matching CSF rule (or mark **missing** /
   **partial** / **n/a-html**).
5. Spot-check 5–10 high-value cases by running the same *intent* through our
   converter with equivalent CSF fixtures (hand-written), not by feeding CME
   HTML into CSF.
6. Write the matrix + ranked follow-ups.

## Done when

- [ ] CME rule/capability inventory exists (macros + notable HTML overrides)
- [ ] Ours inventory cross-linked to `rule_*.go`
- [ ] Gap matrix with severity and HTML-vs-CSF notes
- [ ] Short recommendation list (≤10 items): implement / defer / ignore, with
      rationale
- [ ] Optional: draft follow-up task IDs for any “implement” items (stubs only)

## Related

- Ours: `spacemosquito/internal/contentmd/csf/`
- CME: https://github.com/Spenhouet/confluence-markdown-exporter (MIT — attribute
  if later porting ideas/fixtures)
- Prior research: CME uses client-side HTML→MD; no native Atlassian Markdown API
