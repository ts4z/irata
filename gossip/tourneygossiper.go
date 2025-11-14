package gossip

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/tournament"
)

// TournamentGossiper provides a tattletale for changes to tournaments.  Subscribers can either
// modify tournaments locally, or wait for the db notification to percolate back.
type TournamentGossiper struct {
	tournamentListeners   map[int64][]channels
	tournamentListenersMu sync.Mutex
	next                  CacheStorage[model.Tournament]
	mgr                   *tournament.Manager
}

func NewTournamentGossiper(next CacheStorage[model.Tournament], mgr *tournament.Manager) *TournamentGossiper {
	return &TournamentGossiper{
		tournamentListeners: make(map[int64][]channels),
		next:                next,
		mgr:                 mgr,
	}
}

func (s *TournamentGossiper) ListenTournamentVersion(ctx context.Context, id int64, version int64, errCh chan<- error, tournamentCh chan<- *model.Tournament) {
	s.tournamentListenersMu.Lock()
	defer s.tournamentListenersMu.Unlock()

	t, err := s.next.Fetch(ctx, id)
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

	log.Printf("gossiper: client listening for tournament %d changes from version %d", id, version)
	s.tournamentListeners[id] = append(s.tournamentListeners[id], channels{errCh, tournamentCh})
}

func (s *TournamentGossiper) resetTournamentListeners(id int64) []channels {
	s.tournamentListenersMu.Lock()
	defer s.tournamentListenersMu.Unlock()
	listeners := s.tournamentListeners[id]
	delete(s.tournamentListeners, id)
	return listeners
}

func (g *TournamentGossiper) NotifyUpdated(ctx context.Context, t *model.Tournament) {
	listeners := g.resetTournamentListeners(t.EventID)

	go func() {
		g.mgr.FillTransientsAndAdvanceClock(ctx, t)
		for _, chs := range listeners {
			// Pass the updated tournament directly
			chs.tournamentCh <- t.Clone()
		}
		log.Printf("notified %d listeners of tournament %d version %d change", len(listeners), t.EventID, t.Version)
	}()
}
