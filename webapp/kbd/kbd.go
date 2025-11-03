package kbd

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/permission"
	"github.com/ts4z/irata/state"
	"github.com/ts4z/irata/tournament"
)

type modifiers struct {
	Shift bool
}

func ifb[T any](cond bool, t T, f T) T {
	if cond {
		return t
	}
	return f
}

func if10(b bool) int {
	return ifb(b, 10, 1)
}

func if10min(b bool) time.Duration {
	return ifb(b, 10*time.Minute, 1*time.Minute)
}

type KeyboardShortcutDispatcher struct {
	keyToMutation     map[string]func(context.Context, *model.Tournament, *modifiers) error
	tournamentStorage state.TournamentStorage
	tm                *tournament.Manager
}

func NewKeyboardShortcutDispatcher(tm *tournament.Manager, ts state.TournamentStorage) *KeyboardShortcutDispatcher {
	k2m := map[string]func(ctx context.Context, t *model.Tournament, bb *modifiers) error{
		"PreviousLevel": func(ctx context.Context, t *model.Tournament, bb *modifiers) error { return tm.PreviousLevel(t) },
		"SkipLevel":     func(ctx context.Context, t *model.Tournament, bb *modifiers) error { return tm.AdvanceLevel(t) },
		"StopClock":     func(ctx context.Context, t *model.Tournament, bb *modifiers) error { return tm.StopClock(t) },
		"StartClock":    func(ctx context.Context, t *model.Tournament, bb *modifiers) error { return tm.StartClock(t) },
		"RemovePlayer": func(ctx context.Context, t *model.Tournament, bb *modifiers) error {
			return tm.ChangePlayers(ctx, t, -if10(bb.Shift))
		},
		"AddPlayer": func(ctx context.Context, t *model.Tournament, bb *modifiers) error {
			return tm.ChangePlayers(ctx, t, if10(bb.Shift))
		},
		"AddBuyIn": func(ctx context.Context, t *model.Tournament, bb *modifiers) error {
			return tm.ChangeBuyIns(ctx, t, if10(bb.Shift))
		},
		"AddAddOn": func(ctx context.Context, t *model.Tournament, bb *modifiers) error {
			return tm.ChangeAddOns(ctx, t, if10(bb.Shift))
		},
		"RemoveAddOn": func(ctx context.Context, t *model.Tournament, bb *modifiers) error {
			return tm.ChangeAddOns(ctx, t, -if10(bb.Shift))
		},
		"RemoveBuyIn": func(ctx context.Context, t *model.Tournament, bb *modifiers) error {
			return tm.ChangeBuyIns(ctx, t, -if10(bb.Shift))
		},
		"PlusMinute": func(ctx context.Context, t *model.Tournament, bb *modifiers) error {
			return tm.PlusTime(ctx, t, if10min(bb.Shift))
		},
		"MinusMinute": func(ctx context.Context, t *model.Tournament, bb *modifiers) error {
			return tm.MinusTime(ctx, t, if10min(bb.Shift))
		},
		"MuteSound":   func(ctx context.Context, t *model.Tournament, bb *modifiers) error { return tm.MuteSound(t) },
		"UnmuteSound": func(ctx context.Context, t *model.Tournament, bb *modifiers) error { return tm.UnmuteSound(t) },
		"Restart": func(ctx context.Context, t *model.Tournament, bb *modifiers) error {
			if bb.Shift {
				return tm.PauseAndRestartTournament(t)
			} else {
				return tm.PauseAndRestartLevel(t)
			}
		},
	}

	return &KeyboardShortcutDispatcher{
		keyToMutation:     k2m,
		tournamentStorage: ts,
		tm:                tm,
	}
}

func (app *KeyboardShortcutDispatcher) HandleKeypress(ctx context.Context, r *http.Request) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("can't read response body: %v", err)
	}

	type KeyboardModifyEvent struct {
		TournamentID int64
		Event        string
		Shift        bool
	}

	var event KeyboardModifyEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("can't unmarshal event %s: %v", string(body), err)
	}

	// Redundant check (storage checks too) to marginally improve logs + error.
	if permission.CheckWriteAccessToTournamentID(ctx, event.TournamentID) != nil {
		return he.HTTPCodedErrorf(http.StatusUnauthorized, "permission denied")
	}

	if h, ok := app.keyToMutation[event.Event]; !ok {
		return he.HTTPCodedErrorf(404, "unknown keyboard event")
	} else {
		t, err := app.tournamentStorage.FetchTournament(ctx, event.TournamentID)
		if err != nil {
			return he.HTTPCodedErrorf(404, "tournament not found: %w", err)
		}

		if err := h(ctx, t, &modifiers{Shift: event.Shift}); err != nil {
			return he.HTTPCodedErrorf(500, "while applying keyboard event: %w", err)
		}

		if err := app.tournamentStorage.SaveTournament(ctx, t); err != nil {
			return he.HTTPCodedErrorf(500, "save tournament after keypress: %w", err)
		}
	}
	return nil
}
