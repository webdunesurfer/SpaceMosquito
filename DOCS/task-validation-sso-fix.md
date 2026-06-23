# Task: Fix False-Positive Session Validation (SSO Redirects)

## Objective
Prevent invalid or expired sessions from being falsely reported as "Authenticated" during the validation phase. Ensure the validation logic accurately identifies when a request has been intercepted by a Single Sign-On (SSO) gateway.

## Problem Description
Currently, the `ValidateWithConfluence` function in `internal/session/session.go` relies primarily on checking for a `200 OK` HTTP status code. 

In enterprise environments (e.g., behind Azure AD SSO), an unauthenticated request to the Confluence REST API does not always return a `401 Unauthorized`. Instead, the proxy intercepts the request and issues a `302 Found` redirect to an external login page (like `login.microsoftonline.com`). 

Because Go's standard `http.Client` follows redirects by default, it eventually receives a `200 OK` containing the HTML of the login page. The current code ignores JSON parsing errors, resulting in a false-positive "authenticated" state, which later causes the scraper to fail with "invalid character '<' looking for beginning of value".

## Implementation Plan

### 1. Disable Automatic Redirects
Modify the `http.Client` used in `ValidateWithConfluence` to explicitly reject redirects. A redirect during an API probe is a definitive sign that authentication has failed.

*   **File**: `internal/session/session.go`
*   **Action**: Add a `CheckRedirect` policy to the `http.Client`.
```go
client := &http.Client{
    Timeout: time.Duration(timeoutSeconds) * time.Second,
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        return http.ErrUseLastResponse // Do not follow redirects
    },
}
```

### 2. Enforce Strict Content-Type Validation
Before attempting to parse the response body, verify that the server returned JSON.

*   **Action**: Add a check for `application/json`.
```go
contentType := resp.Header.Get("Content-Type")
if !strings.Contains(contentType, "application/json") {
    // If we get HTML or anything else, we hit a login wall or WAF.
    // Treat as failed authentication.
}
```

### 3. Handle Decoding Errors
Do not silently ignore JSON parsing failures. 

*   **Action**: Check the error returned by `json.NewDecoder`. If it fails, report the session as invalid.
```go
if err := json.NewDecoder(resp.Body).Decode(&myself); err != nil {
    return &ValidationResult{Valid: false, Message: "failed to parse API response (possible SSO interception)"}, nil
}
```

### 4. Handle 30x Status Codes
Update the status code checking logic to treat redirects as auth failures.

*   **Action**:
```go
if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
    return &ValidationResult{Valid: false, Message: "authentication failed — redirected to SSO"}, nil
}
```

## Expected Outcome
If a user captures an incomplete cookie set or their session expires, clicking "Validate" in the extension will accurately return an error (e.g., "Redirected to SSO" or "Invalid Content Type") rather than a false "Authenticated" status with an empty name. This prevents downstream crawling failures.