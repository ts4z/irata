package model

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ts4z/irata/ts"
)

const (
	millisPerMinute = 60 * 1000
)

var (
	dashDashRE = regexp.MustCompile(`\s*--\s*`)
)

type Level struct {
	// configuration
	Description     string
	DurationMinutes int // TODO: convert this to a string
	IsBreak         bool
}

func parseLevelBreak(s string) bool {
	if strings.EqualFold(s, "BREAK") {
		return true
	} else {
		return false
	}
}

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
			DurationMinutes: durationMins,
			IsBreak:         isBreak,
			Description:     description,
		})
	}
	return levels, nil
}

// FooterPlugs is possible values for decorating the footer.
//
// This is intended to support things other than text, but that's not
// implemented yet.
type FooterPlugs struct {
	FooterPlugsID  int64
	OptimisticLock int64
	TextPlugs      []string
}

// Tournaments are the things that we're running.
type Tournament struct {
	EventID        int64 // TODO: rename to TournamentID
	OptimisticLock int64

	EventName     string
	Description   string
	FooterPlugsID int64
	StructureID   int64
	Structure     *Structure
	State         *State
	Transients    *Transients
}

// Strucutre describes the structure of a tournament.
type Structure struct {
	StructureID    int64
	OptimisticLock int64

	Levels        []*Level
	ChipsPerBuyIn int
	ChipsPerAddOn int
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
	TotalChips         int
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

// Transients are computed from State and Structure, and are not serialized to the database.
type Transients struct {
	AverageChips int
	// NextBreakAt is the time the next break starts, in Unix millis, or nil if there are no more breaks.
	NextBreakAt *int64
	// NextLevel is the next non-break level, or nil if there are no more levels.
	NextLevel *Level
}

// Current level returns the current level, or if the tourn
func (m *Tournament) CurrentLevel() *Level {
	var lvl int = m.State.CurrentLevelNumber
	if lvl < 0 {
		lvl = 0
	} else if lvl >= len(m.Structure.Levels) {
		lvl = len(m.Structure.Levels) - 1
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
func (m *Tournament) adjustStateForElapsedTime() {
	if m.CurrentLevel() == nil {
		m.RestartLastLevel()
		return
	}

	if !m.State.IsClockRunning {
		if m.State.TimeRemainingMillis == nil {
			if m.CurrentLevel() != nil {
				log.Printf("BUG: clock is not running but TimeRemainingMillis is nil, resetting to full time")
				remaining := int64(time.Duration(m.CurrentLevel().DurationMinutes) * time.Minute)
				m.State.TimeRemainingMillis = &remaining
			} else {
				log.Printf("BUG: clock running, no time remaining, no level?")
				m.RestartLastLevel()
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
		later := ts.Now().Add(time.Duration(m.CurrentLevel().DurationMinutes) * time.Minute).UnixMilli()
		m.State.CurrentLevelEndsAt = &later
		m.State.TimeRemainingMillis = nil
		return
	}

	for m.CurrentLevel() != nil {
		endsAt := m.CurrentLevelEndsAtAsTime()
		if endsAt.After(ts.Now()) {
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
			m.RestartLastLevel()
			break
		}

		levelDuration := time.Duration(newLevel.DurationMinutes) * time.Minute
		newEndsAt := endsAt.Add(levelDuration).UnixMilli()
		asInt64 := int64(newEndsAt)
		m.State.CurrentLevelEndsAt = &asInt64
		log.Printf("stepping to level %d, ends at %v", m.State.CurrentLevelNumber, newEndsAt)
	}
}

func (m *Tournament) RestartLastLevel() {
	m.State.CurrentLevelNumber = len(m.Structure.Levels) - 1
	m.restartLevel()
}

// FillTransients fills out computed fields.  (These shouldn't be serialized to
// the database as they're redundant, but they are very convenient for access
// from templates and maybe JS.)
func (m *Tournament) FillTransients() {
	m.Transients = &Transients{}
	m.Transients.AverageChips = m.State.TotalChips / m.State.CurrentPlayers

	m.adjustStateForElapsedTime()

	m.fillNextBreak()
	m.fillNextLevel()
}

func (m *Tournament) fillNextBreak() {
	if !m.State.IsClockRunning {
		m.Transients.NextBreakAt = nil
		return
	}

	if m.State.CurrentLevelEndsAt == nil {
		log.Printf("can't fillNextBreak: CurrentLevelEndsAt is nil")
	}

	when := m.CurrentLevelEndsAtAsTime()

	for i := m.State.CurrentLevelNumber + 1; i < len(m.Structure.Levels); i++ {
		maybeBreakLevel := m.Structure.Levels[i]
		if maybeBreakLevel.IsBreak {
			millis := when.UnixMilli()
			m.Transients.NextBreakAt = &millis
			return
		}

		when = when.Add(time.Duration(maybeBreakLevel.DurationMinutes) * time.Minute)
	}

	// no break for you
	m.Transients.NextBreakAt = nil
}

// fillNextLevel sets Transients.NextLevel to the next non-break level.
func (m *Tournament) fillNextLevel() {
	for i := m.State.CurrentLevelNumber + 1; i < len(m.Structure.Levels); i++ {
		if m.Structure.Levels[i].IsBreak {
			continue
		}
		m.Transients.NextLevel = m.Structure.Levels[i]
		return
	}
	m.Transients.NextLevel = nil
}

func (t *Tournament) PreviousLevel() error {
	if t.State.CurrentLevelNumber <= 0 {
		return errors.New("already at min level")
	}
	t.State.CurrentLevelNumber--
	t.restartLevel()
	return nil
}

func (t *Tournament) endOfTime() {
	log.Printf("tournament %d at end of time", t.EventID)
	one := int64(1)
	t.State.CurrentLevelNumber = len(t.Structure.Levels) - 1
	t.State.TimeRemainingMillis = &one
	t.State.CurrentLevelEndsAt = nil
	t.State.IsClockRunning = false
}

func (t *Tournament) AdvanceLevel() error {
	if t.State.CurrentLevelNumber >= len(t.Structure.Levels)-1 {
		t.endOfTime()
		return nil
	}

	t.State.CurrentLevelNumber++
	t.restartLevel()
	return nil
}

func (t *Tournament) CurrentLevelDuration() *time.Duration {
	if t.CurrentLevel() == nil {
		return nil
	}
	d := time.Duration(t.CurrentLevel().DurationMinutes) * time.Minute
	return &d
}

// restartLevel resets the current level's clocks after a manual level change.
// (It doesn't make sense to call this externally.)
func (t *Tournament) restartLevel() {
	if t.CurrentLevel() == nil {
		log.Printf("debug: can't restart level: no current level")
	}
	minutes := t.CurrentLevel().DurationMinutes
	d := time.Duration(minutes) * time.Minute
	if t.State.IsClockRunning {
		later := ts.Now().Add(d).UnixMilli()
		t.State.CurrentLevelEndsAt = &later
		t.State.TimeRemainingMillis = nil
	} else {
		remainingMillis := int64(d.Milliseconds())
		t.State.TimeRemainingMillis = &remainingMillis
		t.State.CurrentLevelEndsAt = nil
	}
}

func (t *Tournament) StopClock() error {
	log.Printf("STOP CLOCK")
	t.adjustStateForElapsedTime()

	if !t.State.IsClockRunning {
		log.Printf("debug: can't stop a stopped clock")
		return nil
	}

	if t.CurrentLevel() == nil {
		return errors.New("can't stop clock: no current level")
	}

	endsAt := t.CurrentLevelEndsAtAsTime()
	remainingMillis := endsAt.Sub(ts.Now()).Milliseconds()

	t.State.IsClockRunning = false
	t.State.TimeRemainingMillis = &remainingMillis
	t.State.CurrentLevelEndsAt = nil
	return nil
}

func (t *Tournament) StartClock() error {
	t.adjustStateForElapsedTime()

	if t.State.IsClockRunning {
		log.Printf("debug: can't start a started clock")
		return nil
	}

	if t.CurrentLevel() == nil {
		log.Printf("debug: can't start a clock with no current level")
		return errors.New("can't start a clock with no current level")
	}

	var remaining time.Duration
	if t.State.TimeRemainingMillis != nil {
		remaining = time.Duration(*t.State.TimeRemainingMillis) * time.Millisecond
	} else {
		log.Printf("debug: when starting clock, no TimeRemainingMillis, using full level duration")
		remaining = *t.CurrentLevelDuration()
	}

	endsAt := ts.Now().Add(remaining).UnixMilli()
	t.State.CurrentLevelEndsAt = &endsAt
	t.State.TimeRemainingMillis = nil
	t.State.IsClockRunning = true
	return nil
}

func (t *Tournament) RemovePlayer() error {
	if t.State.CurrentPlayers > 1 {
		t.State.CurrentPlayers--
		t.FillTransients()
		return nil
	}
	return errors.New("can't remove the last player")
}

func (t *Tournament) AddPlayer() error {
	t.State.CurrentPlayers++
	t.FillTransients()
	return nil
}

func (t *Tournament) AddBuyIn() error {
	t.State.BuyIns++
	t.FillTransients()
	return nil
}

func (t *Tournament) RemoveBuyIn() error {
	if t.State.BuyIns > 0 {
		t.State.BuyIns--
		t.FillTransients()
		return nil
	}
	return errors.New("can't buy in less than zero")
}

func (t *Tournament) PlusMinute() error {
	t.adjustStateForElapsedTime()

	if t.CurrentLevel() == nil {
		return errors.New("can't add a minute: no current level")
	}

	if t.State.IsClockRunning {
		newEndsAt := t.CurrentLevelEndsAtAsTime().Add(time.Minute)
		asInt64 := newEndsAt.UnixMilli()
		t.State.CurrentLevelEndsAt = &asInt64
		t.State.TimeRemainingMillis = nil
	} else {
		var remaining int64
		if t.State.TimeRemainingMillis != nil {
			remaining = *t.State.TimeRemainingMillis
		} else {
			remaining = int64(*t.CurrentLevelDuration())
		}
		remaining += millisPerMinute
		t.State.TimeRemainingMillis = &remaining
		t.State.CurrentLevelEndsAt = nil
	}

	t.FillTransients()

	return nil
}

func (t *Tournament) MinusMinute() error {
	t.adjustStateForElapsedTime()

	if t.CurrentLevel() == nil {
		return errors.New("can't add a minute: no current level")
	}

	if t.State.IsClockRunning {
		newEndsAt := t.CurrentLevelEndsAtAsTime().Add(-time.Minute)
		asInt64 := newEndsAt.UnixMilli()
		t.State.CurrentLevelEndsAt = &asInt64
		t.State.TimeRemainingMillis = nil

		// special case: if there was less than a minute left and we just
		// bumped to the next level, we just start the next level as normal.
		if newEndsAt.Before(ts.Now()) {
			// Skip to next level, which should reset it (or end the tournamment).
			t.AdvanceLevel()
			return nil
		}
	} else {
		var remaining int64
		if t.State.TimeRemainingMillis != nil {
			remaining = *t.State.TimeRemainingMillis
		} else {
			log.Printf("debug: when adding a minute, no TimeRemainingMillis, using full level duration")
			remaining = int64(*t.CurrentLevelDuration())
		}

		remaining -= millisPerMinute

		if int64(remaining) < 0 {
			// special case: if we just exhausted this level, go to the next level
			// and give it a full time allotment.
			t.AdvanceLevel()
			return nil
		} else {
			t.State.TimeRemainingMillis = &remaining
			t.State.CurrentLevelEndsAt = nil
		}
	}

	t.FillTransients()

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
	IsAdmin bool
	Slugs   []TournamentSlug
}
