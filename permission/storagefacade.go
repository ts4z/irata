package permission

import (
	"context"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
)

type StorageDecorator struct {
	Storage state.Storage
}

var _ state.Storage = &StorageDecorator{}

func (s *StorageDecorator) DeleteTournament(ctx context.Context, id int64) error {
	if err := CheckWriteAccessToTournamentID(ctx, id); err != nil {
		return err
	}
	return s.Storage.DeleteTournament(ctx, id)
}

func (s *StorageDecorator) FetchOverview(ctx context.Context) (*model.Overview, error) {
	return s.Storage.FetchOverview(ctx)
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
