# ADR-002: Hybrid Authentication Approach

- **Status**: Accepted
- **Date**: 2025-01-17
- **Context**: Users need to authenticate with Confluence (OAuth/SSO/2FA) on their machine. We need a reliable way to capture and reuse session credentials without exposing them.
- **Decision**: Use a hybrid authentication approach where the Firefox extension captures session cookies during interactive login and exports them to the Go backend via an encrypted session file
- **Rationale**:
  - Direct form-based login from Go is fragile with modern Confluence (OAuth redirects, SSO, 2FA)
  - The extension runs in a real browser where the user can complete all authentication flows naturally
  - Session cookies can be captured after login and exported to the backend for automated cron runs
  - Cookies are stored encrypted on disk (AES-GCM), never in plaintext
  - Extension on the host talks to the local backend at localhost; cookies are exported into the encrypted session file under the data directory
- **Alternatives considered**:
  - Remote desktop / noVNC login — clunky and harder for 2FA/SSO
  - OAuth client credentials — rejected because users won't provide API tokens and we must "pretend to be a normal user"
  - Puppeteer/Playwright headless login — fragile with modern auth flows, loses the real browser's session capabilities
- **Consequences**:
  - Session file must be protected; encryption key should be user-provided or derived from OS keyring
  - Session cookies expire; the extension must handle re-authentication gracefully
  - The cookie exchange happens over localhost (encrypted or not), which is a trust boundary
