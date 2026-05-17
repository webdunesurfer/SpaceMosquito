# Phase 6: Firefox Extension — Auth & Session

## Objective
Build the Firefox extension for interactive authentication, session capture, and export to the Go backend.

## Deliverables
- TypeScript extension scaffold with `web-ext` tooling
- Manifest V3 with background script and content script
- Auth detection: check if user is logged into Confluence
- Login panel UI for authentication
- Session capture and export to backend

## Tasks

### 6.1 — Extension Scaffold
- `firefox-extension/`:
  ```
  firefox-extension/
  ├── manifest.json
  ├── background.ts          # Service worker
  ├── content.ts             # Injected into Confluence pages
  ├── popup/
  │   ├── popup.html
  │   ├── popup.ts
  │   └── popup.css
  ├── lib/
  │   ├── session.ts         # Cookie capture
  │   ├── api.ts             # Backend API client
  │   └── auth.ts            # Auth state detection
  ├── package.json
  ├── tsconfig.json
  └── webpack.config.js
  ```
- `package.json` dependencies:
  - `typescript`, `@types/firefox-webext-browser`
  - `webextension-polyfill`
  - `webpack`, `ts-loader`, `copy-webpack-plugin`
- `manifest.json` (Manifest V3):
  ```json
  {
    "manifest_version": 3,
    "name": "SpaceMosquito",
    "version": "0.1.0",
    "description": "Capture Confluence spaces for offline access",
    "permissions": ["cookies", "storage", "downloads"],
    "host_permissions": ["https://*.atlassian.net/*", "http://localhost:8080/*"],
    "background": {
      "service_worker": "dist/background.js"
    },
    "content_scripts": [
      {
        "matches": ["https://*.atlassian.net/*"],
        "js": ["dist/content.js"]
      }
    ],
    "action": {
      "default_popup": "popup/popup.html",
      "default_icon": "icons/icon48.png"
    }
  }
  ```

### 6.2 — Auth State Detection
- `lib/auth.ts`:
  - Content script runs on Confluence pages (`https://*.atlassian.net/*`)
  - Check for auth indicators:
    - Presence of user profile element in DOM
    - Attempt `fetch('/rest/myself')` — 200 = authenticated, 401 = not
    - Check for `atl-token` or `AJS_CONFLUENCE_TOKEN` cookies
  - Report auth state to background: `chrome.runtime.sendMessage({ type: 'auth-state', state: 'authenticated'|'unauthenticated' })`
  - Background stores auth state in `chrome.storage.local`

### 6.3 — Session Capture
- `lib/session.ts`:
  - `captureCookies()` — use `chrome.cookies.getAll({ domain: '.atlassian.net' })` to get all Confluence cookies
  - Filter to relevant cookies: ATLSSO, atlassian.token, aui.token, session-*, etc.
  - Construct session object:
    ```typescript
    interface Session {
      confluence_url: string;
      cookies: Array<{ name: string; value: string; domain: string; path: string; secure: boolean; httpOnly: boolean; sameSite: string }>;
    }
    ```
  - Send to backend: `POST http://localhost:8080/api/session`

### 6.4 — Login Panel
- `popup/popup.html`:
  - If authenticated: show status "Connected to Confluence", "Start Crawl" button, "Disconnect" button
  - If not authenticated: show login form or redirect link
  - Show crawl progress if active
  - Settings link (output directory, encryption key)
- `popup/popup.ts`:
  - Handle auth state changes
  - Trigger cookie capture on user action
  - Display backend API responses

### 6.5 — Backend API Client
- `lib/api.ts`:
  - `postSession(session: Session): Promise<void>` — POST to `/api/session`
  - `getSessionStatus(): Promise<{ valid: boolean; expiresAt?: string }>`
  - `startCrawl(spaceUrl: string): Promise<{ jobId: string }>`
  - `getCrawlStatus(jobId: string): Promise<{ status: string; progress: number }>`
  - Error handling: show toast notifications for failures

### 6.6 — Build & Development
- `webpack.config.js`:
  - TypeScript compilation to `dist/`
  - Copy assets (icons, popup HTML)
  - Dev mode with `web-ext run`
- `Makefile`:
  ```makefile
  build:
      cd firefox-extension && npx webpack --mode production

  dev:
      cd firefox-extension && npx web-ext run --source-dir ./dist --target firefox

  lint:
      cd firefox-extension && npx tsc --noEmit
  ```

## Acceptance Criteria
- Extension loads in Firefox as a temporary extension
- Auth detection correctly identifies login state on Confluence pages
- User can click extension popup → capture session → send to backend
- Backend receives and stores session (Phase 2)
- Build process produces `dist/` bundle
