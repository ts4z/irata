package model

import (
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	millisPerMinute = 60 * 1000
)

var (
	dashDashRE = regexp.MustCompile(`\s*--\s*`)
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

func parseLevelBreak(s string) bool {
	if strings.EqualFold(s, "BREAK") {
		return true
	} else {
		return false
	}
}

// ParseLevels is for parsing levels from an old text input;
// it is not clear to me if it needs to exist now.
//
// level 0 is *always* a break.
func ParseLevels(input string) ([]*Level, error) {
	levels := []*Level{}
	lines := strings.Split(input, "\n")
	for _, line := range lines {

		parts := dashDashRE.Split(line, 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("line unparsable: %q", line)
		}
		durationMins, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("can't parse duration in line %q: %w", line, err)
		}
		isBreak := parseLevelBreak(parts[1])
		description := parts[2]
		levels = append(levels, &Level{
			Banner:          description,
			DurationMinutes: durationMins,
			IsBreak:         isBreak,
			Description:     description,
		})
	}
	if len(levels) < 2 {
		return nil, fmt.Errorf("need at least two levels, got %d", len(levels))
	}
	levels[0].IsBreak = true
	return levels, nil
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

	Structure  *StructureData
	State      *State
	Transients *Transients
}

func (old *Tournament) Clone() *Tournament {
	new := *old

	if old.Structure != nil {
		new.Structure = old.Structure.Clone()
	}
	if old.State != nil {
		new.State = old.State.Clone()
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

// State represents the mutable state of a tournament.
//
// This is distinct from Transients, which are computed from State and Structure.
// This is serialized to the database but changes frequently.
type State struct {
	IsClockRunning     bool
	CurrentLevelNumber int
	CurrentPlayers     int
	BuyIns             int
	AddOns             int
	TotalChipsOverride int    // if > 0, overrides computed total chips
	PrizePool          string // right-hand side display

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
	TotalChips   int
	AverageChips int
	// NextBreakAt is the time the next break starts, in Unix millis, or nil if there are no more breaks.
	NextBreakAt *int64
	// NextLevel is the next non-break level, or nil if there are no more levels.
	NextLevel *Level
}

// Current level returns the current level, or if the tourn
func (old *Tournament) CurrentLevel() *Level {
	var lvl int = old.State.CurrentLevelNumber
	if lvl < 0 {
		lvl = 0
	} else if lvl >= len(old.Structure.Levels) {
		lvl = len(old.Structure.Levels) - 1
	}
	return old.Structure.Levels[lvl]
}

func (old *Tournament) CurrentLevelEndsAtAsTime() time.Time {
	if old.State.CurrentLevelEndsAt == nil {
		panic("can't get CurrentLevelEndsAtAsTime: CurrentLevelEndsAt is nil")
	}
	return time.UnixMilli(*old.State.CurrentLevelEndsAt)
}

// adjustStateForElapsedTime fixes the state to reflect the current time.
func (old *Tournament) adjustStateForElapsedTime(clock Clock) {
	if old.CurrentLevel() == nil {
		old.RestartLastLevel(clock)
		return
	}

	if !old.State.IsClockRunning {
		if old.State.TimeRemainingMillis == nil {
			if old.CurrentLevel() != nil {
				log.Printf("BUG: clock is not running but TimeRemainingMillis is nil, resetting to full time")
				old.restartLevel(clock)
			} else {
				log.Printf("BUG: clock running, no time remaining, no level?")
				old.RestartLastLevel(clock)
			}
		}
		return
	}

	if old.State.CurrentLevelNumber < 0 {
		// wtf
		log.Printf("warning: current level number %d < 0, resetting to 0", old.State.CurrentLevelNumber)
		old.State.CurrentLevelNumber = 0
	}

	if old.State.CurrentLevelNumber >= len(old.Structure.Levels) {
		// wtf
		log.Printf("warning: current level number %d >= max %d, resetting to max-1", old.State.CurrentLevelNumber, len(old.Structure.Levels))
		old.State.CurrentLevelNumber = len(old.Structure.Levels) - 1
	}

	if old.State.CurrentLevelEndsAt == nil {
		log.Printf("BUG: clock is running but CurrentLevelEndsAt is nil, resetting to full time")
		later := clock.Now().Add(time.Duration(old.CurrentLevel().DurationMinutes) * time.Minute).UnixMilli()
		old.State.CurrentLevelEndsAt = &later
		old.State.TimeRemainingMillis = nil
		return
	}

	for old.CurrentLevel() != nil {
		endsAt := old.CurrentLevelEndsAtAsTime()
		if endsAt.After(clock.Now()) {
			// end of level still in the future!  we're good.
			break
		}

		// step the level forward, assuming no clock pauses.
		old.State.CurrentLevelNumber++
		if old.State.CurrentLevelNumber >= len(old.Structure.Levels) {
			old.endOfTime()
			return
		}
		newLevel := old.CurrentLevel()
		if newLevel == nil {
			old.RestartLastLevel(clock)
			break
		}

		levelDuration := time.Duration(newLevel.DurationMinutes) * time.Minute
		newEndsAt := endsAt.Add(levelDuration).UnixMilli()
		asInt64 := int64(newEndsAt)
		old.State.CurrentLevelEndsAt = &asInt64
	}
}

func (old *Tournament) RestartLastLevel(clock Clock) {
	old.State.CurrentLevelNumber = len(old.Structure.Levels) - 1
	old.restartLevel(clock)
}

type Clock interface {
	Now() time.Time
}

// FillTransients fills out computed fields.  (These shouldn't be serialized to
// the database as they're redundant, but they are very convenient for access
// from templates and maybe JS.)
func (old *Tournament) FillTransients(clock Clock) {
	old.Transients = &Transients{}

	if old.State.TotalChipsOverride > 0 {
		old.Transients.TotalChips = old.State.TotalChipsOverride
	} else {
		old.Transients.TotalChips = old.State.BuyIns*old.Structure.ChipsPerBuyIn + old.State.AddOns*old.Structure.ChipsPerAddOn
	}

	if old.State.CurrentPlayers == 0 {
		old.Transients.AverageChips = 0
	} else {
		old.Transients.AverageChips = int(math.Round(float64(old.Transients.TotalChips) / float64(old.State.CurrentPlayers)))
	}

	old.adjustStateForElapsedTime(clock)

	old.fillNextBreak()
	old.fillNextLevel()
}

func (old *Tournament) fillNextBreak() {
	if !old.State.IsClockRunning {
		old.Transients.NextBreakAt = nil
		return
	}

	if old.State.CurrentLevelEndsAt == nil {
		log.Printf("can't fillNextBreak: CurrentLevelEndsAt is nil")
	}

	when := old.CurrentLevelEndsAtAsTime()

	for i := old.State.CurrentLevelNumber + 1; i < len(old.Structure.Levels); i++ {
		maybeBreakLevel := old.Structure.Levels[i]
		if maybeBreakLevel.IsBreak {
			millis := when.UnixMilli()
			old.Transients.NextBreakAt = &millis
			return
		}

		when = when.Add(time.Duration(maybeBreakLevel.DurationMinutes) * time.Minute)
	}

	// no break for you
	old.Transients.NextBreakAt = nil
}

// fillNextLevel sets Transients.NextLevel to the next non-break level.
func (old *Tournament) fillNextLevel() {
	for i := old.State.CurrentLevelNumber + 1; i < len(old.Structure.Levels); i++ {
		if old.Structure.Levels[i].IsBreak {
			continue
		}
		old.Transients.NextLevel = old.Structure.Levels[i]
		return
	}
	old.Transients.NextLevel = nil
}

func (old *Tournament) PreviousLevel(clock Clock) error {
	if old.State.CurrentLevelNumber <= 0 {
		return errors.New("already at min level")
	}
	old.State.CurrentLevelNumber--
	old.restartLevel(clock)
	return nil
}

func (old *Tournament) endOfTime() {
	log.Printf("tournament %d at end of time", old.EventID)
	one := int64(1)
	old.State.CurrentLevelNumber = len(old.Structure.Levels) - 1
	old.State.TimeRemainingMillis = &one
	old.State.CurrentLevelEndsAt = nil
	old.State.IsClockRunning = false
}

func (old *Tournament) AdvanceLevel(clock Clock) error {
	if old.State.CurrentLevelNumber >= len(old.Structure.Levels)-1 {
		old.endOfTime()
		return nil
	}

	old.State.CurrentLevelNumber++
	old.restartLevel(clock)
	return nil
}

func (old *Tournament) CurrentLevelDuration() *time.Duration {
	if old.CurrentLevel() == nil {
		return nil
	}
	d := time.Duration(old.CurrentLevel().DurationMinutes) * time.Minute
	return &d
}

// restartLevel resets the current level's clocks after a manual level change.
// (It doesn't make sense to call this externally.)
func (old *Tournament) restartLevel(clock Clock) {
	if old.CurrentLevel() == nil {
		log.Printf("debug: can't restart level: no current level")
	}
	minutes := old.CurrentLevel().DurationMinutes
	d := time.Duration(minutes) * time.Minute
	if old.State.IsClockRunning {
		later := clock.Now().Add(d).UnixMilli()
		old.State.CurrentLevelEndsAt = &later
		old.State.TimeRemainingMillis = nil
	} else {
		remainingMillis := int64(d.Milliseconds())
		old.State.TimeRemainingMillis = &remainingMillis
		old.State.CurrentLevelEndsAt = nil
	}
}

func (old *Tournament) StopClock(clock Clock) error {
	log.Printf("stop clock request for tournament %d", old.EventID)
	old.adjustStateForElapsedTime(clock)

	if !old.State.IsClockRunning {
		log.Printf("debug: can't stop a stopped clock")
		return nil
	}

	if old.CurrentLevel() == nil {
		return errors.New("can't stop clock: no current level")
	}

	endsAt := old.CurrentLevelEndsAtAsTime()
	remainingMillis := endsAt.Sub(clock.Now()).Milliseconds()

	old.State.IsClockRunning = false
	old.State.TimeRemainingMillis = &remainingMillis
	old.State.CurrentLevelEndsAt = nil
	return nil
}

func (old *Tournament) StartClock(clock Clock) error {
	old.adjustStateForElapsedTime(clock)

	if old.State.IsClockRunning {
		log.Printf("debug: can't start a started clock")
		return nil
	}

	if old.CurrentLevel() == nil {
		log.Printf("debug: can't start a clock with no current level")
		return errors.New("can't start a clock with no current level")
	}

	var remaining time.Duration
	if old.State.TimeRemainingMillis != nil {
		remaining = time.Duration(*old.State.TimeRemainingMillis) * time.Millisecond
	} else {
		log.Printf("debug: when starting clock, no TimeRemainingMillis, using full level duration")
		remaining = *old.CurrentLevelDuration()
	}

	endsAt := clock.Now().Add(remaining).UnixMilli()
	old.State.CurrentLevelEndsAt = &endsAt
	old.State.TimeRemainingMillis = nil
	old.State.IsClockRunning = true
	return nil
}

func (old *Tournament) RemovePlayer(clock Clock) error {
	if old.State.CurrentPlayers > 1 {
		old.State.CurrentPlayers--
		old.FillTransients(clock)
		return nil
	}
	return errors.New("can't remove the last player")
}

func (old *Tournament) AddPlayer(clock Clock) error {
	old.State.CurrentPlayers++
	old.FillTransients(clock)
	return nil
}

func (old *Tournament) AddBuyIn(clock Clock) error {
	old.State.BuyIns++
	old.FillTransients(clock)
	return nil
}

func (old *Tournament) RemoveBuyIn(clock Clock) error {
	if old.State.BuyIns > 0 {
		old.State.BuyIns--
		old.FillTransients(clock)
		return nil
	}
	return errors.New("can't buy in less than zero")
}

func (old *Tournament) PlusMinute(clock Clock) error {
	old.adjustStateForElapsedTime(clock)

	if old.CurrentLevel() == nil {
		return errors.New("can't add a minute: no current level")
	}

	if old.State.IsClockRunning {
		newEndsAt := old.CurrentLevelEndsAtAsTime().Add(time.Minute)
		asInt64 := newEndsAt.UnixMilli()
		old.State.CurrentLevelEndsAt = &asInt64
		old.State.TimeRemainingMillis = nil
	} else {
		var remaining int64
		if old.State.TimeRemainingMillis != nil {
			remaining = *old.State.TimeRemainingMillis
		} else {
			remaining = int64(*old.CurrentLevelDuration())
		}
		remaining += millisPerMinute
		old.State.TimeRemainingMillis = &remaining
		old.State.CurrentLevelEndsAt = nil
	}

	old.FillTransients(clock)

	return nil
}

func (old *Tournament) MinusMinute(clock Clock) error {
	old.adjustStateForElapsedTime(clock)

	if old.CurrentLevel() == nil {
		return errors.New("can't add a minute: no current level")
	}

	if old.State.IsClockRunning {
		newEndsAt := old.CurrentLevelEndsAtAsTime().Add(-time.Minute)
		asInt64 := newEndsAt.UnixMilli()
		old.State.CurrentLevelEndsAt = &asInt64
		old.State.TimeRemainingMillis = nil

		// special case: if there was less than a minute left and we just
		// bumped to the next level, we just start the next level as normal.
		if newEndsAt.Before(clock.Now()) {
			// Skip to next level, which should reset it (or end the tournamment).
			old.AdvanceLevel(clock)
			return nil
		}
	} else {
		var remaining int64
		if old.State.TimeRemainingMillis != nil {
			remaining = *old.State.TimeRemainingMillis
		} else {
			log.Printf("debug: when adding a minute, no TimeRemainingMillis, using full level duration")
			remaining = int64(*old.CurrentLevelDuration())
		}

		remaining -= millisPerMinute

		if int64(remaining) < 0 {
			// special case: if we just exhausted this level, go to the next level
			// and give it a full time allotment.
			old.AdvanceLevel(clock)
			return nil
		} else {
			old.State.TimeRemainingMillis = &remaining
			old.State.CurrentLevelEndsAt = nil
		}
	}

	old.FillTransients(clock)

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
