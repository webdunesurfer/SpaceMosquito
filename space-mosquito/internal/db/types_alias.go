package db

import "github.com/vkh/spacemosquito/internal/store"

type (
	Space        = store.Space
	Page         = store.Page
	PageSummary  = store.PageSummary
	SearchResult = store.SearchResult
	PageStats    = store.PageStats
)

var _ store.Store = (*DB)(nil)
