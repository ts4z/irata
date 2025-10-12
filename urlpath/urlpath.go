package urlpath

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ts4z/irata/he"
)

// idPathValue extracts the "id" path variable from the request and parses it.
//
// On error, an error is reported to the client, and a delay is imposed in case
// the client is sending crap in a tight loop.
func IDPathValue(w http.ResponseWriter, r *http.Request) (int64, error) {
	id, err := idPathValueFromRequest(r)
	if err != nil {
		time.Sleep(10 * time.Second)
		he.SendErrorToHTTPClient(w, "parsing URL", err)
	}
	return id, nil
}

func idPathValueFromRequest(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return -1, he.HTTPCodedErrorf(400, "can't parse id from url path: %v", err)
	}
	return id, nil
}
