package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"

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

func (s *DBStorage) FetchSiteConfig(ctx context.Context) (*model.SiteConfig, error) {
	rows, err := s.db.Query("SELECT key, value from site_info;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	readSiteConfig := false
	config := &model.SiteConfig{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		switch key {
		case "conf":
			if readSiteConfig {
				return nil, errors.New("duplicate site config")
			}
			if err := json.Unmarshal([]byte(value), &config); err != nil {
				return nil, err
			} else {
				readSiteConfig = true
			}
		}
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if !readSiteConfig {
		return nil, errors.New("no site config found")
	}

	return config, nil
}

func (s *DBStorage) FetchPlugs(ctx context.Context, id int64) (*model.FooterPlugs, error) {
	return nil, errors.ErrUnsupported
}

func (s *DBStorage) FetchStructure(ctx context.Context, id int64) (*model.Structure, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT version, model_data, name FROM structures where structure_id=$1", id)
	if err != nil {
		return nil, fmt.Errorf("querying structure: %w", err)
	}

	defer rows.Close()

	st := &model.Structure{}
	for rows.Next() {
		var name string
		var lock int64
		var bytes []byte

		if err := rows.Scan(&lock, &bytes, &name); err != nil {
			return nil, err
		}

		err := json.Unmarshal(bytes, st)
		if err != nil {
			return nil, err
		}

		st.Name = name
		st.ID = id
		st.Version = lock
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if st == nil {
		return nil, he.New(404, fmt.Errorf("no such structure id %d", id))
	}

	return st, nil
}

func (s *DBStorage) FetchStructureSlugs(ctx context.Context, offset, limit int) ([]*model.StructureSlug, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name FROM structures LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying structures: %w", err)
	}

	defer rows.Close()

	slugs := []*model.StructureSlug{}
	for rows.Next() {
		var name string
		var id int64

		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}

		slugs = append(slugs, &model.StructureSlug{
			ID:   id,
			Name: name,
		})
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return slugs, nil
}

func (s *DBStorage) FetchOverview(ctx context.Context, offset, limit int) (*model.Overview, error) {
	rows, err := s.db.Query("SELECT tournament_id, model_data from tournaments LIMIT $1 OFFSET $2;", limit, offset)
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
		// log.Printf("read: id=%d, bytes=%q", id, bytes)
		tournament := model.Tournament{}
		err := json.Unmarshal(bytes, &tournament)
		if err != nil {
			log.Printf("JSON unmarshal failed: %v", err)
			continue
		}
		slug := model.TournamentSlug{
			TournamentID:   id,
			TournamentName: tournament.EventName,
			Description:    tournament.Description,
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
	rows, err := s.db.QueryContext(ctx, `SELECT version, model_data FROM tournaments where tournament_id=$1`, id)

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
		tm.Version = lock
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
	return tm, nil
}

func (s *DBStorage) CreateTournament(
	ctx context.Context,
	tm *model.Tournament) (int64, error) {

	cpy := *tm
	cpy.Transients = nil
	cpy.State.BuyIns = 0
	cpy.State.AddOns = 0
	cpy.State.CurrentPlayers = 0

	bytes, err := json.Marshal(&cpy)
	if err != nil {
		return 0, err
	}

	result, err := s.db.ExecContext(ctx, `INSERT INTO tournaments (handle, model_data) VALUES ($1, $2) RETURNING tournament_id;`,
		tm.Handle, bytes)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *DBStorage) SaveTournament(
	ctx context.Context,
	tm *model.Tournament) error {
	cpy := *tm

	cpy.Transients = nil
	bytes, err := json.Marshal(&cpy)
	if err != nil {
		return err
	}
	if result, err := s.db.ExecContext(ctx,
		`UPDATE tournaments SET version=$1+1, model_data=$2 WHERE tournament_id=$3 AND version=$1;`,
		tm.Version,
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

func (s *DBStorage) DeleteTournament(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE from tournaments WHERE tournament_id=$1", id)

	if err != nil {
		return err
	} else {
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return fmt.Errorf("%d rows deleted", n)
		} else {
			return nil
		}
	}
}

func (s *DBStorage) SaveStructure(
	ctx context.Context,
	st *model.Structure) error {
	cpy := *st

	bytes, err := json.Marshal(&cpy)
	if err != nil {
		return err
	}
	if result, err := s.db.ExecContext(ctx,
		`UPDATE tournaments SET version=$1+1, name=$4, model_data=$2 WHERE structure_id=$3 AND version=$1;`,
		st.Version, bytes, st.ID, st.Name); err != nil {
		log.Printf("update failed: %v", err)
		return err
	} else {
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return fmt.Errorf("optimistic lock failure, %d rows affected", n)
		}
	}

	// log.Printf("wrote: id=%d, bytes=%q", st.ID, bytes)
	return nil
}

func (s *DBStorage) DeleteStructure(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE from structures WHERE id=$1", id)

	if err != nil {
		return err
	} else {
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return fmt.Errorf("%d rows deleted", n)
		} else {
			return nil
		}
	}
}

func (s *DBStorage) CreateStructure(
	ctx context.Context,
	st *model.Structure) (int64, error) {

	cpy := *st

	bytes, err := json.Marshal(&cpy)
	if err != nil {
		return 0, err
	}

	result, err := s.db.ExecContext(ctx,
		`INSERT INTO structures (name, model_data) 
		    VALUES ($1, $2) RETURNING structure_id;`,
		st.Name, bytes)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}
