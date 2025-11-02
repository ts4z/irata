package state

// package state manages persistence.

import (
	"context"
	"time"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/paytable"
	"github.com/ts4z/irata/soundmodel"
)

type Closer interface {
	Close()
}

// AppStorage describes storage's view of state management.
type AppStorage interface {
	Closer

	FetchOverview(ctx context.Context, offset, limit int) (*model.Overview, error)

	CreateTournament(ctx context.Context, t *model.Tournament) (int64, error)
	SaveTournament(ctx context.Context, m *model.Tournament) error
	DeleteTournament(ctx context.Context, id int64) error
	FetchTournament(ctx context.Context, id int64) (*model.Tournament, error)
	ListenTournamentVersion(ctx context.Context, id int64, version int64, errCh chan<- error, tournamentCh chan<- *model.Tournament)

	FetchPlugs(ctx context.Context, id int64) (*model.FooterPlugs, error)

	// List all footer plug sets (metadata only, not plugs).
	ListFooterPlugSets(ctx context.Context) ([]*model.FooterPlugs, error)

	// Create a new footer plug set with a name and initial plugs.
	CreateFooterPlugSet(ctx context.Context, name string, plugs []string) (int64, error)

	// Update the name and plugs of a footer plug set.
	UpdateFooterPlugSet(ctx context.Context, id int64, name string, plugs []string) error

	// Delete a footer plug set and all its plugs.
	DeleteFooterPlugSet(ctx context.Context, id int64) error

	FetchStructure(ctx context.Context, id int64) (*model.Structure, error)
	FetchStructureSlugs(ctx context.Context, offset, limit int) ([]*model.StructureSlug, error)
	SaveStructure(ctx context.Context, s *model.Structure) error
	DeleteStructure(ctx context.Context, id int64) error
	CreateStructure(ctx context.Context, s *model.Structure) (int64, error)
}

type SiteStorage interface {
	Closer

	FetchSiteConfig(ctx context.Context) (*model.SiteConfig, error)
	SaveSiteConfig(ctx context.Context, config *model.SiteConfig) error
}

type UserStorage interface {
	Closer

	FetchUsers(ctx context.Context) ([]*model.UserIdentity, error)
	CreateUser(ctx context.Context, nick string, emailAddress string, passwordHash string, isAdmin bool) error
	FetchUserByUserID(ctx context.Context, id int64) (*model.UserIdentity, error)
	FetchUserRow(ctx context.Context, nick string) (*model.UserRow, error)
	DeleteUserByNick(ctx context.Context, nick string) error

	AddPassword(ctx context.Context, userID int64, passwordHash string) error
	RemoveExpiredPasswords(ctx context.Context, before time.Time) error
	ReplacePassword(ctx context.Context, userID int64, newPasswordHash string, oldPasswordsExpire time.Time) error
}

type PaytableStorage interface {
	Closer

	FetchPaytableByID(ctx context.Context, id int64) (*paytable.Paytable, error)
	FetchPaytableSlugs(ctx context.Context) ([]*paytable.PaytableSlug, error)
}

type SoundEffectStorage interface {
	Closer

	FetchSoundEffectByID(ctx context.Context, id int64) (*soundmodel.SoundEffect, error)
	FetchSoundEffectSlugs(ctx context.Context) ([]*soundmodel.SoundEffectSlug, error)
}
