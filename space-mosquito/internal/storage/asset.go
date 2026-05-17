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
	"sync"
	"time"

	"github.com/vkh/spacemosquito/pkg/logging"
)

const (
	defaultMaxRetries = 3
	defaultRetryDelay = 2 * time.Second
	defaultRateLimit  = 5 * time.Second
)

type AssetDownloader struct {
	client    *http.Client
	log       logging.Sugar
	maxRetries int
	retryDelay time.Duration
	rateLimit  time.Duration
	mu         sync.Mutex
	lastReq    time.Time
}

func NewAssetDownloader(log logging.Sugar) *AssetDownloader {
	return &AssetDownloader{
		client:     &http.Client{Timeout: 30 * time.Second},
		log:        log,
		maxRetries: defaultMaxRetries,
		retryDelay: defaultRetryDelay,
		rateLimit:  defaultRateLimit,
	}
}

func (d *AssetDownloader) Download(destDir, rawURL string) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if attempt > 0 {
			retryDelay := time.Duration(attempt) * d.retryDelay
			if d.log.Enabled() {
				d.log.Warnw("retrying asset download",
					"url", rawURL,
					"attempt", attempt+1,
					"max_retries", d.maxRetries,
					"retry_delay_ms", retryDelay.Milliseconds())
			}
			time.Sleep(retryDelay)
		}

		// Rate limiting
		d.mu.Lock()
		sinceLast := time.Since(d.lastReq)
		if sinceLast < d.rateLimit {
			wait := d.rateLimit - sinceLast
			d.mu.Unlock()
			if d.log.Enabled() && attempt == 0 {
				d.log.Debugw("rate limit wait", "wait_ms", wait.Milliseconds())
			}
			time.Sleep(wait)
		} else {
			d.mu.Unlock()
		}
		d.mu.Lock()
		d.lastReq = time.Now()
		d.mu.Unlock()

		resp, err := d.client.Get(rawURL)
		if err != nil {
			lastErr = fmt.Errorf("request error: %w", err)
			if d.log.Enabled() {
				d.log.Warnw("asset download attempt failed: request error",
					"url", rawURL,
					"attempt", attempt+1,
					"error", err)
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			resp.Body.Close()
			if d.log.Enabled() {
				d.log.Warnw("asset download attempt failed: non-200 status",
					"url", rawURL,
					"status", resp.StatusCode,
					"attempt", attempt+1)
			}
			continue
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

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			lastErr = fmt.Errorf("create asset dir: %w", err)
			if d.log.Enabled() {
				d.log.Errorw("asset download failed: directory creation",
					"path", filepath.Dir(destPath), "error", err)
			}
			continue
		}

		if _, err := os.Stat(destPath); err == nil {
			resp.Body.Close()
			if d.log.Enabled() {
				d.log.Debugw("asset already exists, skipping download",
					"url", rawURL,
					"path", destPath)
			}
			return destPath, nil
		}

		f, err := os.Create(destPath)
		if err != nil {
			resp.Body.Close()
			lastErr = fmt.Errorf("create asset file: %w", err)
			if d.log.Enabled() {
				d.log.Errorw("asset download failed: file creation",
					"path", destPath, "error", err)
			}
			continue
		}

		size, err := io.Copy(f, resp.Body)
		resp.Body.Close()
		f.Close()

		if err != nil {
			lastErr = fmt.Errorf("write asset: %w", err)
			if d.log.Enabled() {
				d.log.Errorw("asset download failed: write error",
					"url", rawURL, "error", err)
			}
			_ = os.Remove(destPath)
			continue
		}

		if d.log.Enabled() {
			d.log.Infow("asset downloaded successfully",
				"url", rawURL,
				"path", destPath,
				"bytes", size)
		}

		return destPath, nil
	}

	return "", fmt.Errorf("download %s: failed after %d attempts: %w", rawURL, d.maxRetries+1, lastErr)
}

func (d *AssetDownloader) RewriteURL(rawURL, assetBase string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		if d.log.Enabled() {
			d.log.Debugw("asset URL rewrite failed: parse error", "url", rawURL, "error", err)
		}
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
