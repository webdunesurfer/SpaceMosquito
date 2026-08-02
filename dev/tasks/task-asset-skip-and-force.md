# Task: Skip already-downloaded assets (+ `--force` to override)

## Problem

Asset downloading re-fetches files that are already on disk, and the one skip
that exists doesn't actually save anything.

- **`DownloadAs`** (the CSF path — all images and draw.io diagrams) has **no
  skip at all** ([asset.go](../../spacemosquito/internal/storage/asset.go)). Every
  crawl re-fetches and **overwrites** every asset via `os.Create` (truncate).
- **`Download`** (legacy `<img>`/`<a href>` path) *does* skip, but the
  `os.Stat` check runs **after** the HTTP GET and the ≥5s rate-limit wait
  ([asset.go:157](../../spacemosquito/internal/storage/asset.go#L157)) — so it only
  avoids the final disk write, not the network request or the wait.

At ≥5s per asset (shared, serial rate limit), re-crawling a space with hundreds
of images takes many minutes of pure waiting on files we already have. Nothing
higher up (scraper / DB / metadata) checks "already downloaded" either.

## Goal

1. **Skip an asset when the destination file already exists — *before* the
   network request** (save both the GET and the 5s rate-limit wait).
2. Add a **`--force`** option to the `crawl` command that bypasses the skip and
   re-downloads (overwrites) everything — for refreshing stale/corrupt assets.

Default behaviour becomes: existing file → skip; missing → download. `--force`
→ always download.

## Design

### Downloader changes ([internal/storage/asset.go](../../spacemosquito/internal/storage/asset.go))

- Add a `force bool` field + `SetForce(bool)` on `AssetDownloader` (set once per
  crawl; shared instance, guard with the existing mutex).
- **`DownloadAs(destPath, rawURL)`** — trivial, `destPath` is known upfront:
  ```go
  if !d.force {
      if fi, err := os.Stat(destPath); err == nil && fi.Size() > 0 {
          return nil // already have it
      }
  }
  // …existing GET + write…
  ```
  Move this check to the very top, before `rateLimitWait()`/`get()`.
- **`Download(destDir, rawURL)`** — trickier, because the filename is
  `sha256(url)[:8] + ext` and `ext` can come from the response `Content-Type`
  when the URL has no extension, i.e. it isn't fully known pre-GET.
  - If the URL **has** an extension → compute `destPath` up front and stat it
    before the GET (same as `DownloadAs`).
  - If the URL has **no** extension → the exact name needs the response. Options:
    - **(a)** glob for an existing `sha256(url)[:8].*` in `destDir` and skip if
      any match exists (pre-GET, extension-agnostic); or
    - **(b)** accept that extensionless URLs can't pre-skip and keep the
      post-GET stat for that case only.
    *Recommend (a)* — extension-agnostic hash glob, so extensionless assets also
    skip the network.

### Skip guardrails

- Only skip files with **size > 0** (a zero-byte file is a failed/partial
  download; re-fetch it).
- Corrupt-but-nonempty legacy files (e.g. the old HTML-as-`.png` login pages)
  would be *kept* by a naive skip. Mitigation: the Content-Type guard already
  prevents creating new ones; `--force` clears old ones. Note this in `--force`
  help text.

### CLI wiring ([internal/cliapp/run.go](../../spacemosquito/internal/cliapp/run.go))

- `crawl` currently takes only `<url>` ([run.go:96](../../spacemosquito/internal/cliapp/run.go#L96),
  [runCrawl](../../spacemosquito/internal/cliapp/run.go#L401)). Add a `flag.FlagSet`
  parsing `--force` (like `bootstrap import-saved` already does).
- After building the downloader ([run.go:438](../../spacemosquito/internal/cliapp/run.go#L438)),
  call `assetDownloader.SetForce(force)` before `CrawlSpace`.
- Same wiring for the server/API-triggered and cron crawl paths if we want
  `--force` there (out of scope for v1 — CLI only; see Open Questions).

## Integration points

- `internal/storage/asset.go` — `force` field, `SetForce`, pre-GET skip in
  `DownloadAs` and `Download`.
- `internal/cliapp/run.go` — `crawl --force` flag → `SetForce`.
- Reindex/import — **unaffected**: `reindex --content` performs no downloads.

## Testing

- `DownloadAs`: existing non-empty file → **no HTTP request made** (assert the
  test server handler is never hit) and returns nil; missing file → downloads;
  `force=true` → downloads even when present (overwrites).
- `DownloadAs`: zero-byte existing file → re-downloaded (not skipped).
- `Download`: URL with extension → pre-GET skip; extensionless URL → skip via
  hash glob (per chosen option).
- `--force` end-to-end: flag flips `SetForce`.
- Existing asset tests stay green.

## Non-goals

- Content-hash / ETag / Last-Modified freshness checks (skip is existence-only).
- Re-downloading when the *remote* asset changed but the filename is unchanged
  (that's what `--force` is for).
- `--force` on server/cron crawl paths (v1 is CLI `crawl` only).

## Open questions

1. **`Download` extensionless skip** — hash glob (a) vs post-GET only (b)?
   *Recommend (a).*
2. **Config vs flag** — also expose as a `config.yaml` option, or CLI `--force`
   only? *Recommend CLI only for v1.*
3. **Scope of `--force`** — crawl only, or also a `reindex --assets` mode that
   (re)downloads assets offline? Currently reindex never downloads; a future
   `--assets` could, but needs a session. *Defer.*
4. **Corrupt-file detection** — is size > 0 enough, or also sniff magic bytes on
   skip? *Recommend size > 0 for v1; magic-byte validation is a separate
   hardening task.*
