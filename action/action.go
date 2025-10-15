package action

import (
	"context"
	"log"
	"net/url"
	"strconv"
	"strings"

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

func (a *Actor) EditEvent(ctx context.Context, id int64, form url.Values) error {
	log.Printf("edit path: %v", id)

	t, err := a.storage.FetchTournament(ctx, id)
	if err != nil {
		return he.HTTPCodedErrorf(404, "can't get tournament from database")
	}

	maybeCopyInt64(form, &t.Version, "Version")

	maybeCopyInt(form, &t.Structure.ChipsPerBuyIn, "ChipsPerBuyin")
	maybeCopyInt(form, &t.Structure.ChipsPerAddOn, "ChipsPerAddOn")
	maybeCopyInt(form, &t.State.TotalChipsOverride, "TotalChipsOverride")

	maybeCopyString(form, &t.EventName, "EventName")
	maybeCopyString(form, &t.State.PrizePool, "PrizePool")
	maybeCopyString(form, &t.Description, "Description")

	return a.storage.SaveTournament(ctx, t)
}
