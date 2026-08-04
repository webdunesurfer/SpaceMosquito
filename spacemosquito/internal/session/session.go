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

	pageURL := confluenceURL
	if pageURL == "" {
		pageURL = s.ConfluenceURL
	}
	probes := validationProbes(s.Flavor, pageURL)

	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var lastErr error
	sawUnauthorized := false
	sawSSOIntercept := false

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

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		outcome := classifyValidationResponse(resp)
		resp.Body.Close()

		switch outcome.kind {
		case validationOK:
			now := time.Now()
			s.ValidatedAt = &now
			s.Flavor = p.flavor
			return &ValidationResult{
				Valid:   true,
				Message: fmt.Sprintf("authenticated as %s", outcome.name),
				Flavor:  p.flavor,
			}, nil
		case validationUnauthorized:
			sawUnauthorized = true
		case validationSSOIntercept:
			sawSSOIntercept = true
			if s.log.Enabled() {
				s.log.Infow("validation probe rejected (SSO/non-JSON)",
					"url", testURL,
					"status", outcome.status,
					"reason", outcome.reason)
			}
		case validationNotFound:
			// try next probe
		}
	}

	if sawUnauthorized {
		return &ValidationResult{Valid: false, Message: "authentication failed — session expired"}, nil
	}
	if sawSSOIntercept {
		return &ValidationResult{Valid: false, Message: "authentication failed — SSO or non-JSON response (session expired or incomplete)"}, nil
	}
	if lastErr != nil {
		return &ValidationResult{Valid: false, Message: fmt.Sprintf("request failed: %v", lastErr)}, nil
	}

	return &ValidationResult{Valid: false, Message: "confluence API not found (404) at probed endpoints"}, nil
}

type validationProbe struct {
	path   string
	flavor SessionFlavor
}

func validationProbes(flavor SessionFlavor, pageURL string) []validationProbe {
	cloud := validationProbe{"/wiki/rest/api/user/current", FlavorCloud}
	server := []validationProbe{
		{"/rest/api/latest/myself", FlavorServer},
		{"/rest/api/user/current", FlavorServer},
	}
	if preferServerProbes(flavor, pageURL) {
		return append(server, cloud)
	}
	return append([]validationProbe{cloud}, server...)
}

// preferServerProbes puts Server/DC endpoints first when flavor or URL shape
// indicates non-Cloud Confluence, so a bogus Cloud-path HTML 200 from SSO does
// not short-circuit before a real Server probe.
func preferServerProbes(flavor SessionFlavor, pageURL string) bool {
	if flavor == FlavorServer {
		return true
	}
	if flavor == FlavorCloud {
		return false
	}
	u := strings.ToLower(pageURL)
	if strings.Contains(u, "atlassian.net") || strings.Contains(u, "/wiki/") {
		return false
	}
	if strings.Contains(u, "/spaces/") || strings.Contains(u, "/display/") {
		return true
	}
	// Custom host with unknown path: Server-first is safer for SSO/DC.
	if u != "" {
		if parsed, err := url.Parse(pageURL); err == nil && parsed.Host != "" && !strings.Contains(parsed.Host, "atlassian.net") {
			return true
		}
	}
	return false
}

type validationKind int

const (
	validationOK validationKind = iota
	validationUnauthorized
	validationSSOIntercept
	validationNotFound
)

type validationOutcome struct {
	kind   validationKind
	name   string
	status int
	reason string
}

func classifyValidationResponse(resp *http.Response) validationOutcome {
	status := resp.StatusCode
	if status == http.StatusMovedPermanently ||
		status == http.StatusFound ||
		status == http.StatusSeeOther ||
		status == http.StatusTemporaryRedirect ||
		status == http.StatusPermanentRedirect {
		return validationOutcome{kind: validationSSOIntercept, status: status, reason: "redirect"}
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return validationOutcome{kind: validationUnauthorized, status: status}
	}
	if status == http.StatusNotFound {
		return validationOutcome{kind: validationNotFound, status: status}
	}
	if status != http.StatusOK {
		return validationOutcome{kind: validationSSOIntercept, status: status, reason: "non-200"}
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(ct), "application/json") {
		return validationOutcome{kind: validationSSOIntercept, status: status, reason: "non-json content-type"}
	}

	var myself map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&myself); err != nil {
		return validationOutcome{kind: validationSSOIntercept, status: status, reason: "json decode failed"}
	}
	name, ok := identityFromMyself(myself)
	if !ok {
		return validationOutcome{kind: validationSSOIntercept, status: status, reason: "missing displayName/username"}
	}
	return validationOutcome{kind: validationOK, name: name, status: status}
}

func identityFromMyself(myself map[string]interface{}) (string, bool) {
	if name, ok := myself["displayName"].(string); ok {
		if name = strings.TrimSpace(name); name != "" {
			return name, true
		}
	}
	if name, ok := myself["username"].(string); ok {
		if name = strings.TrimSpace(name); name != "" {
			return name, true
		}
	}
	return "", false
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
