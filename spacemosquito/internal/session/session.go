package session

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

type SessionFlavor string

const (
	FlavorCloud  SessionFlavor = "cloud"
	FlavorServer SessionFlavor = "server"
)

type Session struct {
	ConfluenceURL string        `json:"confluence_url"`
	Cookies       []Cookie      `json:"cookies"`
	CapturedAt    time.Time     `json:"captured_at"`
	ValidatedAt   *time.Time    `json:"validated_at,omitempty"`
	Flavor        SessionFlavor `json:"flavor,omitempty"`
	log           logging.Sugar
}

type ValidationResult struct {
	Valid     bool          `json:"valid"`
	Message   string        `json:"message,omitempty"`
	ExpiresAt *int64        `json:"expires_at,omitempty"`
	Flavor    SessionFlavor `json:"flavor,omitempty"`
}

func (s *Session) IsExpired(maxAge time.Duration) bool {
	return time.Since(s.CapturedAt) > maxAge
}

func (s *Session) SetLogger(l logging.Sugar) {
	s.log = l
}

// AsHeaders returns a map of HTTP headers including the Cookie header
func (s *Session) AsHeaders() map[string]string {
	headers := make(map[string]string)

	if len(s.Cookies) > 0 {
		var cookieParts []string
		for _, c := range s.Cookies {
			cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", c.Name, c.Value))
		}
		headers["Cookie"] = strings.Join(cookieParts, "; ")
	}

	// XSRF protection bypass for simple requests
	headers["X-Atlassian-Token"] = "no-check"
	headers["Accept"] = "application/json"

	return headers
}

func (s *Session) ValidateWithConfluence(confluenceURL string, timeoutSeconds int, remoteAddr string) (*ValidationResult, error) {
	rootURL := extractConfluenceRoot(confluenceURL)
	if rootURL == "" {
		rootURL = extractConfluenceRoot(s.ConfluenceURL)
	}
	if rootURL == "" {
		return &ValidationResult{Valid: false, Message: "no confluence URL available"}, nil
	}

	if len(s.Cookies) == 0 {
		return &ValidationResult{Valid: false, Message: "no cookies in session"}, nil
	}

	// Try standard endpoints
	type probe struct {
		path   string
		flavor SessionFlavor
	}

	probes := []probe{
		{"/wiki/rest/api/user/current", FlavorCloud},
		{"/rest/api/latest/myself", FlavorServer},
		{"/rest/api/user/current", FlavorServer},
	}

	var lastErr error
	for _, p := range probes {
		testURL := fmt.Sprintf("%s%s", rootURL, p.path)
		if s.log.Enabled() {
			s.log.Infow("probing session validation", "url", testURL, "flavor", p.flavor)
		}

		req, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			continue
		}

		for _, c := range s.Cookies {
			req.AddCookie(&http.Cookie{
				Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path,
				Expires: time.Unix(c.Expires, 0), Secure: c.Secure, HttpOnly: c.HTTPOnly,
			})
		}

		client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			now := time.Now()
			s.ValidatedAt = &now
			s.Flavor = p.flavor

			var myself map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&myself)

			msg := "authenticated"
			if name, ok := (myself["displayName"].(string)); ok {
				msg = fmt.Sprintf("authenticated as %s", name)
			} else if name, ok := (myself["username"].(string)); ok {
				msg = fmt.Sprintf("authenticated as %s", name)
			}

			return &ValidationResult{
				Valid:   true,
				Message: msg,
				Flavor:  p.flavor,
			}, nil
		}

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return &ValidationResult{Valid: false, Message: "authentication failed — session expired"}, nil
		}
	}

	if lastErr != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("request failed: %v", lastErr)}, nil
	}

	return &ValidationResult{Valid: false, Message: "confluence API not found (404) at probed endpoints"}, nil
}

// GetSpaceKeyFromURL extracts space key from Confluence URL
func GetSpaceKeyFromURL(url string) string {
	// Handle /wiki/spaces/KEY
	if strings.Contains(url, "/wiki/spaces/") {
		parts := strings.Split(url, "/wiki/spaces/")
		if len(parts) > 1 {
			return strings.Split(parts[1], "/")[0]
		}
	}
	// Handle /spaces/KEY (Common for custom domains)
	if strings.Contains(url, "/spaces/") {
		parts := strings.Split(url, "/spaces/")
		if len(parts) > 1 {
			return strings.Split(parts[1], "/")[0]
		}
	}
	// Handle /display/KEY (Standard Server/DC)
	if strings.Contains(url, "/display/") {
		parts := strings.Split(url, "/display/")
		if len(parts) > 1 {
			return strings.Split(parts[1], "/")[0]
		}
	}
	return ""
}

// GetSpaceNameFromURL extracts space name from Confluence URL
func GetSpaceNameFromURL(url string) string {
	key := GetSpaceKeyFromURL(url)
	if key != "" {
		return strings.ToUpper(key)
	}
	return ""
}

// extractConfluenceRoot extracts the base URL (scheme + host) from a Confluence URL
func extractConfluenceRoot(urlStr string) string {
	if urlStr == "" {
		return ""
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	// Base is just scheme + host
	root := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	return root
}
