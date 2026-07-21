package contentmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
)

const (
	// ContentFileName is the on-disk markdown artifact per ADR-015.
	ContentFileName = "content.md"
)

var multiNewline = regexp.MustCompile(`\n{3,}`)

var (
	mdConverter     *converter.Converter
	mdConverterOnce sync.Once
)

var contentSelectors = []string{
	"#main-content",
	".wiki-content",
	`[data-testid="page-content"]`,
	"main",
}

func markdownConverter() *converter.Converter {
	mdConverterOnce.Do(func() {
		mdConverter = converter.NewConverter(
			converter.WithPlugins(
				base.NewBasePlugin(),
				table.NewTablePlugin(),
			),
		)
	})
	return mdConverter
}

// HTMLToMarkdown converts clean or raw HTML into normalized Markdown.
func HTMLToMarkdown(html string) (string, error) {
	html = strings.TrimSpace(html)
	if html == "" {
		return "", nil
	}

	scoped := scopeHTML(html)
	md, err := markdownConverter().ConvertString(scoped)
	if err != nil {
		return "", fmt.Errorf("convert html to markdown: %w", err)
	}
	return normalizeMarkdown(md), nil
}

// HTMLFileToMarkdown reads a file and converts it to Markdown.
func HTMLFileToMarkdown(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return HTMLToMarkdown(string(b))
}

// WriteFile writes content.md into a page directory.
func WriteFile(dir, markdown string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("empty page directory")
	}
	path := filepath.Join(dir, ContentFileName)
	return os.WriteFile(path, []byte(markdown), 0644)
}

func scopeHTML(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return html
	}

	for _, sel := range contentSelectors {
		node := doc.Find(sel).First()
		if node.Length() == 0 {
			continue
		}
		fragment, err := node.Html()
		if err != nil || strings.TrimSpace(fragment) == "" {
			continue
		}
		return fragment
	}
	return html
}

func normalizeMarkdown(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	s = strings.TrimSpace(strings.Join(lines, "\n"))
	s = multiNewline.ReplaceAllString(s, "\n\n")
	return s
}
