package action

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
)

type Actor struct {
	storage state.AppStorage
}

func New(s state.AppStorage) *Actor {
	return &Actor{storage: s}
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

// ApplyFormToTournament takes form values and applies them to a tournament,
// returning the modified tournament and any error encountered.
// This function may need to fetch additional data (like structures) from storage.
func ApplyFormToTournament(ctx context.Context, form url.Values, t *model.Tournament, storage state.AppStorage) error {
	maybeCopyInt64(form, &t.Version, "Version")

	changeStructure := form.Get("ChangeStructure") == "on"
	if changeStructure {
		structureID, err := strconv.ParseInt(form.Get("StructureID"), 10, 64)
		if err != nil || structureID == 0 {
			return he.HTTPCodedErrorf(400, "invalid structure ID")
		}

		// Fetch the new structure
		structure, err := storage.FetchStructure(ctx, structureID)
		if err != nil {
			return he.HTTPCodedErrorf(404, "can't fetch structure")
		}

		// Replace the structure and reset tournament state
		t.Structure = structure.StructureData
		t.FromStructureID = structureID
		t.State.CurrentLevelNumber = 0
		t.State.IsClockRunning = false
		timeRemaining := (time.Duration(structure.Levels[0].DurationMinutes) * time.Millisecond).Milliseconds()
		t.State.TimeRemainingMillis = &timeRemaining
		t.State.CurrentLevelEndsAt = nil
		log.Printf("Structure changed to %d, reset to level 0 and paused", structureID)
	}

	if footerPlugsID := form.Get("FooterPlugsID"); footerPlugsID != "" {
		if id, err := strconv.ParseInt(footerPlugsID, 10, 64); err == nil && id > 0 {
			t.FooterPlugsID = id
		}
	}

	maybeCopyInt(form, &t.Structure.ChipsPerBuyIn, "ChipsPerBuyIn")
	maybeCopyInt(form, &t.Structure.ChipsPerAddOn, "ChipsPerAddOn")
	maybeCopyInt(form, &t.State.TotalChipsOverride, "TotalChipsOverride")

	maybeCopyString(form, &t.EventName, "EventName")
	maybeCopyString(form, &t.State.PrizePool, "PrizePool")
	maybeCopyString(form, &t.Description, "Description")

	maybeCopyInt(form, &t.State.CurrentPlayers, "CurrentPlayers")
	maybeCopyInt(form, &t.State.BuyIns, "BuyIns")
	maybeCopyInt(form, &t.State.AddOns, "AddOns")
	maybeCopyInt(form, &t.State.Saves, "NumberOfSaves")
	maybeCopyInt(form, &t.State.AmountPerSave, "AmountPerSave")
	maybeCopyInt(form, &t.State.TotalPrizePoolOverride, "TotalPrizePoolOverride")
	maybeCopyInt(form, &t.PrizePoolPerBuyIn, "PrizePoolPerBuyIn")
	maybeCopyInt(form, &t.PrizePoolPerAddOn, "PrizePoolPerAddOn")

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

func (a *Actor) EditTournament(ctx context.Context, id int64, form url.Values) error {
	log.Printf("edit path: %v", id)

	t, err := a.storage.FetchTournament(ctx, id)
	if err != nil {
		return he.HTTPCodedErrorf(404, "can't get tournament from database")
	}

	err = ApplyFormToTournament(ctx, form, t, a.storage)
	if err != nil {
		return err
	}

	return a.storage.SaveTournament(ctx, t)
}

func (a *Actor) CreateTournament(ctx context.Context, form url.Values) (int64, error) {
	t := &model.Tournament{
		State: &model.State{},
	}

	err := ApplyFormToTournament(ctx, form, t, a.storage)
	if err != nil {
		return 0, err
	}

	return a.storage.CreateTournament(ctx, t)
}
