package dbcache

import (
	"context"
	"log"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
)

const (
	AppStorageCacheSize = 16
)

type AppStorage struct {
	fpCache *lru.Cache[int64, *model.FooterPlugs]
	next    state.AppStorage

	cacheHit  int
	cacheMiss int
}

var _ state.AppStorage = (*AppStorage)(nil)

func NewAppStorage(size int, nx state.AppStorage) *AppStorage {
	fpCache, err := lru.New[int64, *model.FooterPlugs](size)
	if err != nil {
		log.Fatalf("Failed to create FooterPlugsStorage cache: %v", err)
	}
	return &AppStorage{
		fpCache: fpCache,
		next:    nx,
	}
}

func (a *AppStorage) CreateFooterPlugSet(ctx context.Context, name string, plugs []string) (int64, error) {
	return a.next.CreateFooterPlugSet(ctx, name, plugs)
}

// CreateStructure implements state.AppStorage.
func (a *AppStorage) CreateStructure(ctx context.Context, s *model.Structure) (int64, error) {
	return a.next.CreateStructure(ctx, s)
}

// DeleteFooterPlugSet implements state.AppStorage.
func (a *AppStorage) DeleteFooterPlugSet(ctx context.Context, id int64) error {
	err := a.next.DeleteFooterPlugSet(ctx, id)
	if err != nil {
		a.fpCache.Remove(id)
	}
	return err
}

// DeleteStructure implements state.AppStorage.
func (a *AppStorage) DeleteStructure(ctx context.Context, id int64) error {
	return a.next.DeleteStructure(ctx, id)
}

// FetchPlugs implements state.AppStorage.
//
// TODO: This does not consume from any other server instances;
// we need to be notified of any writes we didn't make.  But for now,
// there's one server instance, so...
func (a *AppStorage) FetchPlugs(ctx context.Context, id int64) (*model.FooterPlugs, error) {
	if plugs, ok := a.fpCache.Get(id); ok {
		a.cacheHit++
		return plugs.Clone(), nil
	}
	a.cacheMiss++
	fp, err := a.next.FetchPlugs(ctx, id)
	if err != nil {
		return nil, err
	}
	a.fpCache.Add(id, fp.Clone())
	return fp, nil
}

// FetchStructure implements state.AppStorage.
func (a *AppStorage) FetchStructure(ctx context.Context, id int64) (*model.Structure, error) {
	return a.next.FetchStructure(ctx, id)
}

// FetchStructureSlugs implements state.AppStorage.
func (a *AppStorage) FetchStructureSlugs(ctx context.Context, offset int, limit int) ([]*model.StructureSlug, error) {
	return a.next.FetchStructureSlugs(ctx, offset, limit)
}

// ListFooterPlugSets implements state.AppStorage.
func (a *AppStorage) ListFooterPlugSets(ctx context.Context) ([]*model.FooterPlugs, error) {
	return a.next.ListFooterPlugSets(ctx)
}

// SaveStructure implements state.AppStorage.
func (a *AppStorage) SaveStructure(ctx context.Context, s *model.Structure) error {
	return a.next.SaveStructure(ctx, s)
}

// UpdateFooterPlugSet implements state.AppStorage.
func (a *AppStorage) UpdateFooterPlugSet(ctx context.Context, id int64, name string, plugs []string) error {
	err := a.next.UpdateFooterPlugSet(ctx, id, name, plugs)
	if err != nil {
		return err
	}
	// Just remove from cache, we'll pick it up on the next read.
	// TODO: If Update returned the updated object or the version number,
	// we could capture the version number and make sure we have the newest
	// one in the case of a write-write conflict.  This works too, it just
	// isn't as efficient.
	a.fpCache.Remove(id)
	return nil
}
