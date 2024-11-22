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
	DurationMinutes int
	IsBreak         bool

	// State -- these are in the wrong place.  Refactoring this means fixing
	// movement.js as well.

	// EndsAt indicates when the level ends iff the clock is running.  This is in
	// Unix millis.  This value is not useful if the current level is paused, because
	// we don't know when the clock will be un-frozen.
	EndsAt *int64
	// TimeRemaining indicates time remaining iff the clock is not running (that
	// is, paused).  This is in Unix millis.  This can always be initialized
	// within a level.
	TimeRemainingMillis int64
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

type Tournament struct {
	// early configuration
	EventID     int64
	EventName   string
	FooterPlugs []string
	Structure   *Structure
	State       *State
	Transients  *Transients
}

type Structure struct {
	Levels        []*Level
	ChipsPerBuyIn int
	ChipsPerAddOn int
}

type State struct {
	IsClockRunning     bool
	CurrentLevelNumber int
	CurrentPlayers     int
	BuyIns             int
	TotalChips         int
	PrizePool          string
}

// This should be removed and computed client-side?
type Transients struct {
	AverageChips int
	NextBreakAt  *int64
	NextLevel    *Level
}

// FillTransients fills out computed fields.  (These shouldn't be serialized to
// the database as they're redundant, but they are very convenient for access
// from templates and maybe JS.)
func (m *Tournament) FillTransients() {
	if m.State.IsClockRunning {
		// Bump current level forward until we find what time we're supposed to be
		// in.
		for m.State.CurrentLevelNumber < len(m.Structure.Levels)-1 {
			cl := m.Structure.Levels[m.State.CurrentLevelNumber]
			if time.UnixMilli(*cl.EndsAt).After(ts.Now()) {
				// this level ends after now, this is the level that should be active
				break
			}

			// step level forward, looking for currently active level
			m.State.CurrentLevelNumber++
		}
	}

	m.fillNextBreak()
	m.fillNextLevel()
}

func (m *Tournament) FillInitialLevelRemaining() {
	for _, level := range m.Structure.Levels {
		level.TimeRemainingMillis = int64(level.DurationMinutes) * millisPerMinute
	}
}

func (m *Tournament) fillNextBreak() {
	m.Transients = &Transients{}
	for i := m.State.CurrentLevelNumber + 1; i < len(m.Structure.Levels); i++ {
		maybeBreakLevel := m.Structure.Levels[i]
		if maybeBreakLevel.IsBreak {
			m.Transients.NextBreakAt = m.Structure.Levels[i-1].EndsAt
			return
		}
	}

	m.Transients.NextBreakAt = nil
}

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

// Update level times will fill out the current level times up to, and
// including, the current level.  If IsClockStopped has changed from the last
// time it was called, UpdateLevelTimes will either move TimeRemainingMillis
// into an updated end-of-level time, or vice versa.
//
// Wait, we don't want to update time in previous levels.  If we step backwards
// (because someone accidentally jumped the clock forward) we probably want to
// keep the current timestamps.
//
// When we switch to a new level, though, we want to recompute EndsAt.
func (m *Tournament) UpdateLevelTimes() {
	defer m.fillNextBreak()

	currentLevel := m.Structure.Levels[m.State.CurrentLevelNumber]

	if !m.State.IsClockRunning {
		// Clock is not running.  Take remaining time and store into
		// TimeRemainingMillis.  (Save to database.)  This level does not get an end time
		log.Printf("clock is stopped")
		if currentLevel.EndsAt != nil {
			//log.Printf("current level ends at %v", currentLevel.EndsAt)
			endsAt := time.UnixMilli(*currentLevel.EndsAt)
			currentLevel.EndsAt = nil
			currentLevel.TimeRemainingMillis = int64(endsAt.Sub(ts.Now()) / time.Millisecond)
			// log.Printf("time remaining in this level is %v", currentLevel.TimeRemainingMillis)
		}

		for i := m.State.CurrentLevelNumber + 1; i < len(m.Structure.Levels); i++ {
			level := m.Structure.Levels[i]
			level.EndsAt = nil
			level.TimeRemainingMillis = int64(level.DurationMinutes) * millisPerMinute
		}
		return
	}

	// Clock is running.  If the clock was just un-paused, the current level
	// will have weird data in it we need to reset, and later levels will have
	// similar trash.
	later := ts.Now()

	resetEndsAt := func(level *Level) {
		left := time.Duration(level.TimeRemainingMillis) * time.Millisecond
		later = later.Add(left)
		laterMilli := later.UnixMilli()
		level.EndsAt = &laterMilli
	}

	if currentLevel.EndsAt == nil || *currentLevel.EndsAt <= 0 {
		resetEndsAt(currentLevel)
	}

	for i := m.State.CurrentLevelNumber + 1; i < len(m.Structure.Levels); i++ {
		resetEndsAt(m.Structure.Levels[i])
	}
}

func (t *Tournament) PreviousLevel() error {
	if t.State.CurrentLevelNumber <= 0 {
		return errors.New("already at min level")
	}
	t.State.CurrentLevelNumber--
	t.UpdateLevelTimes()
	return nil
}

func (t *Tournament) SkipLevel() error {
	if t.State.CurrentLevelNumber < len(t.Structure.Levels)-1 {
		t.State.CurrentLevelNumber++
		t.UpdateLevelTimes()
	}
	return nil
}

func (t *Tournament) StopClock() error {
	t.State.IsClockRunning = false
	t.UpdateLevelTimes()
	return nil
}

func (t *Tournament) StartClock() error {
	t.State.IsClockRunning = true
	t.UpdateLevelTimes()
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
	if t.ActiveLevel().EndsAt == nil {
		t.ActiveLevel().TimeRemainingMillis += millisPerMinute
	} else {
		endsAt := time.UnixMilli(*t.ActiveLevel().EndsAt)
		aMinuteLater := endsAt.Add(time.Minute).UnixMilli()
		t.ActiveLevel().EndsAt = &aMinuteLater
		t.UpdateLevelTimes()
	}
	return nil
}

func (t *Tournament) MinusMinute() error {
	if t.ActiveLevel().EndsAt == nil {
		if t.ActiveLevel().TimeRemainingMillis > millisPerMinute {
			t.ActiveLevel().TimeRemainingMillis -= millisPerMinute
		}
	} else {
		endsAt := time.UnixMilli(*t.ActiveLevel().EndsAt)
		if endsAt.Sub(ts.Now()) > time.Minute {
			aMinuteSooner := endsAt.Add(-time.Minute).UnixMilli()
			t.ActiveLevel().EndsAt = &aMinuteSooner
		} else {
			// Make the minimum time left in this level 1s from now.
			oneSecondFromNow := ts.Now().Add(time.Second).UnixMilli()
			t.ActiveLevel().EndsAt = &oneSecondFromNow
			t.UpdateLevelTimes()
		}
	}
	return nil
}

func (t *Tournament) ActiveLevel() *Level {
	return t.Structure.Levels[t.State.CurrentLevelNumber]
}

// EventOverview describes a single event for rendering the event list.
type EventOverview struct {
	EventID   int64
	EventName string
}

// Overview describes the available events for the event list.
type Overview struct {
	Events []EventOverview
}
