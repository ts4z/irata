// package model represents wire-protocol and database-stored models.
// (One of my previous employers says it's bad to write the same models
// to the database and the wire, but for this application, it's not so bad.)
//
// Methods in here should probably be relocated, generally to a package like
// `tournament`.

package model

import (
	"time"
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
	ID         int64
	Nick       string
	IsAdmin    bool
	IsOperator bool
}

func (ui *UserIdentity) Clone() *UserIdentity {
	new := *ui
	return &new
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
	Name                    string
	CookieDomain            string
	AllowedOriginDomains    []string
	BonusHTTPPorts          []int
	BonusHTTPSPorts         []int
	Theme                   string
	DefaultNextLevelSoundID int64
	CookieKeys              []CookieKeyPair
}

type Level struct {
	AutoPause       bool
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

func (fp *FooterPlugs) Clone() *FooterPlugs {
	new := *fp
	new.TextPlugs = make([]string, len(fp.TextPlugs))
	copy(new.TextPlugs, fp.TextPlugs)
	return &new
}

// Tournaments are the things that we're running.
type Tournament struct {
	EventID int64 // TODO: rename to TournamentID
	Version int64

	EventName        string
	Description      string
	FooterPlugsID    int64
	NextLevelSoundID int64

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
	Name          string
	ID            int64
	ChipsPerBuyIn int
	ChipsPerAddOn int
}

// State represents the mutable state of a tournament (stuff that is supposed to
// change during the tournament).  If we clone a tournament, we don't clone this part.
//
// This is distinct from Transients, which are computed from State and Structure.
// This is serialized to the database but changes frequently.
type State struct {
	SoundMuted         bool
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
// (Transients should be split out of Tournament entirely.  When we fetch a model.Tournament,
// these should arrive with it, but shouldn't be stored in that model.)
type Transients struct {
	ProtocolVersion int
	ServerVersion   int
	TotalChips      int
	AverageChips    int

	// Semi-stopgap.  We want the URL path to the sound file, but we store only the
	// sound ID in the model, which is useless to the client.  So we'll fetch it as
	// part of transients, which is currently quite cheap.
	NextLevelSoundPath string
}

// TODO: Move to tournament/tm.go.
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
