# issues found by Claude when trying to run the MCP query

Here are the problems I hit, in order of impact:

---
1. confluence_get_page requires integer ConfluenceID, but confluence_search returns UUIDs

**Status: fixed** — see `DOCS/task-mcp-confluence-id.md`. Search and get_page both use `confluence_id` + `space_key`.

This was the biggest blocker. The search tool gives you an internal UUID (PageID), but confluence_get_page takes an integer (page_id). There's no way to go from a search result directly to the full page content — you have to separately discover the integer ID. These should use the same ID system, or confluence_get_page should also accept the UUID.

Fix: Accept the UUID from search results as page_id in confluence_get_page. Or add ConfluenceID to search results so you can bridge the two.

---
2. confluence_search excerpts are too short and always from the page start

**Status: fixed** — see `DOCS/task-mcp-search-excerpts.md`. Excerpts use `ts_headline` centered on query matches (~400 chars).

Every excerpt was ~200 chars and always showed the top of the page, regardless of what I queried. A BM25 search should return a snippet centered around the matching terms. This made it impossible to read different parts of a long page
through multiple queries.

Fix: Return excerpts centered on the matched text, not always the page beginning. 300–500 chars is mor

---
3. confluence_list_space has no pagination (no offset)

The space has hundreds of pages. With only a limit parameter and no offset, the only way to reach pages beyond position N is to request a huge limit, which produces a multi-megabyte result that overflows the context window.

Fix: Add an offset (or cursor) parameter. Even better: since the space is sorted by ConfluenceID, add after_id so you can paginate with after_id: 545917120.

---
4. confluence_list_space returns full page content for every result

Each entry in the list includes the entire Content field. For a 500-page space that's ~1.5MB just to g most of which I didn't need.

Fix: Add a fields parameter or a lightweight mode (titles_only: true) that returns just ID, Confluence without Content.

---
5. No way to fetch a page by title or by internal UUID directly

If I know the title ("Stampcard Filtering Logics") or the UUID from a search result, I still can't fetch the page. I'm forced to go through the integer ConfluenceID, which is the hardest thing to know.

Fix: Add confluence_get_page_by_title(space_key, title) or make confluence_get_page accept the UUID.

---
Summary priority

```
┌─────────────┬──────────────────────────────────────────────────────────────────────────────┐
│  Priority   │                                    Change                                    │
├─────────────┼──────────────────────────────────────────────────────────────────────────────┤
│ 🔴 Critical │ confluence_get_page should accept the UUID returned by confluence_search     │
├─────────────┼──────────────────────────────────────────────────────────────────────────────┤
│ 🔴 Critical │ Search excerpts should be centered on matched terms, not page start          │
├─────────────┼──────────────────────────────────────────────────────────────────────────────┤
│ 🟡 High     │ confluence_list_space needs offset/after_id pagination                       │
├─────────────┼──────────────────────────────────────────────────────────────────────────────┤
│ 🟡 High     │ confluence_list_space should have a titles-only mode (no Content by default) │
├─────────────┼──────────────────────────────────────────────────────────────────────────────┤
│ 🟢 Nice     │ confluence_get_page_by_title shortcut                                        │
└─────────────┴──────────────────────────────────────────────────────────────────────────────┘
```