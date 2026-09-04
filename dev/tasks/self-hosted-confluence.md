# Self-hosted Confluence (epic)

- **Task ID:** `self-hosted-confluence`
- **Status:** done
- **Children:**
  - [`self-hosted-cron-webui`](self-hosted-cron-webui.md)
  - [`self-hosted-context-path`](self-hosted-context-path.md)
  - [`self-hosted-extension-cookies`](self-hosted-extension-cookies.md)

## Goal

End-to-end support for **self-hosted / custom-domain** Confluence (Server/DC and
Cloud on `wiki.mycompany.com`), not only `*.atlassian.net`.

## Locked decisions (all phases)

| # | Topic | Decision |
|---|--------|----------|
| 1 | Context path | **Auto-detect** from URL; no required config |
| 2 | Cookie domain | **Hostname only** |
| 3 | Extension code | **Keep Chrome/Firefox duplicates** (no `shared/`) |
| 4 | Firefox permissions | **`<all_urls>`** (already done) |
| 5 | Cron page URL | Prefer stored **webui / browse URL** |
| 6 | Cloud custom domain | API on **same hostname** |
| 7 | `auth.ts` | Keep **client cookie peek** |
| 8 | Content scripts | **Leave unregistered** |

## Already done (do not re-implement)

- Firefox/Chrome `isConfluenceUrl` + background space parse (`/wiki/spaces/`, `/spaces/`, `/display/`)
- Firefox `<all_urls>` + non-Atlassian CSP `connect-src`
- Firefox host-based cookie capture (needs hostname-only polish — see child)
- `auth.ts` no longer hardcodes a Cloud tenant
- Backend Server flavor probes / partial URL parsing

## Remaining children (order)

1. ~~**Cron webui**~~ — done
2. ~~**Context path**~~ — done
3. ~~**Extension cookies**~~ — done

Tests and docs land in each child’s `done_when`. Manual smoke on custom host confirmed.

## Epic done when

- [x] All three children shipped
- [x] Capture / validate / crawl work on `wiki.example.com` (manual checklist)
- [x] No hardcoded tenant URLs in cron
- [x] Cloud path unchanged
- [x] README documents self-hosted examples

## Suggested ADR

After children ship: **Self-hosted Confluence URL handling** — flavor-driven
(Cloud vs Server), path-based extension detection, auto-detected context path,
cron uses stored browse URL.
