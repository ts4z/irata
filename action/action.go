package action

import (
	"log"
	"net/url"

	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/storage"
)

func EditEvent(id int64, form url.Values) error {
	log.Printf("edit path: %v", id)

	t, err := storage.FetchTournament(id)
	if err != nil {
		return he.HTTPCodedErrorf(404, "can't get tournament from database")
	}

	if v, ok := form["EventName"]; ok && len(v) > 0 {
		t.EventName = v[0]
	}
	if v, ok := form["PrizePool"]; ok && len(v) > 0 {
		t.State.PrizePool = v[0]
	}
	if v, ok := form["Levels"]; ok && len(v) > 0 {
		pl, err := model.ParseLevels(v[0])
		if err != nil {
			return err
		}
		t.Structure.Levels = pl
	}

	return storage.SaveTournament(id, t)
}
