package mcp

import (
	"context"

	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/internal/store"
)

type pageStore interface {
	GetPage(ctx context.Context, spaceKey string, confluenceID int) (*db.Page, error)
}

type dbPageStore struct {
	db store.Store
}

func (s dbPageStore) GetPage(ctx context.Context, spaceKey string, confluenceID int) (*db.Page, error) {
	page, err := s.db.GetPage(ctx, spaceKey, confluenceID)
	if err != nil {
		return nil, err
	}
	return page, nil
}

func (s *Server) pageStore() pageStore {
	if s.pages != nil {
		return s.pages
	}
	return dbPageStore{db: s.db}
}
