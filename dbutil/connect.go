package dbutil

import (
	"database/sql"
	"errors"
	"log"
	"maps"
	"slices"

	"github.com/ts4z/irata/config"
)

func connectWithPgx() (*sql.DB, error) {
	url := config.DBURL()
	log.Printf("Connecting to database at %s", url)
	if url == "" {
		return nil, errors.New("database URL is empty")
	}
	return sql.Open("pgx", url)
}

// Connect establishes a database connection using the configured SQL connector.
//
// A previouis version of this code supported the Google Cloud connector.  That
// code neds some work (it shouldn't need a password, but it did) and has been
// removed for now.  However, it increased the binary size by about 10MB to pull in
// the various GCP libraries.  Since we're not using it, better not have it.
func Connect() (*sql.DB, error) {
	factories := map[string]func() (*sql.DB, error){
		"pgx": connectWithPgx,
	}
	factory, ok := factories[config.SQLConnector()]
	if !ok {
		log.Fatalf("unknown value for config.SQLConnector(): %q; known values are %v", config.SQLConnector(), slices.Collect(maps.Keys(factories)))
	}
	return factory()
}
