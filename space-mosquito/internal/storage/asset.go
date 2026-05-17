package storage

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AssetDownloader struct {
	client *http.Client
}

func NewAssetDownloader() *AssetDownloader {
	return &AssetDownloader{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (d *AssetDownloader) Download(destDir, rawURL string) (string, error) {
	resp, err := d.client.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d", rawURL, resp.StatusCode)
	}

	ext := filepath.Ext(rawURL)
	if ext == "" {
		contentType := resp.Header.Get("Content-Type")
		switch {
		case strings.Contains(contentType, "image/png"):
			ext = ".png"
		case strings.Contains(contentType, "image/jpeg"):
			ext = ".jpg"
		case strings.Contains(contentType, "image/gif"):
			ext = ".gif"
		case strings.Contains(contentType, "image/webp"):
			ext = ".webp"
		default:
			ext = ".bin"
		}
	}

	hash := sha256.Sum256([]byte(rawURL))
	filename := fmt.Sprintf("%x%s", hash[:8], ext)
	destPath := filepath.Join(destDir, filename)

	if _, err := os.Stat(destPath); err == nil {
		return destPath, nil
	}

	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create asset file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("write asset: %w", err)
	}

	return destPath, nil
}

func (d *AssetDownloader) RewriteURL(rawURL, assetBase string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	if strings.HasPrefix(parsed.Path, "/download/attachments/") {
		ext := filepath.Ext(parsed.Path)
		hash := sha256.Sum256([]byte(rawURL))
		return filepath.Join(assetBase, "attachments", fmt.Sprintf("%x%s", hash[:8], ext))
	}

	if strings.HasPrefix(parsed.Host, "confluence-attachments") ||
		strings.Contains(parsed.Path, "/plugins/attachments") {
		ext := filepath.Ext(parsed.Path)
		if ext == "" {
			ext = ".bin"
		}
		hash := sha256.Sum256([]byte(rawURL))
		return filepath.Join(assetBase, "images", fmt.Sprintf("%x%s", hash[:8], ext))
	}

	return rawURL
}
