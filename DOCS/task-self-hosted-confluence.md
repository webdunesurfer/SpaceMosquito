# Task: Self-Hosted Confluence Support (Custom Domains)

## Objective

Make SpaceMosquito work end-to-end with **self-hosted Confluence** (Server / Data Center) and **custom-domain** installs (e.g. `https://wiki.mycompany.com`), not only `*.atlassian.net` Cloud tenants.

Today the **Go backend** already has partial support (Server API paths, `/display/` and `/spaces/` URL parsing, flavor probing). The **Firefox extension** and several **hardcoded URLs** block real-world use. The **Chrome extension** is closer but still incomplete (space detector, shared logic drift).

**Do not require Docker or live Confluence for unit tests** — use shared URL helpers + fixtures.

---

## Problem Summary

| Area | Current behavior | Impact on `wiki.mycompany.com` |
|------|------------------|--------------------------------|
| Firefox `background.ts` | Rejects URLs without `atlassian.net` | Session capture fails |
| Firefox `lib/session.ts` | `cookies.getAll({ domain: '.atlassian.net' })` | No cookies captured |
| Firefox `manifest.json` | `host_permissions` + CSP limited to `*.atlassian.net` | Extension cannot access custom domain |
| Firefox `lib/auth.ts` | Hardcoded `teamnetconomy.atlassian.net` | Wrong auth pre-check |
| Both `space-detector.ts` | Only `/wiki/spaces/KEY` regex | Server `/display/KEY` not detected in content script |
| Chrome `background.ts` | Better URL/cookie logic | OK for capture; still diverges from Firefox |
| `cron/scheduler.go` | Hardcoded `teamnetconomy.atlassian.net` page URLs | Incremental cron broken off Cloud |
| `session` / `scraper` root URL | `scheme://host` only — no **context path** | Breaks `/confluence/` installs |
| `scraper.go` auto-create space | Fallback `https://example.atlassian.net/wiki/spaces/{key}` | Wrong URL if space auto-created |

---

## Target Behavior

A user on `https://wiki.mycompany.com/display/PROJ` (or `/spaces/PROJ`, or Cloud-style `/wiki/spaces/PROJ` on a custom domain) can:

1. Open the extension popup on that tab
2. **Capture session** — cookies read from `wiki.mycompany.com`
3. **Validate session** — backend probes Server endpoints, sets `flavor: server`
4. **Crawl space** — discovery + scrape via `/rest/api/content...`
5. **Search / MCP** — unchanged

Optional (see Open Questions): Confluence behind a **context path** (`/confluence/`).

---

## Implementation Phases

### Phase 1 — Shared URL & Confluence detection (extensions)

Extract duplicated logic into a shared module used by **both** extensions (copy via build or `shared/` folder — match existing extension build setup).

**New shared helpers** (TypeScript):

| Function | Responsibility |
|----------|----------------|
| `isConfluenceUrl(url)` | True if URL matches known Confluence path patterns |
| `parseSpaceFromUrl(url)` | Returns `{ spaceKey, spaceURL, flavorHint }` |
| `getConfluenceOrigin(url)` | `scheme://host` (+ optional context path — Phase 4) |

**`isConfluenceUrl` patterns** (minimum):

- `*.atlassian.net` (Cloud)
- `/wiki/spaces/`
- `/spaces/` (custom domain Cloud or DC)
- `/display/` (Server/DC classic)

**`parseSpaceFromUrl` rules** (align with Go `GetSpaceKeyFromURL`):

| Pattern | Example | `spaceKey` | Suggested `spaceURL` |
|---------|---------|------------|----------------------|
| `/wiki/spaces/KEY` | Cloud / custom Cloud | `KEY` | `{origin}/wiki/spaces/KEY/overview` |
| `/spaces/KEY` | Custom domain | `KEY` | `{origin}/spaces/KEY/overview` |
| `/display/KEY` | Server/DC | `KEY` | `{origin}/display/KEY` |

Port Chrome `background.ts` `get-space-info` logic as the canonical implementation; replace Firefox's `atlassian.net` checks.

**Files to update**

| File | Change |
|------|--------|
| `firefox-extension/background.ts` | Use `isConfluenceUrl`; port Chrome `get-space-info` logic |
| `chrome-extension/background.ts` | Import shared helpers (dedupe inline logic) |
| `firefox-extension/content/space-detector.ts` | Use shared `parseSpaceFromUrl` |
| `chrome-extension/content/space-detector.ts` | Same |
| `firefox-extension/popup/popup.html` | Neutral placeholder URL |
| `chrome-extension/popup/popup.html` | Same |

---

### Phase 2 — Session cookie capture (extensions)

**Firefox `lib/session.ts`** — match Chrome behavior:

- Change `captureCookies(tabUrl)` signature to accept tab URL
- `cookies.getAll({ domain: hostname })` for the active Confluence host
- Also collect cookies on parent domain if needed (e.g. `.mycompany.com` for subdomain SSO) — see edge cases

**Cookie name filter** — extend for Server/DC (not only Atlassian Cloud names):

| Pattern | Typical Server/DC cookies |
|---------|---------------------------|
| Existing | `session`, `token`, `sso`, `atlassian`, `aui` |
| Add | `jsessionid`, `seraph`, `remember`, `crowd`, `oauth` |

Keep filter permissive enough for enterprise SSO; document that over-capture is trimmed by backend validation.

**Firefox `lib/auth.ts`**

- Remove hardcoded `teamnetconomy.atlassian.net`
- Use last captured `confluence_url` from `browser.storage.local` or active tab origin
- Or delegate auth check to backend `GET /api/session/status` / `POST /api/session/validate` (preferred — single source of truth)

**Files**

| File | Change |
|------|--------|
| `firefox-extension/lib/session.ts` | Host-based cookie capture |
| `firefox-extension/lib/auth.ts` | Remove tenant hardcode; use storage/backend |
| `chrome-extension/lib/session.ts` | Share cookie filter list with Firefox |
| `firefox-extension/lib/session.ts` | Update `captureAndSave` call sites to pass `tabUrl` |

---

### Phase 3 — Manifest & permissions (Firefox)

**`firefox-extension/manifest.json`**

| Field | Current | Target |
|-------|---------|--------|
| `host_permissions` | `https://*.atlassian.net/*` | `<all_urls>` **or** `https://*/*` + `http://*/*` (match Chrome policy) |
| `content_security_policy.connect-src` | `https://*.atlassian.net` | Remove Atlassian-only restriction; keep `localhost:8081` |

**Chrome** — already has `<all_urls>`; verify CSP `connect-src` allows backend only (no change required unless testing on `http://` Confluence).

**Security note:** Document that broad host permissions are required because Confluence URLs are customer-specific. Optional future: user-configured allowlist in extension settings.

---

### Phase 4 — Backend URL base & context path (Go)

**Problem:** `extractConfluenceRoot` / `extractConfluenceBaseURL` return only `scheme://host`. Server at `https://wiki.mycompany.com/confluence/` needs API base `https://wiki.mycompany.com/confluence`.

**Approach**

1. Add `extractConfluenceBaseURL(url)` enhancement in one place (shared between `session` and `scraper` — today duplicated).
2. Detect context path from URL path segments before `/display/`, `/spaces/`, `/wiki/`.
3. Optional config override:

```yaml
confluence:
  base_path: /confluence   # optional; auto-detect when empty
```

**Files**

| File | Change |
|------|--------|
| `internal/session/session.go` | `extractConfluenceRoot` → include context path; used by validation probes |
| `internal/scraper/page.go` | `extractConfluenceBaseURL` — delegate to shared helper or same logic |
| `internal/scraper/discovery.go` | API URLs use full base |
| `internal/scraper/scraper.go` | `ScrapePageAPI` uses full base |
| `internal/config/config.go` | Optional `confluence.base_path` |
| `internal/scraper/scraper.go` | Auto-create space fallback: derive from `spaceURL` arg, not `example.atlassian.net` |

**Edge cases**

- `https://wiki.company.com/confluence/display/PROJ` → base `https://wiki.company.com/confluence`
- `https://wiki.company.com/display/PROJ` → base `https://wiki.company.com`
- Cloud custom domain with `/wiki/` → Cloud probes still use `/wiki/rest/api/...` on same host

---

### Phase 5 — Cron scheduler hardcoded URL

**File:** `internal/cron/scheduler.go` (~line 374)

Replace:

```go
pageURL := fmt.Sprintf("https://teamnetconomy.atlassian.net/wiki/spaces/%s/pages/%d", ...)
```

With URL built from:

- `spaceURL` already passed into `runIncremental` (from config / cron overrides), **or**
- Space record `spaces.url` from DB, **or**
- Shared Go helper: `BuildPageURL(baseURL, spaceKey, pageID, flavor)`

Must produce correct paths per flavor:

| Flavor | Page URL pattern |
|--------|------------------|
| Cloud | `{base}/wiki/spaces/{key}/pages/{id}` |
| Server | `{base}/display/{key}/...` or `{base}/pages/viewpage.action?pageId={id}` (prefer link from discovery/API `webui`) |

**Prefer** storing/using `confluence_url` or `webui` link from DB over reconstructing URLs.

---

### Phase 6 — Tests

#### Go unit tests (extend existing suite)

| Test | Package | Cases |
|------|---------|-------|
| `GetSpaceKeyFromURL` | `session` | Already partial; add `wiki.mycompany.com/display/KEY`, `/confluence/display/KEY` |
| `extractConfluenceBaseURL` | `session` or new `internal/confluence/urls.go` | Root vs context path |
| `BuildPageURL` (new) | `cron` or `scraper` | Cloud vs Server flavors |
| API URL construction | `scraper` | Mock server receives `/rest/api/content` on custom host |

#### Extension tests (lightweight)

If no JS test runner exists, add **manual test checklist** in this doc's Test Plan section; optional: small Node tests for shared URL parser (`shared/confluence-url.test.ts`).

#### Do not add

- Live Confluence E2E in CI
- Browser extension WebDriver tests (defer)

---

### Phase 7 — Documentation

| Doc | Update |
|-----|--------|
| `README.md` | Add self-hosted examples (`wiki.mycompany.com/display/KEY`) alongside Cloud |
| `ARCHITECTURE.md` | Diagram: custom domain + Server flavor |
| `DEVELOPMENT.md` | Extension dev on non-Atlassian URL |
| `CHANGELOG.md` | Under `[Unreleased]` |

---

## Detailed File Checklist

### Extensions (must change)

- [ ] `firefox-extension/background.ts` — remove `atlassian.net` guards
- [ ] `firefox-extension/lib/session.ts` — host-based cookies
- [ ] `firefox-extension/lib/auth.ts` — remove `teamnetconomy` hardcode
- [ ] `firefox-extension/manifest.json` — broaden permissions/CSP
- [ ] `firefox-extension/content/space-detector.ts` — `/display/`, `/spaces/`
- [ ] `chrome-extension/content/space-detector.ts` — same as Firefox
- [ ] `chrome-extension/background.ts` — dedupe into shared module
- [ ] `firefox-extension/popup/popup.html` — placeholder text
- [ ] `chrome-extension/popup/popup.html` — placeholder text

### Backend (must change)

- [ ] `internal/cron/scheduler.go` — remove hardcoded tenant URL
- [ ] `internal/scraper/scraper.go` — auto-create space URL fallback
- [ ] `internal/session/session.go` + `internal/scraper/page.go` — unified base URL with context path

### Backend (optional / Phase 4)

- [ ] `internal/config/config.go` — `confluence.base_path`
- [ ] `config.yaml.example` — document optional base path

### Tests

- [ ] Go URL helper tests
- [ ] Update extension-related test fixtures to include `wiki.mycompany.com` examples

---

## Test Plan (Manual)

After implementation, verify on each target:

| # | Scenario | Steps | Expected |
|---|----------|-------|----------|
| 1 | Server/DC root | Browse `https://wiki.mycompany.com/display/PROJ`, capture session (Firefox + Chrome) | Cookies saved; validate → `flavor: server` |
| 2 | Custom domain `/spaces/` | `https://wiki.mycompany.com/spaces/PROJ/overview` | Space key detected; crawl starts |
| 3 | Atlassian Cloud | `https://company.atlassian.net/wiki/spaces/PROJ` | No regression |
| 4 | Incremental cron | Configure self-hosted space URL in cron | Page fetches use correct host, not `teamnetconomy` |
| 5 | CLI crawl | `cli crawl "https://wiki.mycompany.com/display/PROJ"` | Pages discovered via Server API |
| 6 | Context path (if implemented) | `https://wiki.mycompany.com/confluence/display/PROJ` | API hits `/confluence/rest/api/...` |

---

## Acceptance Criteria

- [ ] Firefox extension captures session on `wiki.mycompany.com` (not only `atlassian.net`)
- [ ] Chrome and Firefox use equivalent URL detection and cookie capture logic
- [ ] Space key detected from `/display/`, `/spaces/`, and `/wiki/spaces/` URLs
- [ ] No hardcoded tenant URLs in `auth.ts` or `cron/scheduler.go`
- [ ] Backend crawl + validation work against mock Server API on custom hostname
- [ ] Atlassian Cloud path unchanged (regression check)
- [ ] README documents self-hosted setup
- [ ] `go test ./...` passes

---

## Open Questions

1. **Context path in v1?** — Auto-detect from URL only, or require `confluence.base_path` in config for `/confluence/` installs?
2. **Cookie domain breadth** — `getAll({ domain: hostname })` only, or also parent domain (`.mycompany.com`) for SSO cookies set on parent?
3. **Shared extension code** — `shared/` folder copied at build time vs duplicated files (Chrome/Firefox build pipelines differ)?
4. **Firefox permissions** — `<all_urls>` vs user-configured allowlist (enterprise security)?
5. **Server page URL format** — Reconstruct `/display/{key}` vs use `webui` links from API only (recommended for cron)?
6. **Cloud on custom domain** — Is `/wiki/rest/api/` always on the custom hostname, or do some tenants need special handling?
7. **auth.ts purpose** — Keep client-side cookie peek, or rely entirely on backend `/api/session/validate`?
8. **Content scripts** — Re-enable injection on non-Atlassian URLs in manifest (currently no `content_scripts` match in manifest — space detection runs via background only)?

---

## Related Documents

| Doc | Relationship |
|-----|--------------|
| `DOCS/task-validation-sso-fix.md` | SSO false positives affect self-hosted enterprises too |
| `DOCS/task-go-unit-tests.md` | Extend URL tests in same PR or follow-up |
| `DOCS/epic-dockerless-mode.md` | End users on self-hosted likely use dockerless binaries |
| `CHANGELOG.md` | "Broadened URL detection for custom domains" — complete the Chrome-only partial work |
| `ADR-002` | Hybrid auth still applies; extension captures cookies on real browser |

## Suggested ADR

**ADR-015: Self-Hosted Confluence URL Handling**

- Decision: Treat Confluence as flavor-driven (Cloud vs Server), not hostname-driven; extensions detect by URL path patterns; backend base URL includes optional context path.
- Record after Open Questions 1 and 3 are resolved.
