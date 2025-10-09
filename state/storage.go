package state

// package state manages persistence.

import (
	"context"

	"github.com/ts4z/irata/model"
)

// Storage describes storage's view of state management.
type Storage interface {
	FetchSiteConfig(ctx context.Context) (*model.SiteConfig, error)

	FetchOverview(ctx context.Context, offset, limit int) (*model.Overview, error)

	CreateTournament(ctx context.Context, t *model.Tournament) (int64, error)
	SaveTournament(ctx context.Context, m *model.Tournament) error
	DeleteTournament(ctx context.Context, id int64) error
	FetchTournament(ctx context.Context, id int64) (*model.Tournament, error)

	FetchPlugs(ctx context.Context, id int64) (*model.FooterPlugs, error)

	FetchStructure(ctx context.Context, id int64) (*model.Structure, error)
	FetchStructureSlugs(ctx context.Context, offset, limit int) ([]*model.StructureSlug, error)
	SaveStructure(ctx context.Context, s *model.Structure) error
	DeleteStructure(ctx context.Context, id int64) error
	CreateStructure(ctx context.Context, s *model.Structure) (int64, error)
}
