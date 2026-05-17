package session

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/vkh/spacemosquito/pkg/logging"
)

type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires,omitempty"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"httpOnly"`
	SameSite string `json:"sameSite,omitempty"`
}

type Session struct {
	ConfluenceURL string     `json:"confluence_url"`
	Cookies       []Cookie   `json:"cookies"`
	CapturedAt    time.Time  `json:"captured_at"`
	ValidatedAt   *time.Time `json:"validated_at,omitempty"`
	log           logging.Sugar
}

type ValidationResult struct {
	Valid     bool   `json:"valid"`
	Message   string `json:"message,omitempty"`
	ExpiresAt *int64 `json:"expires_at,omitempty"`
}

func (s *Session) IsExpired(maxAge time.Duration) bool {
	return time.Since(s.CapturedAt) > maxAge
}

func (s *Session) SetLogger(l logging.Sugar) {
	s.log = l
}

func (s *Session) ValidateWithConfluence(confluenceURL string, timeoutSeconds int, remoteAddr string) (*ValidationResult, error) {
	// Extract Confluence root URL from full space URL
	rootURL := extractConfluenceRoot(confluenceURL)
	if rootURL == "" {
		rootURL = extractConfluenceRoot(s.ConfluenceURL)
	}
	if rootURL == "" {
		if s.log.Enabled() {
			s.log.Infow("session validation skipped: no confluence URL", "remote_addr", remoteAddr)
		}
		return &ValidationResult{
			Valid:   false,
			Message: "no confluence URL available",
		}, nil
	}

	if len(s.Cookies) == 0 {
		if s.log.Enabled() {
			s.log.Infow("session validation skipped: no cookies", "remote_addr", remoteAddr)
		}
		return &ValidationResult{
			Valid:   false,
			Message: "no cookies in session",
		}, nil
	}

	// Try Confluence Cloud REST API endpoint first
	testURL := fmt.Sprintf("%s/wiki/rest/api/user/current", rootURL)
	if s.log.Enabled() {
		s.log.Infow("validating session with confluence",
			"url", testURL,
			"root_url", rootURL,
			"cookie_count", len(s.Cookies),
			"remote_addr", remoteAddr)
	}
	if s.log.Enabled() {
		s.log.Infow("validating session with confluence",
			"url", testURL,
			"cookie_count", len(s.Cookies),
			"remote_addr", remoteAddr)
	}

	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for _, c := range s.Cookies {
		req.AddCookie(&http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  time.Unix(c.Expires, 0),
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}

	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		if s.log.Enabled() {
			s.log.Errorw("session validation request failed",
				"url", testURL,
				"error", err,
				"remote_addr", remoteAddr)
		}
		return &ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("request failed: %s", err.Error()),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if s.log.Enabled() {
			s.log.Warnw("session validation failed: auth rejected",
				"status", resp.StatusCode,
				"remote_addr", remoteAddr)
		}
		return &ValidationResult{
			Valid:   false,
			Message: "authentication failed — session expired or invalid",
		}, nil
	}

	if resp.StatusCode >= 400 {
		if s.log.Enabled() {
			s.log.Warnw("session validation failed: unexpected status",
				"status", resp.StatusCode,
				"remote_addr", remoteAddr)
		}
		return &ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("unexpected response: %d", resp.StatusCode),
		}, nil
	}

	now := time.Now()
	s.ValidatedAt = &now

	var myself map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&myself); err != nil {
		if s.log.Enabled() {
			s.log.Errorw("session validation failed: parse error",
				"remote_addr", remoteAddr,
				"error", err)
		}
		return &ValidationResult{
			Valid:   false,
			Message: "failed to parse response",
		}, nil
	}

	// Confluence Cloud uses "username" field
	if username, ok := myself["username"].(string); ok {
		if s.log.Enabled() {
			s.log.Infow("session validated successfully",
				"username", username,
				"remote_addr", remoteAddr)
		}
		return &ValidationResult{
			Valid:   true,
			Message: fmt.Sprintf("authenticated as %s", username),
		}, nil
	}

	// Fallback: check for displayName
	if displayName, ok := myself["displayName"].(string); ok {
		if s.log.Enabled() {
			s.log.Infow("session validated successfully",
				"displayName", displayName,
				"remote_addr", remoteAddr)
		}
		return &ValidationResult{
			Valid:   true,
			Message: fmt.Sprintf("authenticated as %s", displayName),
		}, nil
	}

	if s.log.Enabled() {
		s.log.Infow("session validated successfully", "remote_addr", remoteAddr)
	}
	return &ValidationResult{
		Valid:   true,
		Message: "authenticated",
	}, nil
}

// GetSpaceKeyFromURL extracts space key from Confluence URL
func GetSpaceKeyFromURL(url string) string {
	// Handle URLs like: https://tenant.atlassian.net/wiki/spaces/SPACEKEY/...
	if strings.Contains(url, "/wiki/spaces/") {
		parts := strings.Split(url, "/wiki/spaces/")
		if len(parts) > 1 {
			spaceKey := strings.Split(parts[1], "/")[0]
			return spaceKey
		}
	}
	return ""
}

// GetSpaceNameFromURL extracts space name from Confluence URL
func GetSpaceNameFromURL(url string) string {
	if strings.Contains(url, "/wiki/spaces/") {
		parts := strings.Split(url, "/wiki/spaces/")
		if len(parts) > 1 {
			spacePart := strings.Split(parts[1], "/")[0]
			// Convert SPACEKEY to space name (e.g., NCHB -> NCHB)
			return strings.ToUpper(spacePart)
		}
	}
	return ""
}

// extractConfluenceRoot extracts the base URL from a Confluence URL
func extractConfluenceRoot(url string) string {
	if url == "" {
		return ""
	}

	// Remove trailing slash
	url = strings.TrimRight(url, "/")

	// Handle Atlassian Cloud URLs: https://tenant.atlassian.net/wiki/...
	if strings.Contains(url, "atlassian.net/wiki/") {
		parts := strings.Split(url, "/wiki/")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	// Handle custom Confluence URLs with /wiki/
	if strings.Contains(url, "/wiki/") {
		parts := strings.Split(url, "/wiki/")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	// Handle Confluence Data Center URLs with /confluence/
	if strings.Contains(url, "/confluence/") {
		parts := strings.Split(url, "/confluence/")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	return url
}
