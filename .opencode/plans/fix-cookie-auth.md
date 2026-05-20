# Fix Cookie Auth — SameSite + Domain

## Problem
`SetupContextWithSession` maps cookies to `proto.NetworkCookie` but omits `SameSite`. Atlassian's `tenant.session.token` has `SameSite=None` — without it, rod defaults to `Strict` and Atlassian rejects the cookie, redirecting to login.

## Changes

**File: `space-mosquito/internal/scraper/scraper.go`**

1. Add `strings` import
2. Add `resolveCookieDomain` helper — expands `teamnetconomy.atlassian.net` → `.atlassian.net`
3. Add `resolveCookieSameSite` helper — maps string to `proto.NetworkCookieSameSite` enum
4. Modify `SetupContextWithSession` to use: `strings.TrimSpace(c.Value)`, `resolveCookieDomain(c.Domain)`, `resolveCookieSameSite(c.SameSite)`
5. Add base URL navigation in `CrawlSpace` — visits Atlassian root to prime cookie context

## Execution Status
- [x] Plan approved by user
- [ ] Add `strings` import
- [ ] Add `resolveCookieDomain` helper
- [ ] Add `resolveCookieSameSite` helper  
- [ ] Modify `SetupContextWithSession` cookie mapping
- [ ] Add base URL navigation in `CrawlSpace`
- [ ] Run tests
- [ ] Build
