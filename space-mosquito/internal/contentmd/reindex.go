package contentmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vkh/spacemosquito/internal/store"
)

// ReindexResult summarizes a content re-extraction run.
type ReindexResult struct {
	Updated int
	Skipped int
	Errors  []string
}

// ReindexAll regenerates content.md and pages.content from saved index.html files.
func ReindexAll(ctx context.Context, db store.Store, savedBase string) (ReindexResult, error) {
	pages, err := db.ListAllPages(ctx)
	if err != nil {
		return ReindexResult{}, err
	}

	var result ReindexResult
	for _, page := range pages {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		htmlPath := resolveHTMLPath(page, savedBase)
		if htmlPath == "" {
			result.Skipped++
			continue
		}

		md, err := HTMLFileToMarkdown(htmlPath)
		if err != nil || strings.TrimSpace(md) == "" {
			result.Skipped++
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", htmlPath, err))
			}
			continue
		}

		dir := pageDir(page, htmlPath)
		if dir != "" {
			if err := WriteFile(dir, md); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: write content.md: %v", dir, err))
			}
		}

		page.Content = md
		if err := db.UpsertPage(ctx, &page); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("confluence_id=%d: %v", page.ConfluenceID, err))
			result.Skipped++
			continue
		}
		result.Updated++
	}
	return result, nil
}

func resolveHTMLPath(page store.Page, savedBase string) string {
	candidates := []string{}
	if page.HTMLPath != "" {
		candidates = append(candidates, page.HTMLPath)
	}
	if page.FileDir != "" {
		candidates = append(candidates, filepath.Join(page.FileDir, "index.html"))
	}
	if savedBase != "" && page.FileDir != "" {
		candidates = append(candidates, filepath.Join(savedBase, filepath.Base(filepath.Dir(page.FileDir)), filepath.Base(page.FileDir), "index.html"))
	}

	seen := map[string]bool{}
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func pageDir(page store.Page, htmlPath string) string {
	if page.FileDir != "" {
		return page.FileDir
	}
	if htmlPath != "" {
		return filepath.Dir(htmlPath)
	}
	return ""
}
