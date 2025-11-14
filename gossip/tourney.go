package gossip

import (
	"context"
	"fmt"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
)

// A listen request eventually results in exactly one write to one of these channels
// (possibly before the pair is constructed).
type channels struct {
	errCh        chan<- error
	tournamentCh chan<- *model.Tournament
}

// TournamentStorage provides a place to hang listeners.  This intercepts writes
// and notifies other interested listeners that the object has changed.
//
// (If the database is notifying back, this is at best an optimization and perhaps
// redundant and may mask or create bugs.  In the best path, this saves a DB round-trip.
type TournamentStorage struct {
	gossiper *TournamentGossiper
	next     state.TournamentStorage
}

func NewTournamentStorage(storage state.TournamentStorage, g *TournamentGossiper) *TournamentStorage {
	return &TournamentStorage{
		next:     storage,
		gossiper: g,
	}
}

var _ state.TournamentStorage = (*TournamentStorage)(nil)

// CreateTournament implements state.TournamentListenerStorage.
func (s *TournamentStorage) CreateTournament(ctx context.Context, t *model.Tournament) (int64, error) {
	return s.next.CreateTournament(ctx, t)
}

func (g *TournamentGossiper) NotifyDeleted(ctx context.Context, id int64) {
	// Purge any active listeners.

	g.tournamentListenersMu.Lock()
	defer g.tournamentListenersMu.Unlock()

	listeners := g.resetTournamentListeners(id)

	for _, ch := range listeners {
		// Pass the updated tournament directly
		go func(chs channels) {
			chs.errCh <- fmt.Errorf("tournament %d has been deleted", id)
		}(ch)
	}

}

// DeleteTournament implements state.TournamentListenerStorage.
func (s *TournamentStorage) DeleteTournament(ctx context.Context, id int64) error {
	err := s.next.DeleteTournament(ctx, id)
	if err != nil {
		return err
	}

	s.gossiper.NotifyDeleted(ctx, id)

	return nil
}

// FetchOverview implements state.TournamentListenerStorage.
func (s *TournamentStorage) FetchOverview(ctx context.Context, offset int, limit int) (*model.Overview, error) {
	return s.next.FetchOverview(ctx, offset, limit)
}

// FetchTournament implements state.TournamentListenerStorage.
func (s *TournamentStorage) FetchTournament(ctx context.Context, id int64) (*model.Tournament, error) {
	return s.next.FetchTournament(ctx, id)
}

// SaveTournament implements state.TournamentListenerStorage.
func (s *TournamentStorage) SaveTournament(ctx context.Context, t *model.Tournament) error {
	err := s.next.SaveTournament(ctx, t)
	if err != nil {
		return err
	}

	s.gossiper.NotifyUpdated(ctx, t)
	return nil
}
