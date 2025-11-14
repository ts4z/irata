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
	tournamentStorageCacheHits            = varz.NewInt("tournamentStoragecacheHits")
	tournamentStorageCacheMisses          = varz.NewInt("tournamentStoragecacheMisses")
	tournamentStorageCacheDuplicateUpdate = varz.NewInt("tournamentStoragecacheDuplicateUpdate")
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

// Alternate name, making this suitable for dbnotify CacheStorage interface.
// (This is probably a better name, since we already know we're talking about tournaments.)
func (s *TournamentStorage) Fetch(ctx context.Context, id int64) (*model.Tournament, error) {
	return s.FetchTournament(ctx, id)
}

func (s *TournamentStorage) CacheInvalidate(_ context.Context, id int64, version int64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if t, ok := s.cache.Get(id); ok {
		if t.Version <= version {
			s.cache.Remove(id)
		}
	}
}

func (s *TournamentStorage) CacheStore(ctx context.Context, t *model.Tournament) {
	id := t.EventID
	s.lock.Lock()
	defer s.lock.Unlock()
	cached, ok := s.cache.Get(id)
	if ok {

		if cached.Version > t.Version {
			log.Printf("cache: have version %d, incoming %d, ignoring", cached.Version, t.Version)
			return
		} else if cached.Version == t.Version {
			tournamentStorageCacheDuplicateUpdate.Add(1)
			log.Printf("cache: already have version %d, ignoring", cached.Version)
			return
		}
	}
	s.cache.Add(id, t)
}

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
	log.Printf("cache store from fetch tournament id=%d version=%d", t.EventID, t.Version)
	s.CacheStore(ctx, t)
	return t, nil
}

func (s *TournamentStorage) SaveTournament(ctx context.Context, m *model.Tournament) error {
	err := s.next.SaveTournament(ctx, m)
	if err != nil {
		return err
	}
	log.Printf("cache store from save tournament id=%d version=%d", m.EventID, m.Version)
	s.CacheStore(ctx, m)
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
