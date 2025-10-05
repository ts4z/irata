package fakes

import (
	"sync"

	"github.com/ts4z/irata/defaults"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/model"
)

type FakeStorage struct {
	rw    sync.Mutex
	cache map[int64]*model.Tournament
}

func NewFakeStorage() *FakeStorage {
	return &FakeStorage{
		rw: sync.Mutex{},
		cache: map[int64]*model.Tournament{
			1: makeFakeTournament(),
		},
	}
}

func makeFakeTournament() *model.Tournament {
	// Not implemented, return dummy
	m := &model.Tournament{
		EventID:   1,
		EventName: "Friday $40 NLHE",
		State: &model.State{
			IsClockRunning:     false,
			CurrentLevelNumber: 0,
			CurrentPlayers:     12,
			BuyIns:             12,
			PrizePool: `1.......$240
2........$72
3........$48
`,
		},
		Structure: defaults.Structure(),
	}

	m.FillTransients()

	return m
}

func (s *FakeStorage) Lock() func() {
	s.rw.Lock()
	return func() { s.rw.Unlock() }
}

func (s *FakeStorage) FetchTournament(id int64) (*model.Tournament, error) {
	unlock := s.Lock()
	defer unlock()
	if t, ok := s.cache[id]; ok {
		t.FillTransients()
		return t, nil
	} else {
		return nil, he.HTTPCodedErrorf(404, "tournament id %d not found", id)
	}
}

func (s *FakeStorage) FetchOverview() (*model.Overview, *he.HTTPError) {
	unlock := s.Lock()
	defer unlock()
	events := []model.TournamentSlug{}
	for id, t := range s.cache {
		events = append(events, model.TournamentSlug{
			TournamentID:   id,
			TournamentName: t.EventName,
		})
	}
	return &model.Overview{
		Slugs: events,
	}, nil
}

func (s *FakeStorage) SaveTournament(m *model.Tournament) error {
	unlock := s.Lock()
	defer unlock()

	cpy := model.Tournament{}
	cpy = *m
	s.cache[m.EventID] = &cpy

	return nil
}
