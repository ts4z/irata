package permission

import (
	"context"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
)

type StorageDecorator struct {
	Storage state.AppStorage
}

var _ state.AppStorage = &StorageDecorator{}

func (s *StorageDecorator) CreateTournament(ctx context.Context, t *model.Tournament) (int64, error) {
	if err := CheckCreateTournamentAccess(ctx); err != nil {
		return -1, err
	}
	return s.Storage.CreateTournament(ctx, t)
}

func (s *StorageDecorator) FetchStructureSlugs(ctx context.Context, offset, limit int) ([]*model.StructureSlug, error) {
	return s.Storage.FetchStructureSlugs(ctx, offset, limit)
}

func (s *StorageDecorator) DeleteTournament(ctx context.Context, id int64) error {
	if err := CheckWriteAccessToTournamentID(ctx, id); err != nil {
		return err
	}
	return s.Storage.DeleteTournament(ctx, id)
}

func (s *StorageDecorator) FetchOverview(ctx context.Context, offset, limit int) (*model.Overview, error) {
	return s.Storage.FetchOverview(ctx, offset, limit)
}

func (s *StorageDecorator) FetchPlugs(ctx context.Context, id int64) (*model.FooterPlugs, error) {
	return s.Storage.FetchPlugs(ctx, id)
}

func (s *StorageDecorator) FetchStructure(ctx context.Context, id int64) (*model.Structure, error) {
	return s.Storage.FetchStructure(ctx, id)
}

func (s *StorageDecorator) FetchTournament(ctx context.Context, id int64) (*model.Tournament, error) {
	return s.Storage.FetchTournament(ctx, id)
}

func (s *StorageDecorator) SaveTournament(ctx context.Context, m *model.Tournament) error {
	if err := CheckWriteAccessToTournamentID(ctx, m.EventID); err != nil {
		return err
	}
	return s.Storage.SaveTournament(ctx, m)
}

func (s *StorageDecorator) SaveStructure(ctx context.Context, st *model.Structure) error {
	if err := CheckAdminAccess(ctx); err != nil {
		return err
	}
	return s.Storage.SaveStructure(ctx, st)
}

func (s *StorageDecorator) DeleteStructure(ctx context.Context, id int64) error {
	if err := CheckAdminAccess(ctx); err != nil {
		return err
	}
	return s.Storage.DeleteStructure(ctx, id)
}

func (s *StorageDecorator) CreateStructure(ctx context.Context, st *model.Structure) (int64, error) {
	if err := CheckAdminAccess(ctx); err != nil {
		return -1, err
	}
	return s.Storage.CreateStructure(ctx, st)
}

func (s *StorageDecorator) ListenTournamentVersion(ctx context.Context, id int64, version int64, errCh chan<- error, tournamentCh chan<- *model.Tournament) {
	s.Storage.ListenTournamentVersion(ctx, id, version, errCh, tournamentCh)
}
