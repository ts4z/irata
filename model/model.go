package model

import (
	"errors"
	"log"
	"time"

	"github.com/ts4z/irata/ts"
)

const (
	millisPerMinute = 60 * 1000
)

type Level struct {
	// configuration
	Description     string
	DurationMinutes int
	IsBreak         bool

	// State

	// EndsAt indicates when the level ends iff the clock is running.  This is in
	// Unix millis.  This value is not useful if the current level is paused, because
	// we don't know when the clock will be un-frozen.
	EndsAt *int64
	// TimeRemaining indicates time remaining iff the clock is not running (that
	// is, paused).  This is in Unix millis.  This can always be initialized
	// within a level.
	TimeRemainingMillis int64
}

type Tournament struct {
	// early configuration
	EventName     string
	VenueName     string
	Levels        []*Level
	FooterPlugs   []string
	ChipsPerBuyIn int
	ChipsPerAddOn int

	// state
	IsClockRunning     bool
	CurrentLevelNumber int
	CurrentPlayers     int
	BuyIns             int
	TotalChips         int

	// transients
	AverageChips int
	NextBreakAt  *int64
}

func (m *Tournament) NLevels() int {
	return len(m.Levels)
}

// FillTransients fills out computed fields.  (These shouldn't be serialized to
// the database as they're redundant, but they are very convenient for access
// from templates and maybe JS.)
func (m *Tournament) FillTransients() {
	m.TotalChips = m.ChipsPerBuyIn*(m.Rebuys+m.BuyIns) + m.ChipsPerAddOn*m.AddOns
	m.AverageChips = m.TotalChips / m.CurrentPlayers

	if m.IsClockRunning {
		// Bump current level forward until we find what time we're supposed to be
		// in.
		for m.CurrentLevelNumber < m.NLevels()-1 {
			cl := m.Levels[m.CurrentLevelNumber]
			if time.UnixMilli(*cl.EndsAt).After(ts.Now()) {
				// this level ends after now, this is the level that should be active
				break
			}

			// step level forward, looking for currently active level
			m.CurrentLevelNumber++
		}
	}

	m.fillNextBreak()
}

func (m *Tournament) FillInitialLevelRemaining() {
	for _, level := range m.Levels {
		level.TimeRemainingMillis = int64(level.DurationMinutes) * millisPerMinute
	}
}

func (m *Tournament) fillNextBreak() {
	const millisPerMinute = 1000 * 60

	for i := m.CurrentLevelNumber + 1; i < len(m.Levels); i++ {
		maybeBreakLevel := m.Levels[i]
		if maybeBreakLevel.IsBreak {
			m.NextBreakAt = m.Levels[i-1].EndsAt
			return
		}
	}

	m.NextBreakAt = nil
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

	currentLevel := m.Levels[m.CurrentLevelNumber]

	if !m.IsClockRunning {
		// Clock is not running.  Take remaining time and store into
		// TimeRemainingMillis.  (Save to database.)  This level does not get an end time
		log.Printf("clock is now not running")
		if currentLevel.EndsAt != nil {
			//log.Printf("current level ends at %v", currentLevel.EndsAt)
			endsAt := time.UnixMilli(*currentLevel.EndsAt)
			currentLevel.EndsAt = nil
			currentLevel.TimeRemainingMillis = int64(endsAt.Sub(ts.Now()) / time.Millisecond)
			// log.Printf("time remaining in this level is %v", currentLevel.TimeRemainingMillis)
		}

		for i := m.CurrentLevelNumber + 1; i < len(m.Levels); i++ {
			level := m.Levels[i]
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

	for i := m.CurrentLevelNumber + 1; i < len(m.Levels); i++ {
		resetEndsAt(m.Levels[i])
	}
}

func (t *Tournament) PreviousLevel() error {
	if t.CurrentLevelNumber <= 0 {
		return errors.New("already at min level")
	}
	t.CurrentLevelNumber--
	t.UpdateLevelTimes()
	return nil
}

func (t *Tournament) SkipLevel() error {
	if t.CurrentLevelNumber < len(t.Levels)-1 {
		t.CurrentLevelNumber++
		t.UpdateLevelTimes()
	}
	return nil
}

func (t *Tournament) TogglePause() error {
	t.IsClockRunning = !t.IsClockRunning
	t.UpdateLevelTimes()
	return nil
}

func (t *Tournament) RemovePlayer() error {
	if t.CurrentPlayers > 1 {
		t.CurrentPlayers--
		t.FillTransients()
		return nil
	}
	return errors.New("can't remove the last player")
}

func (t *Tournament) AddPlayer() error {
	t.CurrentPlayers++
	t.FillTransients()
	return nil
}

func (t *Tournament) AddBuyIn() error {
	t.BuyIns++
	t.FillTransients()
	return nil
}

func (t *Tournament) RemoveBuyIn() error {
	if t.BuyIns > 0 {
		t.BuyIns--
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
			t.UpdateLevelTimes()
		}
	}
	return nil
}

func (t *Tournament) ActiveLevel() *Level {
	return t.Levels[t.CurrentLevelNumber]
}
