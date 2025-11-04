package listener

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
	"github.com/ts4z/irata/tournament"
)

// A listen request eventually results in exactly one write to one of these channels
// (possibly before the pair is constructed).
type channels struct {
	errCh        chan<- error
	tournamentCh chan<- *model.Tournament
}

// TournamentStorage provides a place to hang listeners.  This intercepts writes
// and notifies other interested listeners that the object has changed.
type TournamentStorage struct {
	tournamentListeners   map[int64][]channels
	tournamentListenersMu sync.Mutex

	mgr  *tournament.Manager
	next state.TournamentStorage
}

func NewTournamentStorage(storage state.TournamentStorage, mgr *tournament.Manager) *TournamentStorage {
	return &TournamentStorage{
		next:                storage,
		mgr:                 mgr,
		tournamentListeners: make(map[int64][]channels),
	}
}

var _ state.TournamentListenerStorage = (*TournamentStorage)(nil)

// CreateTournament implements state.TournamentListenerStorage.
func (s *TournamentStorage) CreateTournament(ctx context.Context, t *model.Tournament) (int64, error) {
	return s.next.CreateTournament(ctx, t)
}

// DeleteTournament implements state.TournamentListenerStorage.
func (s *TournamentStorage) DeleteTournament(ctx context.Context, id int64) error {
	err := s.next.DeleteTournament(ctx, id)
	if err != nil {
		return err
	}

	// Purge any active listeners.

	s.tournamentListenersMu.Lock()
	defer s.tournamentListenersMu.Unlock()

	listeners := s.resetTournamentListeners(id)

	for _, ch := range listeners {
		// Pass the updated tournament directly
		go func(chs channels) {
			chs.errCh <- fmt.Errorf("tournament %d has been deleted", id)
		}(ch)
	}
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

// ListenTournamentVersion implements state.TournamentListenerStorage.
func (s *TournamentStorage) ListenTournamentVersion(ctx context.Context, id int64, version int64, errCh chan<- error, tournamentCh chan<- *model.Tournament) {
	s.tournamentListenersMu.Lock()
	defer s.tournamentListenersMu.Unlock()

	t, err := s.next.FetchTournament(ctx, id)
	if err != nil {
		errCh <- fmt.Errorf("can't listen for changes: can't fetch %d: %v", id, err)
		return
	}

	if t.Version != version {
		// Database already has something different, just send it.
		if t.Version < version {
			// This is un-possible, but a malicious client could be messing with us,
			// or we could just have a bug.
			log.Printf("can't happen: reported version %d is newer than stored version %d for tournament %d", version, t.Version, id)
		}
		s.mgr.FillTransientsAndAdvanceClock(ctx, t)
		tournamentCh <- t
		return
	}

	log.Printf("listening for tournament %d changes from version %d", id, version)
	s.tournamentListeners[id] = append(s.tournamentListeners[id], channels{errCh, tournamentCh})
}

func (s *TournamentStorage) resetTournamentListeners(id int64) []channels {
	s.tournamentListenersMu.Lock()
	defer s.tournamentListenersMu.Unlock()
	listeners := s.tournamentListeners[id]
	delete(s.tournamentListeners, id)
	return listeners
}

// SaveTournament implements state.TournamentListenerStorage.
func (s *TournamentStorage) SaveTournament(ctx context.Context, t *model.Tournament) error {
	err := s.next.SaveTournament(ctx, t)
	if err != nil {
		return err
	}

	listeners := s.resetTournamentListeners(t.EventID)

	go func() {
		s.mgr.FillTransientsAndAdvanceClock(ctx, t)
		for _, chs := range listeners {
			// Pass the updated tournament directly
			chs.tournamentCh <- t.Clone()
		}
		log.Printf("notified %d listeners of tournament %d version %d change", len(listeners), t.EventID, t.Version)
	}()

	return err
}
