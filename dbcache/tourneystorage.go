package dbcache

import (
	"context"
	"log"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
	"github.com/ts4z/irata/varz"
)

var (
	tournamentStorageCacheHits   = varz.NewInt("tournamentStoragecacheHits")
	tournamentStorageCacheMisses = varz.NewInt("tournamentStoragecacheMisses")
)

type TournamentStorage struct {
	cache *lru.Cache[int64, *model.Tournament]
	lock  sync.Mutex
	next  state.TournamentStorage
}

// CreateTournament implements state.TournamentStorage.
func (s *TournamentStorage) CreateTournament(ctx context.Context, t *model.Tournament) (int64, error) {
	return s.next.CreateTournament(ctx, t)
}

// DeleteTournament implements state.TournamentStorage.
func (s *TournamentStorage) DeleteTournament(ctx context.Context, id int64) error {
	err := s.next.DeleteTournament(ctx, id)
	if err != nil {
		s.cache.Remove(id)
	}
	return err
}

// FetchOverview implements state.TournamentStorage.
func (s *TournamentStorage) FetchOverview(ctx context.Context, offset int, limit int) (*model.Overview, error) {
	return s.next.FetchOverview(ctx, offset, limit)
}

// FetchTournament implements state.TournamentStorage.
func (s *TournamentStorage) FetchTournament(ctx context.Context, id int64) (*model.Tournament, error) {
	if t, ok := s.cache.Get(id); ok {
		tournamentStorageCacheHits.Add(1)
		return t, nil
	}

	tournamentStorageCacheMisses.Add(1)
	t, err := s.next.FetchTournament(ctx, id)
	if err != nil {
		return nil, err
	}
	s.cache.Add(id, t)
	return t, nil
}

// SaveTournament implements state.TournamentStorage.
func (s *TournamentStorage) SaveTournament(ctx context.Context, m *model.Tournament) error {
	err := s.next.SaveTournament(ctx, m)
	if err != nil {
		return err
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	oldVersion := int64(0)
	if cached, ok := s.cache.Get(m.EventID); ok {
		oldVersion = cached.Version
	}
	if oldVersion < m.Version {
		s.cache.Add(m.EventID, m)
	}
	s.cache.Add(m.EventID, m)
	return nil
}

var _ state.TournamentStorage = (*TournamentStorage)(nil)

func NewTournamentStorage(size int, next state.TournamentStorage) *TournamentStorage {
	cache, err := lru.New[int64, *model.Tournament](size)
	if err != nil {
		log.Fatalf("Failed to create TournamentStorage cache: %v", err)
	}
	return &TournamentStorage{
		cache: cache,
		next:  next,
	}
}
