package session

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	ConfluenceURL string    `json:"confluence_url"`
	Cookies       []Cookie  `json:"cookies"`
	CapturedAt    time.Time `json:"captured_at"`
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
	testURL := confluenceURL
	if testURL == "" {
		testURL = s.ConfluenceURL
	}
	if testURL == "" {
		if s.log.Enabled() {
			s.log.Infow("session validation skipped: no confluence URL", "remote_addr", remoteAddr)
		}
		return &ValidationResult{
			Valid: false,
			Message: "no confluence URL available",
		}, nil
	}

	if len(s.Cookies) == 0 {
		if s.log.Enabled() {
			s.log.Infow("session validation skipped: no cookies", "remote_addr", remoteAddr)
		}
		return &ValidationResult{
			Valid: false,
			Message: "no cookies in session",
		}, nil
	}

	testURL = fmt.Sprintf("%s/rest/myself", testURL)
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

	if s.log.Enabled() {
		s.log.Infow("session validated successfully", "remote_addr", remoteAddr)
	}
	return &ValidationResult{
		Valid:   true,
		Message: "authenticated",
	}, nil
}
