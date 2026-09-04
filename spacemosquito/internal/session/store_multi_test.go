package session

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/vkh/spacemosquito/pkg/logging"
)

func TestHostnameKey(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://Wiki.Example.com/spaces/X", "wiki.example.com"},
		{"https://acme.atlassian.net/wiki/spaces/K", "acme.atlassian.net"},
		{"https://wiki.example.com:8443/display/K", "wiki.example.com:8443"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := HostnameKey(tc.in); got != tc.want {
			t.Errorf("HostnameKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStore_UpsertTwoHosts(t *testing.T) {
	tmpFile := t.TempDir() + "/session.enc"
	store := NewStore(tmpFile, logging.Sugar{})

	a := &Session{
		ConfluenceURL: "https://wiki-a.example.com/display/A",
		Cookies:       []Cookie{{Name: "JSESSIONID", Value: "aaa", Domain: "wiki-a.example.com"}},
		CapturedAt:    time.Now(),
	}
	b := &Session{
		ConfluenceURL: "https://wiki-b.example.com/spaces/B",
		Cookies:       []Cookie{{Name: "JSESSIONID", Value: "bbb", Domain: "wiki-b.example.com"}},
		CapturedAt:    time.Now(),
	}

	if err := store.Upsert(a, testKey); err != nil {
		t.Fatal(err)
	}
	if err := store.Upsert(b, testKey); err != nil {
		t.Fatal(err)
	}

	gotA, err := store.GetForURL(testKey, "https://wiki-a.example.com/pages/viewpage.action?pageId=1")
	if err != nil {
		t.Fatal(err)
	}
	if gotA.Cookies[0].Value != "aaa" {
		t.Fatalf("host A cookies overwritten: %+v", gotA.Cookies)
	}

	gotB, err := store.GetForHostname(testKey, "wiki-b.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if gotB.Cookies[0].Value != "bbb" {
		t.Fatalf("host B wrong: %+v", gotB.Cookies)
	}

	if store.Count(testKey) != 2 {
		t.Fatalf("count = %d, want 2", store.Count(testKey))
	}

	if _, err := store.Load(testKey); err == nil {
		t.Fatal("Load with multiple sessions should error")
	}
}

func TestStore_UpsertReplacesOneSiteOnly(t *testing.T) {
	tmpFile := t.TempDir() + "/session.enc"
	store := NewStore(tmpFile, logging.Sugar{})

	_ = store.Upsert(&Session{
		ConfluenceURL: "https://a.example.com/x",
		Cookies:       []Cookie{{Name: "c", Value: "1"}},
		CapturedAt:    time.Now(),
	}, testKey)
	_ = store.Upsert(&Session{
		ConfluenceURL: "https://b.example.com/x",
		Cookies:       []Cookie{{Name: "c", Value: "2"}},
		CapturedAt:    time.Now(),
	}, testKey)
	_ = store.Upsert(&Session{
		ConfluenceURL: "https://a.example.com/y",
		Cookies:       []Cookie{{Name: "c", Value: "1b"}},
		CapturedAt:    time.Now(),
	}, testKey)

	gotA, _ := store.GetForHostname(testKey, "a.example.com")
	gotB, _ := store.GetForHostname(testKey, "b.example.com")
	if gotA.Cookies[0].Value != "1b" {
		t.Fatalf("A not updated: %v", gotA.Cookies[0].Value)
	}
	if gotB.Cookies[0].Value != "2" {
		t.Fatalf("B overwritten: %v", gotB.Cookies[0].Value)
	}
}

func TestStore_GetForURL_MissingHost(t *testing.T) {
	tmpFile := t.TempDir() + "/session.enc"
	store := NewStore(tmpFile, logging.Sugar{})
	_ = store.Upsert(&Session{
		ConfluenceURL: "https://a.example.com/x",
		Cookies:       []Cookie{{Name: "c", Value: "1"}},
		CapturedAt:    time.Now(),
	}, testKey)

	_, err := store.GetForURL(testKey, "https://other.example.com/spaces/X")
	if !errors.Is(err, ErrNoSessionForHost) {
		t.Fatalf("want ErrNoSessionForHost, got %v", err)
	}
}

func TestStore_MigrateLegacySingleSession(t *testing.T) {
	tmpFile := t.TempDir() + "/session.enc"
	store := NewStore(tmpFile, logging.Sugar{})

	legacy := &Session{
		ConfluenceURL: "https://legacy.example.com/wiki/spaces/L",
		Cookies:       []Cookie{{Name: "t", Value: "legacy"}},
		CapturedAt:    time.Now(),
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.encryptWrite(raw, testKey); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetForURL(testKey, "https://legacy.example.com/display/X")
	if err != nil {
		t.Fatal(err)
	}
	if got.Cookies[0].Value != "legacy" {
		t.Fatalf("got %+v", got.Cookies)
	}

	if err := store.Upsert(&Session{
		ConfluenceURL: "https://other.example.com/x",
		Cookies:       []Cookie{{Name: "t", Value: "new"}},
		CapturedAt:    time.Now(),
	}, testKey); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetForURL(testKey, "https://legacy.example.com/")
	if err != nil {
		t.Fatal(err)
	}
	if got.Cookies[0].Value != "legacy" {
		t.Fatal("legacy session lost after upsert of other host")
	}
}
