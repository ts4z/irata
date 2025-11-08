package dbcache

import (
	"context"
	"time"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
	"github.com/ts4z/irata/varz"
)

// Note that this assumes it is the only writer, and it is not.
// This will be less true in the future if we have multiple app servers.

const (
	ttl = time.Duration(30) * time.Minute
)

type Nower interface {
	Now() time.Time
}

type SiteStorage struct {
	clock Nower
	next  state.SiteStorage

	cachedConfig *model.SiteConfig
	fetchedAt    time.Time
}

var _ state.SiteStorage = (*SiteStorage)(nil)

var (
	siteStorageCacheHits   = varz.NewInt("siteStoragecacheHits")
	siteStorageCacheMisses = varz.NewInt("siteStoragecacheMisses")
)

func NewSiteConfigStorage(next state.SiteStorage, clock Nower) *SiteStorage {
	return &SiteStorage{
		next:  next,
		clock: clock,
	}
}

// FetchSiteConfig implements state.SiteStorage.
func (s *SiteStorage) FetchSiteConfig(ctx context.Context) (*model.SiteConfig, error) {
	if s.cachedConfig != nil && s.fetchedAt.Add(ttl).After(s.clock.Now()) {
		siteStorageCacheHits.Add(1)
		return s.cachedConfig, nil
	}
	siteStorageCacheMisses.Add(1)
	config, err := s.next.FetchSiteConfig(ctx)
	if err != nil {
		return nil, err
	}
	s.fetchedAt = s.clock.Now()
	s.cachedConfig = config
	return config, nil
}

// SaveSiteConfig implements state.SiteStorage.
func (s *SiteStorage) SaveSiteConfig(ctx context.Context, config *model.SiteConfig) error {
	s.cachedConfig = config
	return s.next.SaveSiteConfig(ctx, config)
}
