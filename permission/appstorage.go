package permission

import (
	"context"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
)

var _ state.AppStorage = &AppStorage{}

type AppStorage struct {
	Storage state.AppStorage
}

func (s *AppStorage) CreateFooterPlugSet(ctx context.Context, name string, plugs []string) (int64, error) {
	if err := CheckAdminAccess(ctx); err != nil {
		return -1, err
	}
	return s.Storage.CreateFooterPlugSet(ctx, name, plugs)
}

// DeleteFooterPlugSet implements state.AppStorage.
func (s *AppStorage) DeleteFooterPlugSet(ctx context.Context, id int64) error {
	if err := CheckAdminAccess(ctx); err != nil {
		return err
	}
	return s.Storage.DeleteFooterPlugSet(ctx, id)
}

// ListFooterPlugSets implements state.AppStorage.a
func (s *AppStorage) ListFooterPlugSets(ctx context.Context) ([]*model.FooterPlugs, error) {
	if err := CheckAdminAccess(ctx); err != nil {
		return nil, err
	}
	return s.Storage.ListFooterPlugSets(ctx)
}

// UpdateFooterPlugSet implements state.AppStorage.
func (s *AppStorage) UpdateFooterPlugSet(ctx context.Context, id int64, name string, plugs []string) error {
	if err := CheckAdminAccess(ctx); err != nil {
		return err
	}
	return s.Storage.UpdateFooterPlugSet(ctx, id, name, plugs)
}

func (s *AppStorage) FetchStructureSlugs(ctx context.Context, offset, limit int) ([]*model.StructureSlug, error) {
	return s.Storage.FetchStructureSlugs(ctx, offset, limit)
}

func (s *AppStorage) FetchPlugs(ctx context.Context, id int64) (*model.FooterPlugs, error) {
	return s.Storage.FetchPlugs(ctx, id)
}

func (s *AppStorage) FetchStructure(ctx context.Context, id int64) (*model.Structure, error) {
	return s.Storage.FetchStructure(ctx, id)
}

func (s *AppStorage) SaveStructure(ctx context.Context, st *model.Structure) error {
	if err := CheckAdminAccess(ctx); err != nil {
		return err
	}
	return s.Storage.SaveStructure(ctx, st)
}

func (s *AppStorage) DeleteStructure(ctx context.Context, id int64) error {
	if err := CheckAdminAccess(ctx); err != nil {
		return err
	}
	return s.Storage.DeleteStructure(ctx, id)
}

func (s *AppStorage) CreateStructure(ctx context.Context, st *model.Structure) (int64, error) {
	if err := CheckAdminAccess(ctx); err != nil {
		return -1, err
	}
	return s.Storage.CreateStructure(ctx, st)
}
