// package tournament provides tournament mutation logic independent of the
// underlying trivial data model.
//
// Modifying a tournament in non-trivial ways requires some care.
// We have multiple variants on time, and multiple (too many) ways to
// represent that time.  This package provides a way of working with
// those values that storage can access (so things that come out of
// storage are fully filled out for non-stored tranient fields),
// but allowing dependencies on these objects.  Notably, we need a real
// (sub-second) clock, and we need access to paytables storage, without
// imposing any constraints on the storage or data model scheme.
//
// Many changes are interdependent.  Changing the number of add-ons or
// buy-ins can change the chip count or prize pool.
//
// The name "Mutator" isn't ideal, because some of these things are
// just accessors; but I don't have a better one yet.

package tournament

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/paytable"
	"github.com/ts4z/irata/protocol"
	"github.com/ts4z/irata/soundmodel"
	"github.com/ts4z/irata/textutil"
)

// Clock gets the current time.  clockwork.Clock implements this.
type Clock interface {
	Now() time.Time
}

// PaytableFetcher does what it says on the tin.  Storage implements
// this (as of this writing, it's hardcoded anyway).
type PaytableFetcher interface {
	FetchPaytableByID(ctx context.Context, id int64) (*paytable.Paytable, error)
}

type SoundEffectFetcher interface {
	FetchSoundEffectByID(ctx context.Context, id int64) (*soundmodel.SoundEffect, error)
}

type Mutator struct {
	clock Clock
	ptf   PaytableFetcher
	sef   SoundEffectFetcher
}

func NewMutator(clock Clock, paytableFetcher PaytableFetcher, soundEffectFetcher SoundEffectFetcher) *Mutator {
	return &Mutator{
		clock: clock,
		ptf:   paytableFetcher,
		sef:   soundEffectFetcher,
	}
}

// ComputePrizePoolText calculates the prize pool distribution and returns
// a formatted text block suitable for display in the PrizePool textarea.
// Returns an error if the paytable is nil or if the calculation fails.
func (tm *Mutator) ComputePrizePoolText(m *model.Tournament) (string, error) {
	pt, err := tm.ptf.FetchPaytableByID(context.Background(), m.PaytableID)
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
		placeStr := textutil.FormatPlace(place)
		lines = append(lines, fmt.Sprintf("%s: $%d", placeStr, prize))
	}

	// Add saves if any
	if m.State.Saves > 0 {
		firstSave := len(prizes) + 1
		lastSave := firstSave + m.State.Saves - 1
		if firstSave == lastSave {
			nth := textutil.FormatPlace(firstSave)
			lines = append(lines, fmt.Sprintf("%s: $%d*", nth, m.State.AmountPerSave))
		} else {
			lastth := textutil.FormatPlace(lastSave)
			lines = append(lines, fmt.Sprintf("%d-%s: $%d*", firstSave, lastth, m.State.AmountPerSave))
		}
		lines = append(lines, "* save")
	}

	return strings.Join(lines, "\n"), nil
}

func (tm *Mutator) CurrentLevel(m *model.Tournament) *model.Level {
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

// Deduct time from the running level.
func (tm *Mutator) MinusTime(ctx context.Context, m *model.Tournament, d time.Duration) error {
	if d < 0 {
		log.Fatalf("can't happen: MinusTime with negative duration %v", d)
	}

	if m.CurrentLevel() == nil {
		return errors.New("can't add a minute: no current level")
	}

	// Step clock forward to be consistent with wall-clock.
	tm.adjustStateForElapsedTime(m)

	// Stop clock if running.  Now we can easily manipulate time remaining.
	isRunning := m.State.IsClockRunning
	tm.StopClock(m)

	remainingInLevel := time.Duration(*m.State.TimeRemainingMillis) * time.Millisecond

	remainingInLevel -= d

	for remainingInLevel <= 0 {
		tm.AdvanceLevel(m)
		remainingInLevel += time.Duration(m.CurrentLevel().DurationMinutes) * time.Minute
	}

	*m.State.TimeRemainingMillis = remainingInLevel.Milliseconds()

	// If clock was running, start it again.
	if isRunning {
		tm.StartClock(m)
	}

	tm.FillTransients(ctx, m)

	return nil
}

// adjustStateForElapsedTime fixes the state to reflect the current time.
func (tm *Mutator) adjustStateForElapsedTime(m *model.Tournament) {
	if m.CurrentLevel() == nil {
		tm.RestartLastLevel(m)
		return
	}

	if !m.State.IsClockRunning {
		if m.State.TimeRemainingMillis == nil {
			if m.CurrentLevel() != nil {
				log.Printf("BUG: clock is not running but TimeRemainingMillis is nil, resetting to full time")
				tm.restartLevel(m)
			} else {
				log.Printf("BUG: clock running, no time remaining, no level?")
				tm.RestartLastLevel(m)
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
		later := tm.clock.Now().Add(time.Duration(m.CurrentLevel().DurationMinutes) * time.Minute).UnixMilli()
		m.State.CurrentLevelEndsAt = &later
		m.State.TimeRemainingMillis = nil
		return
	}

	for m.CurrentLevel() != nil {
		endsAt := m.CurrentLevelEndsAtAsTime()
		if endsAt.After(tm.clock.Now()) {
			// end of level still in the future!  we're good.
			break
		}

		// step the level forward, assuming no clock pauses.
		m.State.CurrentLevelNumber++
		if m.State.CurrentLevelNumber >= len(m.Structure.Levels) {
			endOfTime(m)
			return
		}
		newLevel := m.CurrentLevel()
		if newLevel == nil {
			tm.RestartLastLevel(m)
			break
		}

		levelDuration := time.Duration(newLevel.DurationMinutes) * time.Minute
		newEndsAt := endsAt.Add(levelDuration).UnixMilli()
		asInt64 := int64(newEndsAt)
		m.State.CurrentLevelEndsAt = &asInt64
	}
}

func (tm *Mutator) SetLevelRemaining(m *model.Tournament, d time.Duration) {
	if d < 0 {
		d = 0
	}
	if d > time.Duration(m.CurrentLevel().DurationMinutes)*time.Minute {
		d = time.Duration(m.CurrentLevel().DurationMinutes) * time.Minute
	}
	if m.State.IsClockRunning {
		millis := tm.clock.Now().Add(d).UnixMilli()
		m.State.CurrentLevelEndsAt = &millis
	} else {
		remainingMillis := d.Milliseconds()
		m.State.TimeRemainingMillis = &remainingMillis
	}
}

func (tm *Mutator) AdvanceLevel(m *model.Tournament) error {
	if m.State.CurrentLevelNumber >= len(m.Structure.Levels)-1 {
		endOfTime(m)
		return nil
	}

	m.State.CurrentLevelNumber++
	tm.restartLevel(m)
	return nil
}

func (tm *Mutator) RestartLastLevel(m *model.Tournament) {
	m.State.CurrentLevelNumber = len(m.Structure.Levels) - 1
	tm.restartLevel(m)
}

// FillTransients fills out computed fields.  (These shouldn't be serialized to
// the database as they're redundant, but they are very convenient for access
// from templates and maybe JS.)
func (tm *Mutator) FillTransients(ctx context.Context, m *model.Tournament) {
	m.Transients = &model.Transients{
		ProtocolVersion: protocol.Version,
	}

	if m.NextLevelSoundID >= 0 {
		soundEffect, err := tm.sef.FetchSoundEffectByID(ctx, m.NextLevelSoundID)
		if err != nil {
			log.Printf("warning: could not fetch sound effect ID %d: %v", m.NextLevelSoundID, err)
		} else {
			m.Transients.NextLevelSoundPath = soundEffect.Path
		}
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

	tm.adjustStateForElapsedTime(m)

	if tm.ptf != nil && m.State.AutoComputePrizePool {
		if ppt, err := tm.ComputePrizePoolText(m); err == nil {
			m.State.PrizePool = ppt
		}
	}
}

func (tm *Mutator) PreviousLevel(m *model.Tournament) error {
	if m.State.CurrentLevelNumber <= 0 {
		return errors.New("already at min level")
	}
	m.State.CurrentLevelNumber--
	tm.restartLevel(m)
	return nil
}

// restartLevel resets the current level's clocks after a manual level change.
// (It doesn't make sense to call this externally.)
func (tm *Mutator) restartLevel(m *model.Tournament) {
	if m.CurrentLevel() == nil {
		log.Printf("debug: can't restart level: no current level")
	}
	minutes := m.CurrentLevel().DurationMinutes
	d := time.Duration(minutes) * time.Minute
	if m.State.IsClockRunning {
		later := tm.clock.Now().Add(d).UnixMilli()
		m.State.CurrentLevelEndsAt = &later
		m.State.TimeRemainingMillis = nil
	} else {
		remainingMillis := int64(d.Milliseconds())
		m.State.TimeRemainingMillis = &remainingMillis
		m.State.CurrentLevelEndsAt = nil
	}
}

func (tm *Mutator) MuteSound(m *model.Tournament) error {
	m.State.SoundMuted = true
	return nil
}

func (tm *Mutator) UnmuteSound(m *model.Tournament) error {
	m.State.SoundMuted = false
	return nil
}

func (tm *Mutator) RestartLevel(m *model.Tournament) error {
	tm.StopClock(m)
	tm.restartLevel(m)
	return nil
}

func (tm *Mutator) RestartTournament(m *model.Tournament) error {
	m.State.CurrentLevelNumber = 0
	tm.StopClock(m)
	tm.restartLevel(m)
	return nil
}

func (tm *Mutator) StopClock(m *model.Tournament) error {
	log.Printf("stop clock request for tournament %d", m.EventID)
	tm.adjustStateForElapsedTime(m)

	if !m.State.IsClockRunning {
		log.Printf("debug: can't stop a stopped clock")
		return nil
	}

	if m.CurrentLevel() == nil {
		return errors.New("can't stop clock: no current level")
	}

	endsAt := m.CurrentLevelEndsAtAsTime()
	remainingMillis := endsAt.Sub(tm.clock.Now()).Milliseconds()

	m.State.IsClockRunning = false
	m.State.TimeRemainingMillis = &remainingMillis
	m.State.CurrentLevelEndsAt = nil
	return nil
}

func (tm *Mutator) StartClock(m *model.Tournament) error {
	tm.adjustStateForElapsedTime(m)

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

	endsAt := tm.clock.Now().Add(remaining).UnixMilli()
	m.State.CurrentLevelEndsAt = &endsAt
	m.State.TimeRemainingMillis = nil
	m.State.IsClockRunning = true
	return nil
}

func (tm *Mutator) ChangePlayers(ctx context.Context, m *model.Tournament, n int) error {
	m.State.CurrentPlayers += n
	if m.State.CurrentPlayers < 1 {
		m.State.CurrentPlayers = 1
	}
	tm.FillTransients(ctx, m)
	return nil
}

func (tm *Mutator) ChangeBuyIns(ctx context.Context, m *model.Tournament, n int) error {
	m.State.BuyIns += n
	if m.State.BuyIns < 1 {
		m.State.BuyIns = 1
	}
	tm.FillTransients(ctx, m)
	return nil
}

func (tm *Mutator) ChangeAddOns(ctx context.Context, m *model.Tournament, n int) error {
	m.State.AddOns += n
	if m.State.AddOns < 1 {
		m.State.AddOns = 0
	}
	tm.FillTransients(ctx, m)
	return nil
}

func (tm *Mutator) PlusTime(ctx context.Context, m *model.Tournament, d time.Duration) error {

	tm.adjustStateForElapsedTime(m)

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

	tm.FillTransients(ctx, m)

	return nil
}

// endOfTime is a convenience function for putting a tournament at the
// end of its levels.  To go any further will sprinlkle special cases all
// over the code, so instead, we set to the *last* level, *paused*, with
// no time remaining.  Un-pausing would immediately kick to the next level,
// which will encourage somebody to call this right back.
func endOfTime(m *model.Tournament) {
	log.Printf("tournament %d at end of time", m.EventID)
	zero := int64(0)
	m.State.CurrentLevelNumber = len(m.Structure.Levels) - 1
	m.State.TimeRemainingMillis = &zero
	m.State.CurrentLevelEndsAt = nil
	m.State.IsClockRunning = false
}
