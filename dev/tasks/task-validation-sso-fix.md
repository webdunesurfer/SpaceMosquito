# Task: Fix False-Positive Session Validation (SSO Redirects)

## Objective

Prevent expired or invalid sessions from being reported as authenticated.
`ValidateWithConfluence` must fail closed when Azure AD / SSO intercepts the
API probe.

## Confirmed reproduction (Server/DC + Azure AD)

Observed on self-hosted Confluence (`wiki.example.net`, Server/DC) behind Azure
AD. Cloud not confirmed (same bug class is likely wherever SSO returns HTML
200).

**Trigger:** session expires overnight; next day Validate + Capture buttons
show (no green capture CTA because a session file still exists).

**UI symptom:**

| State | Extension status text |
|-------|------------------------|
| False positive (expired) | `Valid: authenticated` (no user name) |
| Real session | `Valid: authenticated as <displayName>` |

Delete → Capture → Validate restores the display-name form and crawl works
again.

**Backend log (false positive):**

```text
session loaded … cookie_count:1
probing session validation url=https://wiki.example.net/wiki/rest/api/user/current flavor=cloud
session saved successfully   ← ValidatedAt written; treat as success
POST /api/session/validate status=200
```

Notes from that run:

- Page URL shape is Server (`/spaces/...`), but the **first probe is Cloud**
  (`/wiki/rest/api/user/current`). That probe returned **HTTP 200 with SSO HTML**
  (confirmed — Azure AD login page after redirects), so validation never reached
  Server probes (`/rest/api/latest/myself`, `/rest/api/user/current`).
- JSON decode of the HTML body fails silently; `displayName` / `username`
  missing → message stays bare `"authenticated"` while `Valid: true`.
- Crawl then hits login redirects; job UI can finish at **0 pages** with status
  **Completed**.

## Root cause

`ValidateWithConfluence` in `internal/session/session.go`:

1. Uses `http.Client` that **follows redirects** by default.
2. Treats any **HTTP 200** as success.
3. **Ignores** `json.Decoder` errors and does not require a user identity field.
4. Probe order is Cloud-first; on Server/DC a bogus 200 on the Cloud path short-
   circuits before a real Server probe.

SSO (Azure AD) typically 302 → login HTML 200, so expired cookies look
“authenticated”.

## Implementation Plan

### 1. Disable automatic redirects

```go
client := &http.Client{
    Timeout: time.Duration(timeoutSeconds) * time.Second,
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        return http.ErrUseLastResponse
    },
}
```

Treat 30x as auth failure (do not continue as success).

### 2. Require JSON Content-Type

If `Content-Type` is not `application/json`, treat as failed auth (login HTML /
WAF). Prefer **continue to next probe** rather than hard-fail the whole
validate, so a bogus Cloud 200 HTML does not block a later Server JSON 200.

### 3. Fail closed on decode / missing identity

- If JSON decode fails → not authenticated for that probe (try next / fail).
- If decode succeeds but neither `displayName` nor `username` is a non-empty
  string → not authenticated (this is the UI signal users already notice).

Do **not** persist `ValidatedAt` / `Flavor` / rewrite `session.enc` on a failed
validation.

### 4. Probe ordering (recommended)

When URL / stored flavor looks Server (no `/wiki/`, or `FlavorServer`), try
Server endpoints first. Avoid promoting `FlavorCloud` from a false Cloud-path
hit on Server/DC hosts. Full custom-domain work stays in
[`task-self-hosted-confluence`](task-self-hosted-confluence.md); this task only
needs safe validate semantics.

## Testing

- httptest: 302 → HTML 200 → `Valid: false` (or next probe), no session rewrite.
- httptest: Server JSON with `displayName` → `Valid: true`, message includes name.
- httptest: Cloud path returns HTML 200, Server path returns real JSON → still
  valid via Server probe (must not stop at Cloud HTML).
- Extension: expired overnight session → Validate shows failure, not bare
  `authenticated`.

## Expected Outcome

Expired Azure AD sessions no longer show `Valid: authenticated` without a name.
Validate fails closed; crawl is not started against a dead session that looks
live. Real sessions still show `authenticated as <displayName>`.

## Related

- [`task-self-hosted-confluence`](task-self-hosted-confluence.md) — capture /
  flavor / custom host (overlaps probe URLs; validate fail-closed is this task).
