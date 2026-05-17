package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Metadata struct {
	Title             string            `json:"title"`
	ConfluenceURL     string            `json:"confluence_url"`
	SpaceKey          string            `json:"space_key"`
	ParentTitle       string            `json:"parent_title,omitempty"`
	Author            string            `json:"author,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	Images            []AssetRef        `json:"images,omitempty"`
	Attachments       []AssetRef        `json:"attachments,omitempty"`
	SavedAt           time.Time         `json:"saved_at"`
}

type AssetRef struct {
	OriginalURL string `json:"original_url"`
	LocalPath   string `json:"local_path"`
}

type Writer struct {
	basePath string
}

func NewWriter(basePath string) *Writer {
	return &Writer{basePath: basePath}
}

func (w *Writer) MakePageDir(spaceKey, pageTitle string) (string, error) {
	safeTitle := sanitizeFilename(pageTitle)
	dir := filepath.Join(w.basePath, spaceKey, safeTitle)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create page dir: %w", err)
	}
	return dir, nil
}

func (w *Writer) SaveHTML(dir, html string) error {
	return os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0644)
}

func (w *Writer) SaveRawHTML(dir, html string) error {
	return os.WriteFile(filepath.Join(dir, "raw.html"), []byte(html), 0644)
}

func (w *Writer) SaveMetadata(dir string, meta *Metadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "metadata.json"), data, 0644)
}

func (w *Writer) SaveAsset(dir, url, localPath string) (string, error) {
	destDir := filepath.Join(dir, localPath)
	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		return "", fmt.Errorf("create asset dir: %w", err)
	}
	return destDir, nil
}

func (w *Writer) GetSavedPath(spaceKey, pageTitle string) string {
	return filepath.Join(w.basePath, spaceKey, sanitizeFilename(pageTitle))
}

func sanitizeFilename(name string) string {
	s := strings.ReplaceAll(name, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.TrimSpace(s)
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}
