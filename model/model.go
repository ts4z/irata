// package model represents wire-protocol and database-stored models.
// (One of my previous employers says it's bad to write the same models
// to the database and the wire, but for this application, it's not so bad.)
//
// However, we have a lot of methods with calls to
// dependencies.  These should be moved elsewhere, as this class should
// have no dependencies.

package model

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/ts4z/irata/paytable"
)

const (
	// ServerVersion indicates an incompatible change to the client/server interaction.
	// If the client gets a different number than it originally got here, it should reload
	// to get a new copy of all server files.  This does not indicate any particular
	// compatibility problem.
	ServerVersion = 10
)

// TODO: Relocate.
type Clock interface {
	Now() time.Time
}

// TODO: Relocate.
type PaytableFetcher interface {
	FetchPaytableByID(ctx context.Context, id int64) (*paytable.Paytable, error)
}

// Dependencies for model methods.
//
// TODO: Relocate.  This is a set of configs for a TournamentUpdater delegate.
type Deps struct {
	Clock           Clock
	PaytableFetcher PaytableFetcher
}

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

// adjustStateForElapsedTime fixes the state to reflect the current time.
func (m *Tournament) adjustStateForElapsedTime(deps *Deps) {
	if m.CurrentLevel() == nil {
		m.RestartLastLevel(deps)
		return
	}

	if !m.State.IsClockRunning {
		if m.State.TimeRemainingMillis == nil {
			if m.CurrentLevel() != nil {
				log.Printf("BUG: clock is not running but TimeRemainingMillis is nil, resetting to full time")
				m.restartLevel(deps)
			} else {
				log.Printf("BUG: clock running, no time remaining, no level?")
				m.RestartLastLevel(deps)
			}
		}
		return
	}

	if m.State.CurrentLevelNumber < 0 {
		// wtf
		log.Printf("warning: current level number %d < 0, resetting to 0", m.State.CurrentLevelNumber)
		m.State.CurrentLevelNumber = 0
	}

	if m.State.CurrentLevelNumber >= len(m.Structure.Levels) {
		// wtf
		log.Printf("warning: current level number %d >= max %d, resetting to max-1", m.State.CurrentLevelNumber, len(m.Structure.Levels))
		m.State.CurrentLevelNumber = len(m.Structure.Levels) - 1
	}

	if m.State.CurrentLevelEndsAt == nil {
		log.Printf("BUG: clock is running but CurrentLevelEndsAt is nil, resetting to full time")
		later := deps.Clock.Now().Add(time.Duration(m.CurrentLevel().DurationMinutes) * time.Minute).UnixMilli()
		m.State.CurrentLevelEndsAt = &later
		m.State.TimeRemainingMillis = nil
		return
	}

	for m.CurrentLevel() != nil {
		endsAt := m.CurrentLevelEndsAtAsTime()
		if endsAt.After(deps.Clock.Now()) {
			// end of level still in the future!  we're good.
			break
		}

		// step the level forward, assuming no clock pauses.
		m.State.CurrentLevelNumber++
		if m.State.CurrentLevelNumber >= len(m.Structure.Levels) {
			m.endOfTime()
			return
		}
		newLevel := m.CurrentLevel()
		if newLevel == nil {
			m.RestartLastLevel(deps)
			break
		}

		levelDuration := time.Duration(newLevel.DurationMinutes) * time.Minute
		newEndsAt := endsAt.Add(levelDuration).UnixMilli()
		asInt64 := int64(newEndsAt)
		m.State.CurrentLevelEndsAt = &asInt64
	}
}

func (m *Tournament) RestartLastLevel(deps *Deps) {
	m.State.CurrentLevelNumber = len(m.Structure.Levels) - 1
	m.restartLevel(deps)
}

// FillTransients fills out computed fields.  (These shouldn't be serialized to
// the database as they're redundant, but they are very convenient for access
// from templates and maybe JS.)
func (m *Tournament) FillTransients(deps *Deps) {
	m.Transients = &Transients{
		ServerVersion: ServerVersion,
	}

	if m.State.TotalChipsOverride > 0 {
		m.Transients.TotalChips = m.State.TotalChipsOverride
	} else {
		m.Transients.TotalChips = m.State.BuyIns*m.Structure.ChipsPerBuyIn + m.State.AddOns*m.Structure.ChipsPerAddOn
	}

	if m.State.CurrentPlayers == 0 {
		m.Transients.AverageChips = 0
	} else {
		m.Transients.AverageChips = int(math.Round(float64(m.Transients.TotalChips) / float64(m.State.CurrentPlayers)))
	}

	m.adjustStateForElapsedTime(deps)

	if deps.PaytableFetcher != nil && m.State.AutoComputePrizePool {
		if ppt, err := m.ComputePrizePoolText(deps.PaytableFetcher); err == nil {
			m.State.PrizePool = ppt
		}
	}
}

func (m *Tournament) PreviousLevel(deps *Deps) error {
	if m.State.CurrentLevelNumber <= 0 {
		return errors.New("already at min level")
	}
	m.State.CurrentLevelNumber--
	m.restartLevel(deps)
	return nil
}

func (m *Tournament) endOfTime() {
	log.Printf("tournament %d at end of time", m.EventID)
	zero := int64(0)
	m.State.CurrentLevelNumber = len(m.Structure.Levels) - 1
	m.State.TimeRemainingMillis = &zero
	m.State.CurrentLevelEndsAt = nil
	m.State.IsClockRunning = false
}

func (m *Tournament) SetLevelRemaining(deps *Deps, d time.Duration) {
	if m.State.IsClockRunning {
		millis := deps.Clock.Now().Add(d).UnixMilli()
		m.State.CurrentLevelEndsAt = &millis
	} else {
		remainingMillis := d.Milliseconds()
		m.State.TimeRemainingMillis = &remainingMillis
	}
}

func (m *Tournament) AdvanceLevel(deps *Deps) error {
	if m.State.CurrentLevelNumber >= len(m.Structure.Levels)-1 {
		m.endOfTime()
		return nil
	}

	m.State.CurrentLevelNumber++
	m.restartLevel(deps)
	return nil
}

func (m *Tournament) CurrentLevelDuration() *time.Duration {
	if m.CurrentLevel() == nil {
		return nil
	}
	d := time.Duration(m.CurrentLevel().DurationMinutes) * time.Minute
	return &d
}

// restartLevel resets the current level's clocks after a manual level change.
// (It doesn't make sense to call this externally.)
func (m *Tournament) restartLevel(deps *Deps) {
	if m.CurrentLevel() == nil {
		log.Printf("debug: can't restart level: no current level")
	}
	minutes := m.CurrentLevel().DurationMinutes
	d := time.Duration(minutes) * time.Minute
	if m.State.IsClockRunning {
		later := deps.Clock.Now().Add(d).UnixMilli()
		m.State.CurrentLevelEndsAt = &later
		m.State.TimeRemainingMillis = nil
	} else {
		remainingMillis := int64(d.Milliseconds())
		m.State.TimeRemainingMillis = &remainingMillis
		m.State.CurrentLevelEndsAt = nil
	}
}

func (m *Tournament) StopClock(deps *Deps) error {
	log.Printf("stop clock request for tournament %d", m.EventID)
	m.adjustStateForElapsedTime(deps)

	if !m.State.IsClockRunning {
		log.Printf("debug: can't stop a stopped clock")
		return nil
	}

	if m.CurrentLevel() == nil {
		return errors.New("can't stop clock: no current level")
	}

	endsAt := m.CurrentLevelEndsAtAsTime()
	remainingMillis := endsAt.Sub(deps.Clock.Now()).Milliseconds()

	m.State.IsClockRunning = false
	m.State.TimeRemainingMillis = &remainingMillis
	m.State.CurrentLevelEndsAt = nil
	return nil
}

func (m *Tournament) StartClock(deps *Deps) error {
	m.adjustStateForElapsedTime(deps)

	if m.State.IsClockRunning {
		log.Printf("debug: can't start a started clock")
		return nil
	}

	if m.CurrentLevel() == nil {
		log.Printf("debug: can't start a clock with no current level")
		return errors.New("can't start a clock with no current level")
	}

	var remaining time.Duration
	if m.State.TimeRemainingMillis != nil {
		remaining = time.Duration(*m.State.TimeRemainingMillis) * time.Millisecond
	} else {
		log.Printf("debug: when starting clock, no TimeRemainingMillis, using full level duration")
		remaining = *m.CurrentLevelDuration()
	}

	endsAt := deps.Clock.Now().Add(remaining).UnixMilli()
	m.State.CurrentLevelEndsAt = &endsAt
	m.State.TimeRemainingMillis = nil
	m.State.IsClockRunning = true
	return nil
}

func (m *Tournament) ChangePlayers(deps *Deps, n int) error {
	m.State.CurrentPlayers += n
	if m.State.CurrentPlayers < 1 {
		m.State.CurrentPlayers = 1
	}
	m.FillTransients(deps)
	return nil
}

func (m *Tournament) ChangeBuyIns(deps *Deps, n int) error {
	m.State.BuyIns += n
	if m.State.BuyIns < 1 {
		m.State.BuyIns = 1
	}
	m.FillTransients(deps)
	return nil
}

func (m *Tournament) ChangeAddOns(deps *Deps, n int) error {
	m.State.AddOns += n
	if m.State.AddOns < 1 {
		m.State.AddOns = 0
	}
	m.FillTransients(deps)
	return nil
}

func (m *Tournament) PlusTime(deps *Deps, d time.Duration) error {
	m.adjustStateForElapsedTime(deps)

	if m.CurrentLevel() == nil {
		return errors.New("can't add a minute: no current level")
	}

	if m.State.IsClockRunning {
		newEndsAt := m.CurrentLevelEndsAtAsTime().Add(d)
		asInt64 := newEndsAt.UnixMilli()
		m.State.CurrentLevelEndsAt = &asInt64
		m.State.TimeRemainingMillis = nil
	} else {
		var remaining time.Duration
		if m.State.TimeRemainingMillis != nil {
			remaining = time.Duration(*m.State.TimeRemainingMillis) * time.Millisecond
		} else {
			log.Printf("debug: when adding a minute, no TimeRemainingMillis, using full level duration")
			remaining = *m.CurrentLevelDuration()
		}

		remaining += d
		remainingMillis := remaining.Milliseconds()

		m.State.TimeRemainingMillis = &remainingMillis
		m.State.CurrentLevelEndsAt = nil
	}

	m.FillTransients(deps)

	return nil
}

func (m *Tournament) MinusTime(deps *Deps, d time.Duration) error {
	if d < 0 {
		log.Fatalf("can't happen: MinusTime with negative duration %v", d)
	}

	m.adjustStateForElapsedTime(deps)

	if m.CurrentLevel() == nil {
		return errors.New("can't add a minute: no current level")
	}

	if m.State.IsClockRunning {
		newEndsAt := m.CurrentLevelEndsAtAsTime().Add(-d)
		asInt64 := newEndsAt.UnixMilli()
		m.State.CurrentLevelEndsAt = &asInt64
		m.State.TimeRemainingMillis = nil

		// special case: if there was less than a minute left and we just
		// bumped to the next level, we just start the next level as normal.
		if newEndsAt.Before(deps.Clock.Now()) {
			// Skip to next level, which should reset it (or end the tournamment).
			m.AdvanceLevel(deps)
			return nil
		}
	} else {
		var remaining time.Duration
		if m.State.TimeRemainingMillis != nil {
			remaining = time.Duration(*m.State.TimeRemainingMillis) * time.Millisecond
		} else {
			log.Printf("debug: when adding a minute, no TimeRemainingMillis, using full level duration")
			remaining = *m.CurrentLevelDuration()
		}

		remaining -= d

		if int64(remaining) < 0 {
			// special case: if we just exhausted this level, go to the next level
			// and give it a full time allotment.
			m.AdvanceLevel(deps)
			return nil
		} else {
			newRemainingMillis := remaining.Milliseconds()
			m.State.TimeRemainingMillis = &newRemainingMillis
			m.State.CurrentLevelEndsAt = nil
		}
	}

	m.FillTransients(deps)

	return nil
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

// ComputePrizePoolText calculates the prize pool distribution and returns
// a formatted text block suitable for display in the PrizePool textarea.
// Returns an error if the paytable is nil or if the calculation fails.
func (m *Tournament) ComputePrizePoolText(ptf PaytableFetcher) (string, error) {
	pt, err := ptf.FetchPaytableByID(context.Background(), m.PaytableID)
	if err != nil {
		return "", fmt.Errorf("while regenerating pay table: failed to fetch paytable: %w", err)
	}

	// Calculate total prize pool
	totalPrizePool := m.TotalPrizePool()

	// Calculate total prize pool less saves
	savesAmount := m.State.AmountPerSave * m.State.Saves
	totalPrizePoolLessSaves := totalPrizePool - savesAmount

	if totalPrizePoolLessSaves <= 0 {
		return "", errors.New("total prize pool less saves must be positive")
	}

	// Use number of buy-ins (not current players) for payout calculation
	numBuyIns := m.State.BuyIns
	if numBuyIns <= 0 {
		return "", errors.New("number of buy-ins must be positive")
	}

	// Get the prize distribution from the paytable
	prizes, err := pt.Payout(totalPrizePoolLessSaves, numBuyIns)
	if err != nil {
		return "", fmt.Errorf("failed to calculate payout: %w", err)
	}

	// Format the output
	var lines []string

	// Add main prizes
	for i, prize := range prizes {
		place := i + 1
		placeStr := formatPlace(place)
		lines = append(lines, fmt.Sprintf("%s: $%d", placeStr, prize))
	}

	// Add saves if any
	if m.State.Saves > 0 {
		firstSave := len(prizes) + 1
		lastSave := firstSave + m.State.Saves - 1
		if firstSave == lastSave {
			nth := formatPlace(firstSave)
			lines = append(lines, fmt.Sprintf("%s: $%d*", nth, m.State.AmountPerSave))
		} else {
			lastth := formatPlace(lastSave)
			lines = append(lines, fmt.Sprintf("%d-%s: $%d*", firstSave, lastth, m.State.AmountPerSave))
		}
		lines = append(lines, "* save")
	}

	return strings.Join(lines, "\n"), nil
}

// formatPlace converts a numeric place (1, 2, 3, ...) to a string ("1st", "2nd", "3rd", ...).
func formatPlace(place int) string {
	suffix := "th"
	if place%100 < 11 || place%100 > 13 {
		switch place % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", place, suffix)
}
