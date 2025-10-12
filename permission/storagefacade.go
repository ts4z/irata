package permission

import (
	"context"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
)

type StoragePermissionFacade struct {
	Storage state.AppStorage
}

func (s *StoragePermissionFacade) CreateFooterPlugSet(ctx context.Context, name string, plugs []string) (int64, error) {
	if err := CheckAdminAccess(ctx); err != nil {
		return -1, err
	}
	return s.Storage.CreateFooterPlugSet(ctx, name, plugs)
}

// DeleteFooterPlugSet implements state.AppStorage.
func (s *StoragePermissionFacade) DeleteFooterPlugSet(ctx context.Context, id int64) error {
	if err := CheckAdminAccess(ctx); err != nil {
		return err
	}
	return s.Storage.DeleteFooterPlugSet(ctx, id)
}

// ListFooterPlugSets implements state.AppStorage.a
func (s *StoragePermissionFacade) ListFooterPlugSets(ctx context.Context) ([]*model.FooterPlugs, error) {
	if err := CheckAdminAccess(ctx); err != nil {
		return nil, err
	}
	return s.Storage.ListFooterPlugSets(ctx)
}

// UpdateFooterPlugSet implements state.AppStorage.
func (s *StoragePermissionFacade) UpdateFooterPlugSet(ctx context.Context, id int64, name string, plugs []string) error {
	if err := CheckAdminAccess(ctx); err != nil {
		return err
	}
	return s.Storage.UpdateFooterPlugSet(ctx, id, name, plugs)
}

var _ state.AppStorage = &StoragePermissionFacade{}

func (s *StoragePermissionFacade) Close() {
	s.Storage.Close()
}

func (s *StoragePermissionFacade) CreateTournament(ctx context.Context, t *model.Tournament) (int64, error) {
	if err := CheckCreateTournamentAccess(ctx); err != nil {
		return -1, err
	}
	return s.Storage.CreateTournament(ctx, t)
}

func (s *StoragePermissionFacade) FetchStructureSlugs(ctx context.Context, offset, limit int) ([]*model.StructureSlug, error) {
	return s.Storage.FetchStructureSlugs(ctx, offset, limit)
}

func (s *StoragePermissionFacade) DeleteTournament(ctx context.Context, id int64) error {
	if err := CheckWriteAccessToTournamentID(ctx, id); err != nil {
		return err
	}
	return s.Storage.DeleteTournament(ctx, id)
}

func (s *StoragePermissionFacade) FetchOverview(ctx context.Context, offset, limit int) (*model.Overview, error) {
	return s.Storage.FetchOverview(ctx, offset, limit)
}

func (s *StoragePermissionFacade) FetchPlugs(ctx context.Context, id int64) (*model.FooterPlugs, error) {
	return s.Storage.FetchPlugs(ctx, id)
}

func (s *StoragePermissionFacade) FetchStructure(ctx context.Context, id int64) (*model.Structure, error) {
	return s.Storage.FetchStructure(ctx, id)
}

func (s *StoragePermissionFacade) FetchTournament(ctx context.Context, id int64) (*model.Tournament, error) {
	return s.Storage.FetchTournament(ctx, id)
}

func (s *StoragePermissionFacade) SaveTournament(ctx context.Context, m *model.Tournament) error {
	if err := CheckWriteAccessToTournamentID(ctx, m.EventID); err != nil {
		return err
	}
	return s.Storage.SaveTournament(ctx, m)
}

func (s *StoragePermissionFacade) SaveStructure(ctx context.Context, st *model.Structure) error {
	if err := CheckAdminAccess(ctx); err != nil {
		return err
	}
	return s.Storage.SaveStructure(ctx, st)
}

func (s *StoragePermissionFacade) DeleteStructure(ctx context.Context, id int64) error {
	if err := CheckAdminAccess(ctx); err != nil {
		return err
	}
	return s.Storage.DeleteStructure(ctx, id)
}

func (s *StoragePermissionFacade) CreateStructure(ctx context.Context, st *model.Structure) (int64, error) {
	if err := CheckAdminAccess(ctx); err != nil {
		return -1, err
	}
	return s.Storage.CreateStructure(ctx, st)
}

func (s *StoragePermissionFacade) ListenTournamentVersion(ctx context.Context, id int64, version int64, errCh chan<- error, tournamentCh chan<- *model.Tournament) {
	s.Storage.ListenTournamentVersion(ctx, id, version, errCh, tournamentCh)
}
