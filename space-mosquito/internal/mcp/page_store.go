package mcp

import (
	"context"

	"github.com/vkh/spacemosquito/internal/db"
)

type pageStore interface {
	GetPage(ctx context.Context, spaceKey string, confluenceID int) (*db.Page, error)
}

type dbPageStore struct {
	db *db.DB
}

func (s dbPageStore) GetPage(ctx context.Context, spaceKey string, confluenceID int) (*db.Page, error) {
	return s.db.GetPage(ctx, spaceKey, confluenceID)
}

func (s *Server) pageStore() pageStore {
	if s.pages != nil {
		return s.pages
	}
	return dbPageStore{db: s.db}
}
