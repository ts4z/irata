package state

// package state manages persistence.

import (
	"context"

	"github.com/ts4z/irata/model"
)

// Storage describes storage's view of state management.
type Storage interface {
	FetchOverview(ctx context.Context) (*model.Overview, error)
	FetchTournament(ctx context.Context, id int64) (*model.Tournament, error)
	FetchPlugs(ctx context.Context, id int64) (*model.FooterPlugs, error)
	FetchStructure(ctx context.Context, id int64) (*model.Structure, error)
	SaveTournament(ctx context.Context, m *model.Tournament) error
	DeleteTournament(ctx context.Context, id int64) error
}
