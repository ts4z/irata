package action

import (
	"context"
	"log"
	"net/url"
	"strconv"

	"github.com/ts4z/irata/he"
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

func (a *Actor) EditEvent(ctx context.Context, id int64, form url.Values) error {
	log.Printf("edit path: %v", id)

	t, err := a.storage.FetchTournament(ctx, id)
	if err != nil {
		return he.HTTPCodedErrorf(404, "can't get tournament from database")
	}

	if v, ok := form["Version"]; ok && len(v) > 0 {
		n, err := strconv.ParseInt(v[0], 10, 64)
		if err != nil {
			return err
		}
		t.Version = n
	}

	maybeCopyString(form, &t.EventName, "EventName")
	maybeCopyString(form, &t.State.PrizePool, "PrizePool")
	maybeCopyString(form, &t.Description, "Description")

	return a.storage.SaveTournament(ctx, t)
}
