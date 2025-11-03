package permission

import (
	"context"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
)

type AppStorage struct {
	Storage state.AppStorage
}

type TournamentStorage struct {
	Storage state.TournamentStorage
}

var _ state.AppStorage = &AppStorage{}
var _ state.TournamentStorage = &TournamentStorage{}

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

func (s *TournamentStorage) CreateTournament(ctx context.Context, t *model.Tournament) (int64, error) {
	if err := CheckCreateTournamentAccess(ctx); err != nil {
		return -1, err
	}
	return s.Storage.CreateTournament(ctx, t)
}

func (s *AppStorage) FetchStructureSlugs(ctx context.Context, offset, limit int) ([]*model.StructureSlug, error) {
	return s.Storage.FetchStructureSlugs(ctx, offset, limit)
}

func (s *TournamentStorage) DeleteTournament(ctx context.Context, id int64) error {
	if err := CheckWriteAccessToTournamentID(ctx, id); err != nil {
		return err
	}
	return s.Storage.DeleteTournament(ctx, id)
}

func (s *TournamentStorage) FetchOverview(ctx context.Context, offset, limit int) (*model.Overview, error) {
	return s.Storage.FetchOverview(ctx, offset, limit)
}

func (s *AppStorage) FetchPlugs(ctx context.Context, id int64) (*model.FooterPlugs, error) {
	return s.Storage.FetchPlugs(ctx, id)
}

func (s *AppStorage) FetchStructure(ctx context.Context, id int64) (*model.Structure, error) {
	return s.Storage.FetchStructure(ctx, id)
}

func (s *TournamentStorage) FetchTournament(ctx context.Context, id int64) (*model.Tournament, error) {
	return s.Storage.FetchTournament(ctx, id)
}

func (s *TournamentStorage) SaveTournament(ctx context.Context, m *model.Tournament) error {
	if err := CheckWriteAccessToTournamentID(ctx, m.EventID); err != nil {
		return err
	}
	return s.Storage.SaveTournament(ctx, m)
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

func (s *TournamentStorage) ListenTournamentVersion(ctx context.Context, id int64, version int64, errCh chan<- error, tournamentCh chan<- *model.Tournament) {
	s.Storage.ListenTournamentVersion(ctx, id, version, errCh, tournamentCh)
}
