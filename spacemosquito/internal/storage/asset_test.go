package storage

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vkh/spacemosquito/pkg/logging"
)

func testDownloader(client *http.Client) *AssetDownloader {
	return &AssetDownloader{
		client:     client,
		maxRetries: 1,
		retryDelay: time.Millisecond,
		rateLimit:  0,
	}
}

func TestAssetDownloader_RewriteURL(t *testing.T) {
	d := NewAssetDownloader(logging.Sugar{})

	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "attachment path",
			url:  "https://example.atlassian.net/wiki/download/attachments/123/file.pdf",
			want: "attachments/",
		},
		{
			name: "confluence-attachments host",
			url:  "https://confluence-attachments.example.com/path/image",
			want: "images/",
		},
		{
			name: "invalid url returns original",
			url:  "://bad",
			want: "://bad",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := d.RewriteURL(tc.url, "assets")
			if tc.name == "invalid url returns original" {
				if got != tc.url {
					t.Errorf("got %q, want original %q", got, tc.url)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("RewriteURL(%q) = %q, want substring %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestAssetDownloader_Download_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("png-bytes"))
	}))
	defer srv.Close()

	d := testDownloader(srv.Client())
	dest := t.TempDir()

	path, err := d.Download(dest, srv.URL+"/file")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("downloaded file missing: %v", err)
	}
}

func TestAssetDownloader_Download_skipsExisting(t *testing.T) {
	body := []byte("cached")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer srv.Close()

	d := testDownloader(srv.Client())
	dest := t.TempDir()
	url := srv.URL + "/asset"

	path1, err := d.Download(dest, url)
	if err != nil {
		t.Fatal(err)
	}

	path2, err := d.Download(dest, url)
	if err != nil {
		t.Fatal(err)
	}
	if path1 != path2 {
		t.Errorf("paths differ: %q vs %q", path1, path2)
	}
}

func TestAssetDownloader_Download_retriesOnFailure(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := testDownloader(srv.Client())
	_, err := d.Download(t.TempDir(), srv.URL)
	if err == nil {
		t.Fatal("expected error after retries")
	}
	if attempts < 2 {
		t.Errorf("attempts = %d, want at least 2", attempts)
	}
}

func TestAssetDownloader_Download_contentTypeExtension(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("jpeg"))
	}))
	defer srv.Close()

	d := testDownloader(srv.Client())
	path, err := d.Download(t.TempDir(), srv.URL+"/noext")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, ".jpg") {
		t.Errorf("path %q should have .jpg extension", path)
	}
}
