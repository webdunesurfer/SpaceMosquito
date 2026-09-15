package scraper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vkh/spacemosquito/internal/session"
)

func TestFetchContentVersion(t *testing.T) {
	when := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/content/42" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if !stringsContains(r.URL.RawQuery, "expand=version") {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"title": "Release Process",
			"version": map[string]any{
				"number": 12,
				"when":   when.Format(time.RFC3339),
			},
			"space": map[string]string{"key": "ENG"},
		})
	}))
	defer srv.Close()

	sess := &session.Session{
		ConfluenceURL: srv.URL,
		Flavor:        session.FlavorServer,
		Cookies:       []session.Cookie{{Name: "JSESSIONID", Value: "x"}},
	}

	cv, err := FetchContentVersion(sess, srv.URL+"/spaces/ENG", 42)
	if err != nil {
		t.Fatal(err)
	}
	if cv.Number != 12 || cv.Title != "Release Process" || cv.SpaceKey != "ENG" {
		t.Fatalf("got %+v", cv)
	}
	if !cv.When.Equal(when) {
		t.Fatalf("when = %v, want %v", cv.When, when)
	}
}

func TestFetchContentVersion_unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()

	sess := &session.Session{
		ConfluenceURL: srv.URL,
		Flavor:        session.FlavorServer,
		Cookies:       []session.Cookie{{Name: "JSESSIONID", Value: "x"}},
	}
	_, err := FetchContentVersion(sess, srv.URL+"/spaces/ENG", 42)
	if !session.IsUnauthorized(err) {
		t.Fatalf("err = %v, want unauthorized", err)
	}
}

func stringsContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}
