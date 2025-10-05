package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/ts4z/irata/defaults"
	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/model"
)

type DBStorage struct {
	lock sync.Mutex
	db   *sql.DB
}

var _ Storage = &DBStorage{}

func NewDBStorage(ctx context.Context, url string) (*DBStorage, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	return &DBStorage{
		lock: sync.Mutex{},
		db:   db,
	}, nil
}

func (s *DBStorage) Close() {
	s.db.Close()
}

func (s *DBStorage) FetchOverview(ctx context.Context) (*model.Overview, error) {
	rows, err := s.db.Query("SELECT tournament_id, model_data from tournaments;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	overview := &model.Overview{}
	for rows.Next() {
		var id int64
		var bytes []byte

		if err := rows.Scan(&id, &bytes); err != nil {
			log.Printf("Row scan failed: %v", err)
			continue
		}
		log.Printf("read: id=%d, bytes=%q", id, bytes)
		tournament := model.Tournament{}
		err := json.Unmarshal(bytes, &tournament)
		if err != nil {
			log.Printf("JSON unmarshal failed: %v", err)
			continue
		}
		slug := model.TournamentSlug{
			TournamentID:   id,
			TournamentName: tournament.EventName,
		}

		overview.Slugs = append(overview.Slugs, slug)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return overview, nil
}

// fetchTournamentPartial fetches without structure.
func (s *DBStorage) fetchTournamentPartial(ctx context.Context, id int64) (*model.Tournament, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT optimistic_lock, model_data FROM tournaments where tournament_id=$1`, id)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tm *model.Tournament

	for rows.Next() {
		if tm != nil {
			return nil, fmt.Errorf("can't happen: duplicate tournament id %d", id)
		}

		var lock int64
		var bytes []byte

		if err := rows.Scan(&lock, &bytes); err != nil {
			return nil, err
		}

		tm = &model.Tournament{}
		err := json.Unmarshal(bytes, tm)
		if err != nil {
			return nil, err
		}

		// These come from the databae row, not the JSON.
		tm.EventID = id
		tm.OptimisticLock = lock
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if tm == nil {
		return nil, he.New(404, fmt.Errorf("no such tournament id %d", id))
	}

	return tm, nil
}

func (s *DBStorage) FetchTournament(ctx context.Context, id int64) (*model.Tournament, error) {
	tm, err := s.fetchTournamentPartial(ctx, id)
	if err != nil {
		return nil, err
	}
	sm, err := s.FetchStructure(ctx, tm.StructureID)
	if err != nil {
		return nil, err
	}
	tm.Structure = sm
	return tm, nil
}

func (s *DBStorage) FetchStructure(ctx context.Context, int int64) (*model.Structure, error) {
	// TODO: Implement storage, including optimistic locking.
	return defaults.Structure(), nil
}

func (s *DBStorage) FetchPlugs(ctx context.Context, id int64) (*model.FooterPlugs, error) {
	// TODO: Implement storage, including optimistic locking.
	return defaults.FooterPlugs(), nil
}

func (s *DBStorage) SaveTournament(
	ctx context.Context,
	tm *model.Tournament) error {
	cpy := *tm
	cpy.Structure = nil
	cpy.Transients = nil
	bytes, err := json.Marshal(&cpy)
	if err != nil {
		return err
	}
	if result, err := s.db.ExecContext(ctx,
		`UPDATE tournaments SET optimistic_lock=$1+1, model_data=$2 WHERE tournament_id=$3 AND optimistic_lock=$1;`,
		tm.OptimisticLock,
		bytes,
		tm.EventID); err != nil {
		log.Printf("update failed: %v", err)
		return err
	} else {
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return fmt.Errorf("optimistic lock failure, %d rows affected", n)
		}
	}

	log.Printf("wrote: id=%d, bytes=%q", tm.EventID, bytes)
	return nil
}
