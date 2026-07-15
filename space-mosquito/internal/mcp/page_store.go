package mcp

import (
	"context"

	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/internal/search"
	"github.com/vkh/spacemosquito/internal/store"
)

type pageStore interface {
	GetPageByConfluenceID(ctx context.Context, confluenceID int, spaceKey string) (*db.Page, string, error)
}

type dbPageStore struct {
	db store.Store
}

func (s dbPageStore) GetPageByConfluenceID(ctx context.Context, confluenceID int, spaceKey string) (*db.Page, string, error) {
	return s.db.GetPageByConfluenceID(ctx, confluenceID, spaceKey)
}

func (s *Server) pageStore() pageStore {
	if s.pages != nil {
		return s.pages
	}
	return dbPageStore{db: s.db}
}

func (s *Server) toolGetPage(args map[string]interface{}) (interface{}, error) {
	spaceKey, confluenceID, err := parseGetPageArgs(args)
	if err != nil {
		return nil, err
	}
	page, resolvedKey, err := s.pageStore().GetPageByConfluenceID(context.Background(), confluenceID, spaceKey)
	if err != nil {
		return nil, err
	}
	return search.ToPageDetail(page, resolvedKey, s.cfg.MCP.ExposeInternalIDs), nil
}
