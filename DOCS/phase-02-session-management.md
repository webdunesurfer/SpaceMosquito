# Phase 2: Session Management

## Objective
Implement secure cookie capture, storage, and validation for Confluence session authentication.

## Deliverables
- Cookie capture types and validation
- AES-GCM encrypted session file storage
- API endpoints for session management
- Extension integration for session export

## Tasks

### 2.1 — Cookie Types & Structure
- `internal/session/export.go`:
  - `Cookie` struct: `{ Name, Value, Domain, Path, Expires, Secure, HttpOnly, SameSite }`
  - `Session` struct: `{ ConfluenceURL string, Cookies []Cookie, CapturedAt time.Time }`
  - Cookie extraction from browser (via extension API)
  - Cookie validation: check if cookies grant valid Confluence access

### 2.2 — Encrypted Storage
- `internal/session/store.go`:
  - `Store` struct: manages encrypted session file
  - `Save(session *Session, key string)` — encrypts with AES-GCM, writes to file
  - `Load(key string) (*Session, error)` — decrypts and returns session
  - `HasSession() bool` — checks if session file exists
  - AES-GCM with 256-bit key (from config/env var)
  - File permissions: 0600

### 2.3 — Session Validation
- `internal/session/export.go`:
  - `Validate(session *Session) error` — makes a test request to Confluence `/rest/myself` with session cookies
  - Returns error if auth fails (cookie expired, user logged out)
  - Auto-refresh detection: compare with last validated timestamp

### 2.4 — API Endpoints
- `internal/api/handler.go`:
  - `POST /api/session` — receive session from extension, store encrypted
  - `DELETE /api/session` — remove stored session
  - `GET /api/session/status` — return auth status (valid/invalid/expired)
  - Request format:
    ```json
    {
      "confluence_url": "https://company.atlassian.net",
      "cookies": [
        {"name": "ATLSSO", "value": "...", "domain": ".atlassian.net", ...}
      ]
    }
    ```

### 2.5 — Scraper Session Integration
- `internal/scraper/scraper.go`:
  - `SetSession(session *session.Session)` — inject cookies into Playwright context
  - `CreatePersistentContext()` — launch Firefox with session cookies pre-loaded
  - Cookie injection into Playwright via `AddCookies()` API

### 2.6 — Error Handling
- Session expired → return 401 with message "Session expired, please re-authenticate"
- Invalid cookies → return 400 with details
- File write errors → return 500 with file path and permissions info

## Acceptance Criteria
- Extension can POST session cookies to backend
- Backend stores cookies encrypted on disk
- Session validation confirms active Confluence auth
- Scraper can use stored session to authenticate with Playwright
