package store

import (
	"errors"
	"fmt"
	"strings"
)

var ErrPageNotFound = errors.New("page not found")

// PageCandidate identifies one page when confluence_id is ambiguous across spaces.
type PageCandidate struct {
	SpaceKey string `json:"space_key"`
	Title    string `json:"title"`
}

// AmbiguousPageError is returned when multiple spaces contain the same confluence_id.
type AmbiguousPageError struct {
	ConfluenceID int
	Candidates   []PageCandidate
}

func (e *AmbiguousPageError) Error() string {
	var parts []string
	for _, c := range e.Candidates {
		parts = append(parts, fmt.Sprintf("%s (%s)", c.SpaceKey, c.Title))
	}
	return fmt.Sprintf("ambiguous confluence_id %d in spaces: %s — pass space_key",
		e.ConfluenceID, strings.Join(parts, ", "))
}
