package permission

import (
	"context"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
)

type SiteStorage struct {
	next state.SiteStorage
}

type SiteStorageReader struct {
	next state.SiteStorageReader
}

var _ state.SiteStorage = &SiteStorage{}
var _ state.SiteStorageReader = &SiteStorageReader{}

func NewSiteConfigStorage(next state.SiteStorage) *SiteStorage {
	return &SiteStorage{
		next: next,
	}
}

func NewSiteConfigStorageReader(next state.SiteStorageReader) *SiteStorageReader {
	return &SiteStorageReader{
		next: next,
	}
}

func (s *SiteStorage) FetchSiteConfig(ctx context.Context) (*model.SiteConfig, error) {
	return requireSiteAdminReturning(ctx, func() (*model.SiteConfig, error) {
		return s.next.FetchSiteConfig(ctx)
	})
}

func (s *SiteStorage) SaveSiteConfig(ctx context.Context, sc *model.SiteConfig) error {
	return requireSiteAdmin(ctx, func() error {
		return s.next.SaveSiteConfig(ctx, sc)
	})
}

func (s *SiteStorageReader) FetchSiteConfig(ctx context.Context) (*model.SiteConfig, error) {
	return s.next.FetchSiteConfig(ctx)
}
