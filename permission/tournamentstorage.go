package permission

import (
	"context"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
)

type TournamentStorage struct {
	Storage state.TournamentListenerStorage
}

var _ state.TournamentStorage = &TournamentStorage{}

func (s *TournamentStorage) CreateTournament(ctx context.Context, t *model.Tournament) (int64, error) {
	if err := CheckCreateTournamentAccess(ctx); err != nil {
		return -1, err
	}
	return s.Storage.CreateTournament(ctx, t)
}

func (s *TournamentStorage) DeleteTournament(ctx context.Context, id int64) error {
	return requireOperator(ctx, func() error {
		return s.Storage.DeleteTournament(ctx, id)
	})
}

func (s *TournamentStorage) FetchOverview(ctx context.Context, offset, limit int) (*model.Overview, error) {
	return s.Storage.FetchOverview(ctx, offset, limit)
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

func (s *TournamentStorage) ListenTournamentVersion(ctx context.Context, id int64, version int64, errCh chan<- error, tournamentCh chan<- *model.Tournament) {
	s.Storage.ListenTournamentVersion(ctx, id, version, errCh, tournamentCh)
}
