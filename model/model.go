// package model represents wire-protocol and database-stored models.
// (One of my previous employers says it's bad to write the same models
// to the database and the wire, but for this application, it's not so bad.)
//
// However, we have a lot of methods with calls to
// dependencies.  These should be moved elsewhere, as this class should
// have no dependencies.

package model

import (
	"time"
)

const (
	// ServerVersion indicates an incompatible change to the client/server interaction.
	// If the client gets a different number than it originally got here, it should reload
	// to get a new copy of all server files.  This does not indicate any particular
	// compatibility problem.
	ServerVersion = 10
)

type AuthCookieData struct {
	RealUserID      int64
	EffectiveUserID int64
}

type Password struct {
	CreatedAt    time.Time
	ExpiresAt    *time.Time
	PasswordHash string
}

type UserRow struct {
	UserIdentity
	Passwords []Password
}

type UserIdentity struct {
	ID      int64
	Nick    string
	IsAdmin bool
}

type CookieKeyValidity struct {
	MintFrom   time.Time
	MintUntil  time.Time
	HonorUntil time.Time
}

type CookieKeyPair struct {
	Validity   CookieKeyValidity
	HashKey64  string
	BlockKey64 string
}

type SiteConfig struct {
	Name       string
	Site       string
	Theme      string
	CookieKeys []CookieKeyPair
}

type Level struct {
	Banner          string
	Description     string
	DurationMinutes int // TODO: convert this to a string?
	IsBreak         bool
}

// FooterPlugs is possible values for decorating the footer.
//
// This is intended to support things other than text, but that's not
// implemented yet.
type FooterPlugs struct {
	FooterPlugsID int64
	Version       int64
	Name          string
	TextPlugs     []string
}

// Tournaments are the things that we're running.
type Tournament struct {
	EventID int64 // TODO: rename to TournamentID
	Version int64

	EventName     string
	Handle        string // datbase unique key
	Description   string
	FooterPlugsID int64

	PrizePoolPerBuyIn int // amount to prize pool per buy-in
	PrizePoolPerAddOn int // amount to prize pool per add-on

	PaytableID      int64 // ID of the paytable to use for prize pool calculation
	FromStructureID int64 // ID of the structure this was denormalized from
	Structure       StructureData

	State      *State
	Transients *Transients
}

func (m *Tournament) Clone() *Tournament {
	new := *m

	if m.State != nil {
		new.State = m.State.Clone()
	}

	// Don't store transients; caller can re-fill them.

	return &new
}

// Data for a structure.  Embedded in Structure and referenced in Tournament.
type StructureData struct {
	Levels        []*Level
	ChipsPerBuyIn int
	ChipsPerAddOn int
}

// Strucutre describes the structure of a tournament.
type Structure struct {
	StructureData
	Name    string
	ID      int64
	Version int64
}

func (old *StructureData) Clone() *StructureData {
	new := *old
	new.Levels = make([]*Level, len(old.Levels))
	for i, lvl := range old.Levels {
		newLvl := *lvl
		new.Levels[i] = &newLvl
	}
	return &new
}

type StructureSlug struct {
	Name string
	ID   int64
}

// State represents the mutable state of a tournament (stuff that is supposed to
// change during the tournament).  If we clone a tournament, we don't clone this part.
//
// This is distinct from Transients, which are computed from State and Structure.
// This is serialized to the database but changes frequently.
type State struct {
	IsClockRunning     bool
	CurrentLevelNumber int
	CurrentPlayers     int
	BuyIns             int
	AddOns             int
	Saves              int
	AmountPerSave      int

	TotalChipsOverride     int // if > 0, overrides computed total chips
	TotalPrizePoolOverride int // if > 0, overrides computed prize pool

	AutoComputePrizePool bool
	PrizePool            string // right-hand side display, usually (but not always) the prize pool

	// EndsAt indicates when the level ends iff the clock is running.  This is in
	// Unix millis.  This value is not useful if the current level is paused, because
	// we don't know when the clock will be un-frozen.
	CurrentLevelEndsAt *int64
	// TimeRemaining indicates time remaining iff the clock is not running (that
	// is, paused).  This is in Unix millis.  This can always be initialized
	// within a level.
	TimeRemainingMillis *int64
}

func (s *State) Clone() *State {
	new := *s
	return &new
}

// Transients are computed from State and Structure, and are not serialized to the database.
type Transients struct {
	ServerVersion int
	TotalChips    int
	AverageChips  int
}

func (m *Tournament) PrizePoolAmount() int {
	if m.State.TotalPrizePoolOverride > 0 {
		return m.State.TotalPrizePoolOverride
	}

	buyIns := m.PrizePoolPerBuyIn * m.State.BuyIns
	addOns := m.PrizePoolPerAddOn * m.State.AddOns
	saves := m.State.AmountPerSave * m.State.Saves

	return buyIns + addOns - saves
}

// Current level returns the current level, or if the tourn
func (m *Tournament) CurrentLevel() *Level {
	var lvl int = m.State.CurrentLevelNumber
	if lvl < 0 {
		lvl = 0
	} else if lvl >= len(m.Structure.Levels) {
		lvl = len(m.Structure.Levels) - 1
	}
	if len(m.Structure.Levels) == 0 {
		return nil
	}
	return m.Structure.Levels[lvl]
}

func (m *Tournament) CurrentLevelEndsAtAsTime() time.Time {
	if m.State.CurrentLevelEndsAt == nil {
		panic("can't get CurrentLevelEndsAtAsTime: CurrentLevelEndsAt is nil")
	}
	return time.UnixMilli(*m.State.CurrentLevelEndsAt)
}

func (m *Tournament) CurrentLevelDuration() *time.Duration {
	if m.CurrentLevel() == nil {
		return nil
	}
	d := time.Duration(m.CurrentLevel().DurationMinutes) * time.Minute
	return &d
}

// TournamentSlug describes a single event for rendering the event list.
type TournamentSlug struct {
	TournamentID   int64
	TournamentName string
	Description    string
	// buyin, host, location, etc.
}

// Overview describes the available events for the event list.
type Overview struct {
	Slugs []TournamentSlug
}

func (m *Tournament) TotalPrizePool() int {
	if m.State.TotalPrizePoolOverride > 0 {
		return int(m.State.TotalPrizePoolOverride)
	} else {
		buyIns := m.PrizePoolPerBuyIn * m.State.BuyIns
		addOns := m.PrizePoolPerAddOn * m.State.AddOns
		return buyIns + addOns
	}
}
