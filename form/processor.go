package form

import (
	"context"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
	"github.com/ts4z/irata/textutil"
	"github.com/ts4z/irata/tournament"
)

type FormProcessor struct {
	appStorage        state.AppStorage
	ts                state.TournamentStorage
	tournamentMutator *tournament.Manager
}

func NewProcessor(as state.AppStorage, ts state.TournamentStorage, tournamentMutator *tournament.Manager) *FormProcessor {
	return &FormProcessor{appStorage: as, ts: ts, tournamentMutator: tournamentMutator}
}

func maybeCopyString(form url.Values, dest *string, key string) {
	if v, ok := form[key]; ok && len(v) > 0 {
		*dest = v[0]
	}
}

func decomma(s string) string {
	return strings.ReplaceAll(s, ",", "")
}

func formNumberToInt64(s string) (int64, error) {
	s = decomma(s)
	return strconv.ParseInt(s, 10, 64)
}

func maybeCopyInt64(form url.Values, dest *int64, key string) {
	if v, ok := form[key]; ok && len(v) > 0 {
		val, err := formNumberToInt64(v[0])
		if err == nil {
			*dest = val
		}
	}
}

func maybeCopyInt(form url.Values, dest *int, key string) {
	if v, ok := form[key]; ok && len(v) > 0 {
		val, err := formNumberToInt64(v[0])
		if err == nil {
			*dest = int(val)
		}
	}
}

func parseOptionalInt(form url.Values, key string) (*int64, error) {
	s := form.Get(key)
	if s == "" {
		return nil, nil
	}
	s = decomma(s)
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func parseClockState(s string) (bool, error) {
	switch s {
	case "running":
		return true, nil
	case "paused":
		return false, nil
	default:
		return false, he.HTTPCodedErrorf(400, "invalid clock state")
	}
}

// ApplyFormToTournament takes form values and applies them to a tournament,
// returning the modified tournament and any error encountered.
// This function may need to fetch additional data (like structures) from storage.
func (a *FormProcessor) ApplyFormToTournament(ctx context.Context, form url.Values, t *model.Tournament) error {
	maybeCopyInt64(form, &t.Version, "Version")

	if lvlp, err := parseOptionalInt(form, "CurrentLevel"); err != nil {
		return err
	} else if lvlp == nil {
		// no form parameter, no change
	} else if lvl := *lvlp; lvl < 0 || lvl >= int64(len(t.Structure.Levels)) {
		return he.HTTPCodedErrorf(400, "level out of range")
	} else {
		t.State.CurrentLevelNumber = int(lvl)
	}

	if cs := form.Get("ClockState"); cs == "" {
		// ok
	} else if runClock, err := parseClockState(cs); err != nil {
		return err
	} else if runClock == t.State.IsClockRunning {
		// no change, hooray
	} else if runClock {
		a.tournamentMutator.StartClock(t)
	} else /* !runClock */ {
		a.tournamentMutator.StopClock(t)
	}

	timeRemainingAsHHMMSS := form.Get("TimeRemaining")
	if timeRemainingAsHHMMSS == "" {
		// great
	} else if duration, err := textutil.ParseDuration(timeRemainingAsHHMMSS); err != nil {
		return err
	} else {
		a.tournamentMutator.SetLevelRemaining(t, duration)
	}

	if form.Get("ChangeStructure") == "on" {
		structureID, err := strconv.ParseInt(form.Get("StructureID"), 10, 64)
		if err != nil || structureID == 0 {
			return he.HTTPCodedErrorf(400, "invalid structure ID")
		}

		// Fetch the new structure
		structure, err := a.appStorage.FetchStructure(ctx, structureID)
		if err != nil {
			return he.HTTPCodedErrorf(404, "can't fetch structure")
		}

		// Replace the structure and reset tournament state
		t.Structure = structure.StructureData
		t.FromStructureID = structureID
		t.State.CurrentLevelNumber = 0
		t.State.IsClockRunning = false
		timeRemaining := (time.Duration(structure.Levels[0].DurationMinutes) * time.Minute).Milliseconds()
		t.State.TimeRemainingMillis = &timeRemaining
		t.State.CurrentLevelEndsAt = nil
		log.Printf("Structure changed to %d, reset to level 0 and paused", structureID)
	}

	maybeCopyInt64(form, &t.FooterPlugsID, "FooterPlugsID")
	maybeCopyInt64(form, &t.NextLevelSoundID, "NextLevelSoundID")

	maybeCopyInt(form, &t.PrizePoolPerBuyIn, "PrizePoolPerBuyIn")
	maybeCopyInt(form, &t.PrizePoolPerAddOn, "PrizePoolPerAddOn")

	maybeCopyInt(form, &t.Structure.ChipsPerBuyIn, "ChipsPerBuyIn")
	maybeCopyInt(form, &t.Structure.ChipsPerAddOn, "ChipsPerAddOn")

	maybeCopyString(form, &t.EventName, "EventName")
	maybeCopyString(form, &t.State.PrizePool, "PrizePool")
	maybeCopyString(form, &t.Description, "Description")

	maybeCopyInt(form, &t.State.AddOns, "AddOns")
	maybeCopyInt(form, &t.State.AmountPerSave, "AmountPerSave")
	maybeCopyInt(form, &t.State.BuyIns, "BuyIns")
	maybeCopyInt(form, &t.State.CurrentPlayers, "CurrentPlayers")
	maybeCopyInt(form, &t.State.Saves, "NumberOfSaves")
	maybeCopyInt(form, &t.State.TotalChipsOverride, "TotalChipsOverride")
	maybeCopyInt(form, &t.State.TotalPrizePoolOverride, "TotalPrizePoolOverride")

	// Handle prize pool mode
	prizePoolMode := form.Get("PrizePoolMode")
	if prizePoolMode == "calculated" {
		t.State.AutoComputePrizePool = true
		maybeCopyInt64(form, &t.PaytableID, "PaytableID")
	} else {
		t.State.AutoComputePrizePool = false
	}

	return nil
}

func (a *FormProcessor) EditTournament(ctx context.Context, id int64, form url.Values) error {
	log.Printf("edit path: %v", id)

	t, err := a.ts.FetchTournament(ctx, id)
	if err != nil {
		return he.HTTPCodedErrorf(404, "can't get tournament from database")
	}

	// a.tournamentMutator.AdvanceLevel(t)

	err = a.ApplyFormToTournament(ctx, form, t)
	if err != nil {
		return err
	}

	return a.ts.SaveTournament(ctx, t)
}

func (a *FormProcessor) CreateTournament(ctx context.Context, form url.Values) (int64, error) {
	t := &model.Tournament{
		State: &model.State{},
	}

	err := a.ApplyFormToTournament(ctx, form, t)
	if err != nil {
		return 0, err
	}

	return a.ts.CreateTournament(ctx, t)
}
