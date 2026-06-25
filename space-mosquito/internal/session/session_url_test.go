package session

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSession_AsHeaders(t *testing.T) {
	t.Run("no cookies", func(t *testing.T) {
		s := &Session{}
		h := s.AsHeaders()
		if _, ok := h["Cookie"]; ok {
			t.Fatal("expected no Cookie header")
		}
		if h["X-Atlassian-Token"] != "no-check" {
			t.Errorf("X-Atlassian-Token = %q", h["X-Atlassian-Token"])
		}
		if h["Accept"] != "application/json" {
			t.Errorf("Accept = %q", h["Accept"])
		}
	})

	t.Run("multiple cookies", func(t *testing.T) {
		s := &Session{
			Cookies: []Cookie{
				{Name: "a", Value: "1"},
				{Name: "b", Value: "2"},
			},
		}
		h := s.AsHeaders()
		if h["Cookie"] != "a=1; b=2" {
			t.Errorf("Cookie = %q", h["Cookie"])
		}
	})
}

func TestGetSpaceKeyFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://company.atlassian.net/wiki/spaces/PROJ/overview", "PROJ"},
		{"https://wiki.company.com/spaces/PROJ", "PROJ"},
		{"https://confluence.company.com/display/PROJ/Home", "PROJ"},
		{"https://company.atlassian.net/wiki/spaces/MY-SPACE/pages/1", "MY-SPACE"},
		{"https://example.com/no-space", ""},
	}
	for _, tc := range tests {
		if got := GetSpaceKeyFromURL(tc.url); got != tc.want {
			t.Errorf("GetSpaceKeyFromURL(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestGetSpaceNameFromURL(t *testing.T) {
	got := GetSpaceNameFromURL("https://company.atlassian.net/wiki/spaces/proj/overview")
	if got != "PROJ" {
		t.Errorf("got %q, want PROJ", got)
	}
}

func TestExtractConfluenceRoot(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://company.atlassian.net/wiki/spaces/PROJ", "https://company.atlassian.net"},
		{"https://confluence.example.com:8443/display/KEY", "https://confluence.example.com:8443"},
		{"", ""},
		{"not-a-url", "://"},
	}
	for _, tc := range tests {
		if got := extractConfluenceRoot(tc.url); got != tc.want {
			t.Errorf("extractConfluenceRoot(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestStore_corruptCiphertext(t *testing.T) {
	tmpFile := t.TempDir() + "/session.enc"
	if err := os.WriteFile(tmpFile, []byte("tooshort"), 0600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(tmpFile, nilSugar())
	_, err := store.Load(testKey)
	if err == nil {
		t.Fatal("expected error loading corrupt ciphertext")
	}
}

func TestStore_nestedPath(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "sub", "dir", "session.enc")
	store := NewStore(tmpFile, nilSugar())
	sess := &Session{
		ConfluenceURL: "https://example.atlassian.net",
		Cookies:       []Cookie{{Name: "t", Value: "v", Domain: ".example.com"}},
		CapturedAt:    time.Now(),
	}
	if err := store.Save(sess, testKey); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tmpFile); err != nil {
		t.Fatal(err)
	}
}

func TestValidateWithConfluence_mockServer(t *testing.T) {
	t.Run("200 json cloud", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/wiki/rest/api/user/current" {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"displayName":"Ada Lovelace"}`))
				return
			}
			http.NotFound(w, r)
		}))
		defer srv.Close()

		sess := &Session{
			ConfluenceURL: srv.URL,
			Cookies:       []Cookie{{Name: "t", Value: "v"}},
		}
		result, err := sess.ValidateWithConfluence("", 5, "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		if !result.Valid || result.Flavor != FlavorCloud {
			t.Fatalf("result = %+v", result)
		}
		if !strings.Contains(result.Message, "Ada Lovelace") {
			t.Errorf("message = %q", result.Message)
		}
	})

	t.Run("401 unauthorized", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}))
		defer srv.Close()

		sess := &Session{
			ConfluenceURL: srv.URL,
			Cookies:       []Cookie{{Name: "t", Value: "v"}},
		}
		result, err := sess.ValidateWithConfluence("", 5, "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		if result.Valid {
			t.Fatal("expected invalid session")
		}
		if !strings.Contains(result.Message, "expired") {
			t.Errorf("message = %q", result.Message)
		}
	})

	t.Run("403 forbidden", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "forbidden", http.StatusForbidden)
		}))
		defer srv.Close()

		sess := &Session{
			ConfluenceURL: srv.URL,
			Cookies:       []Cookie{{Name: "t", Value: "v"}},
		}
		result, err := sess.ValidateWithConfluence("", 5, "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		if result.Valid {
			t.Fatal("expected invalid session")
		}
	})

	t.Run("all probes 404", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer srv.Close()

		sess := &Session{
			ConfluenceURL: srv.URL,
			Cookies:       []Cookie{{Name: "t", Value: "v"}},
		}
		result, err := sess.ValidateWithConfluence("", 5, "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		if result.Valid {
			t.Fatal("expected invalid session")
		}
		if !strings.Contains(result.Message, "404") {
			t.Errorf("message = %q", result.Message)
		}
	})
}
